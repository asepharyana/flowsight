package agents

import (
	"context"
	"fmt"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// fmtIDR renders rupiah compactly (Rp1.2T / Rp340B / Rp12M).
func fmtIDR(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	var s string
	switch {
	case v >= 1e12:
		s = fmt.Sprintf("Rp%.2fT", v/1e12)
	case v >= 1e9:
		s = fmt.Sprintf("Rp%.0fB", v/1e9)
	case v >= 1e6:
		s = fmt.Sprintf("Rp%.0fM", v/1e6)
	default:
		s = fmt.Sprintf("Rp%.0f", v)
	}
	if neg {
		return "-" + s
	}
	return s
}

// AnalyzeSmartMoney (A1) fuses broker top lists with foreign flow.
// Rule: >=3 brokers net-buy 5d + volume > 1.5x 20d avg => accumulation.
func AnalyzeSmartMoney(ctx context.Context, d Deps, ticker string) model.AgentResult {
	_ = ctx
	ticker = strings.ToUpper(ticker)
	res := model.AgentResult{Summary: "no broker snapshots available"}

	var top sectors.BrokerSummaryTop
	topDate, ok := payload(d.DB, ticker, "broker-summary-top", &top)
	var foreign sectors.ForeignFlow
	foreignDate, fok := payload(d.DB, ticker, "foreign-flow", &foreign)
	if !ok && !fok {
		return res
	}

	buySum, sellSum := 0.0, 0.0
	buyers, sellers := 0, 0
	var players []string
	if ok {
		for _, b := range top.TopBuyers {
			if b.NetIDR > 0 {
				buyers++
				buySum += float64(b.NetIDR)
				if len(players) < 3 {
					players = append(players, fmt.Sprintf("%s %s", b.BrokerCode, fmtIDR(float64(b.NetIDR))))
				}
			}
		}
		for _, s := range top.TopSellers {
			if s.NetIDR < 0 {
				sellers++
				sellSum += float64(-s.NetIDR)
			}
		}
		res.Citations = append(res.Citations, model.Cite("v2/broker-summary/"+ticker+"/top/", ticker, topDate))
	}

	fSum := 0.0
	fN := 0
	if fok {
		data := foreign.Data
		if len(data) > 5 {
			data = data[len(data)-5:]
		}
		for _, p := range data {
			fSum += float64(p.NetForeignInflow)
			fN++
		}
		res.Citations = append(res.Citations, model.Cite("v2/foreign-flow/"+ticker+"/", ticker, foreignDate))
	}

	total := buySum + sellSum
	imbalance := 0.0
	if total > 0 {
		imbalance = (buySum - sellSum) / total
	}
	score := imbalance * 70
	score += float64(minInt(buyers, 5)-minInt(sellers, 5)) * 4
	if fok && fN > 0 {
		if fSum > 0 {
			score += 10
		} else if fSum < 0 {
			score -= 10
		}
	}
	score = clampScore(score, -100, 100)
	res.Score = score

	// Volume multiple from stored daily bars.
	volMult := 0.0
	if vols, _, err := d.DB.DailyVolumes(ticker, 21); err == nil && len(vols) >= 2 {
		n := len(vols)
		if a := avg(vols[:n-1]); a > 0 {
			volMult = vols[n-1] / a
			res.Citations = append(res.Citations, model.Cite("v2/daily/"+ticker+"/", ticker, "stored"))
			res.Values = append(res.Values, model.Value{
				Label:     "volume vs 20d avg",
				Display:   fmt.Sprintf("%.1fx", volMult),
				Citations: res.Citations,
			})
		}
	}

	phase := "neutral"
	switch {
	case score >= 30:
		phase = "accumulation"
	case score <= -30:
		phase = "distribution"
	case ok && fok && imbalance*fSum < 0:
		phase = "conflict"
		res.Flags = append(res.Flags, "direction-conflict")
	}
	if ok && fok && imbalance*fSum > 0 {
		res.Flags = append(res.Flags, "direction-agreement")
	}
	if buyers >= 3 && volMult > 1.5 {
		res.Flags = append(res.Flags, "accumulation-rule")
	}
	if volMult > 2 {
		res.Flags = append(res.Flags, "volume-anomaly")
	}

	res.Values = append([]model.Value{{
		Label:     "broker net imbalance",
		Display:   fmt.Sprintf("%s net (%d buyers vs %d sellers)", fmtIDR(buySum-sellSum), buyers, sellers),
		Citations: res.Citations,
	}, {
		Label:     "key players",
		Display:   strings.Join(players, ", "),
		Citations: res.Citations,
	}, {
		Label:     "foreign 5d net",
		Display:   fmtIDR(fSum),
		Citations: res.Citations,
	}}, res.Values...)
	res.Summary = fmt.Sprintf("%s: score %+.0f, %d net-buy brokers, foreign %s",
		phase, score, buyers, fmtIDR(fSum))
	res.Extra = map[string]any{
		"phase": phase, "buyers": buyers, "sellers": sellers,
		"net_sum": buySum - sellSum, "foreign_sum": fSum,
		"volume_mult": volMult, "players": players,
	}
	return res
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
