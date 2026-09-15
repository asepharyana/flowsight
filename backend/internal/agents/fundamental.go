package agents

import (
	"context"
	"fmt"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// AnalyzeFundamental (A4) scores valuation vs subsector median + quality.
// Rubric: profitability 35, growth 25, leverage 20, payout 20. Grade A-F.
func AnalyzeFundamental(ctx context.Context, d Deps, ticker string) model.AgentResult {
	_ = ctx
	ticker = strings.ToUpper(ticker)
	res := model.AgentResult{Summary: "no fundamental snapshots available"}

	var report map[string]any
	repDate, repOK := payload(d.DB, ticker, "company-report", &report)
	var quarters []sectors.QuarterRow
	qDate, qOK := payload(d.DB, ticker, "financials-quarterly", &quarters)
	if !repOK && !qOK {
		return res
	}
	if repOK {
		res.Citations = append(res.Citations, model.Cite("v2/company/report/"+ticker+"/", ticker, repDate))
	}
	if qOK {
		res.Citations = append(res.Citations, model.Cite("v2/financials/quarterly/"+ticker+"/", ticker, qDate))
	}

	pe, pb, roe, de, payout := numAt(report, "pe_ratio", "pe", "p_e"),
		numAt(report, "pb_ratio", "pb", "p_b"),
		numAt(report, "roe", "return_on_equity"),
		numAt(report, "debt_to_equity", "de_ratio", "der"),
		numAt(report, "payout_ratio", "dividend_payout")
	// Normalize fraction-vs-percent inputs: ROE 0.21 == 21%, payout 62 == 62%.
	if roe > 0 && roe < 1 {
		roe *= 100
	}
	if payout > 1 {
		payout /= 100
	}

	// Revenue segments (Sankey-ready) feed the quality read on concentration.
	var segs sectors.Segments
	segNames := ""
	if segDate, segOK := payload(d.DB, ticker, "segments", &segs); segOK && len(segs.RevenueBreakdown) > 0 {
		top := segs.RevenueBreakdown
		for i := range top {
			if i >= 3 {
				break
			}
			if i > 0 {
				segNames += ", "
			}
			segNames += top[i].Source + "→" + top[i].Target
		}
		res.Citations = append(res.Citations, model.Cite("v2/company/get-segments/"+ticker+"/", ticker, segDate))
	}

	var peerPE, peerPB float64
	var subsector string
	var sub map[string]any
	subDate, subOK := payload(d.DB, "IDX", "subsector-valuation", &sub)
	if subOK {
		subsector, _ = sub["sub_sector"].(string)
		peerPE = numAt(sub, "median_pe", "pe_median")
		peerPB = numAt(sub, "median_pb", "pb_median")
		res.Citations = append(res.Citations, model.Cite("v2/subsector/report/", subsector, subDate))
	}

	// 8-quarter revenue/earnings trend.
	revTrend, earnTrend := 0.0, 0.0
	if len(quarters) >= 2 {
		n := len(quarters)
		if n > 8 {
			quarters = quarters[n-8:]
			n = 8
		}
		if first, last := fval(quarters[0].Revenue), fval(quarters[n-1].Revenue); first > 0 {
			revTrend = (last - first) / first
		}
		if first, last := fval(quarters[0].Earnings), fval(quarters[n-1].Earnings); first != 0 {
			earnTrend = (last - first) / abs(first)
		}
	}

	// ROE trajectory: falling ROE across quarters flags even when earnings rise.
	roeSlope := 0.0
	if len(quarters) >= 2 {
		first, last := roeOf(quarters[0]), roeOf(quarters[len(quarters)-1])
		if first > 0 {
			roeSlope = (last - first) / first
		}
	}

	// Weighted rubric 0-100.
	profit := 50.0
	if roe > 0 {
		profit = clampScore(roe*3, 0, 100) // ROE 20%+ => ~60+
	}
	growth := clampScore(50+revTrend*200+earnTrend*100, 0, 100)
	leverage := 60.0
	if de > 0 {
		leverage = clampScore(90-de*30, 0, 100) // DER 1x => ~60
	}
	pay := 50.0
	if payout > 0 && payout <= 0.8 {
		pay = 70
	} else if payout > 0.8 {
		pay = 30 // aggressive payout flagged
	}
	score := profit*0.35 + growth*0.25 + leverage*0.20 + pay*0.20
	res.Score = clampScore(score, 0, 100)

	grade := "F"
	for _, g := range []struct {
		min float64
		ch  string
	}{{85, "A"}, {70, "B"}, {55, "C"}, {40, "D"}} {
		if score >= g.min {
			grade = g.ch
			break
		}
	}

	var flags []string
	var vsPeers string
	if peerPE > 0 && pe > 0 {
		switch {
		case pe > peerPE*1.2:
			vsPeers = fmt.Sprintf("premium P/E %.1f vs %s median %.1f", pe, subsector, peerPE)
			flags = append(flags, "premium-valuation")
		case pe < peerPE*0.8:
			vsPeers = fmt.Sprintf("discount P/E %.1f vs %s median %.1f", pe, subsector, peerPE)
			flags = append(flags, "discount-valuation")
		default:
			vsPeers = fmt.Sprintf("P/E %.1f in line with %s median %.1f", pe, subsector, peerPE)
		}
	} else if pe > 0 {
		vsPeers = fmt.Sprintf("P/E %.1f (no peer median cached)", pe)
	}
	if payout > 0.8 {
		flags = append(flags, "aggressive-payout")
	}
	if len(quarters) >= 2 && earnTrend < -0.15 {
		flags = append(flags, "declining-earnings")
	}
	if len(quarters) >= 2 && roeSlope < -0.10 {
		flags = append(flags, "declining-roe")
	}
	res.Flags = flags

	res.Values = []model.Value{{
		Label:     "fundamental score",
		Display:   fmt.Sprintf("%.0f/100 grade %s", score, grade),
		Citations: res.Citations,
	}}
	if vsPeers != "" {
		res.Values = append(res.Values, model.Value{Label: "valuation vs peers", Display: vsPeers, Citations: res.Citations})
	}
	if len(quarters) >= 2 {
		res.Values = append(res.Values, model.Value{
			Label:     "8Q trend",
			Display:   fmt.Sprintf("revenue %+.0f%%, earnings %+.0f%% over %d quarters", revTrend*100, earnTrend*100, len(quarters)),
			Citations: res.Citations,
		})
	}
	if segNames != "" {
		res.Values = append(res.Values, model.Value{
			Label:     "revenue segments",
			Display:   segNames,
			Citations: res.Citations,
		})
	}
	res.Summary = fmt.Sprintf("grade %s score %.0f; %s", grade, score, vsPeers)
	res.Extra = map[string]any{
		"grade": grade, "pe": pe, "pb": pb, "roe": roe, "de": de,
		"payout": payout, "peer_pe": peerPE, "peer_pb": peerPB,
		"rev_trend": revTrend, "earn_trend": earnTrend,
	}
	return res
}

// numAt digs the first present numeric key out of nested maps.
func numAt(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := lookupNum(m, k); ok {
			return v
		}
	}
	return 0
}

func lookupNum(m map[string]any, key string) (float64, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			if f, ok := toFloat(v); ok {
				return f, true
			}
		}
		if sub, ok := v.(map[string]any); ok {
			if f, ok := lookupNum(sub, key); ok {
				return f, true
			}
		}
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// roeOf approximates quarter ROE from earnings/equity when both present.
func roeOf(q sectors.QuarterRow) float64 {
	e, eq := fval(q.Earnings), fval(q.Equity)
	if eq <= 0 {
		return 0
	}
	return e / eq * 100
}

func fval(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
