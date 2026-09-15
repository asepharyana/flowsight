package alerts

import (
	"context"
	"encoding/json"
	"time"

	"flowsight/internal/model"
	"flowsight/internal/store"
)

// Evaluate runs all 6 rules over stored snapshots for the watchlist and
// records + delivers hits. First-fire only per (rule, ticker, date): rows
// already present in alert_events for today are skipped. Delivery fans out to
// the alert owner's enabled destinations (server env is fallback). Prefer
// EvaluateFor; this wrapper keeps owner empty (server fallback only).
func Evaluate(ctx context.Context, db *store.DB, n *Notifier, alertID int64, ruleJSON string, tickers []string) []Finding {
	owner := ""
	_ = db.QueryRow(`SELECT user_key FROM alerts WHERE id=?`, alertID).Scan(&owner)
	return EvaluateFor(ctx, db, n, alertID, owner, ruleJSON, tickers)
}

// EvaluateFor is Evaluate scoped to an explicit owner: history rows and
// delivery both use owner (no cross-user leakage via alert_id reuse).
func EvaluateFor(ctx context.Context, db *store.DB, n *Notifier, alertID int64, owner, ruleJSON string, tickers []string) []Finding {
	today := time.Now().Format("2006-01-02")
	var out []Finding
	for _, t := range tickers {
		cands := candidates(db, t, today)
		for _, f := range cands {
			if !MatchRule(ruleJSON, f) {
				continue
			}
			if firedToday(db, f.Rule, t, today) {
				continue
			}
			cites, _ := json.Marshal(f.Citations)
			ctxJSON, _ := json.Marshal(f.Context)
			if _, err := db.InsertAlertEvent(alertID, t, today, f.Message, string(ctxJSON), string(cites), owner); err != nil {
				continue
			}
			_ = n.SendTo(ctx, owner, f, string(cites))
			_ = db.MarkAlertFired(alertID)
			out = append(out, f)
		}
	}
	return out
}

// candidates runs every rule against one ticker's stored state. Every
// finding carries real citations (no "derived" placeholder).
func candidates(db *store.DB, ticker, today string) []Finding {
	var out []Finding
	brokerDate := today
	if nets, err := db.NetBuySum5d(ticker); err == nil && len(nets) > 0 {
		volMult := 0.0
		volDate := today
		if vols, dates, err := db.DailyVolumes(ticker, 21); err == nil && len(vols) >= 2 {
			n := len(vols)
			if a := avg(vols[:n-1]); a > 0 {
				volMult = vols[n-1] / a
			}
			volDate = lastOf(dates)
		}
		if _, d, err := db.LatestSnapshot(ticker, "broker-summary-top"); err == nil {
			brokerDate = d
		}
		if f, ok := Accumulation(ticker, nets, volMult); ok {
			f.Citations = []model.Citation{
				model.Cite("v2/broker-summary/"+ticker+"/top/", ticker, brokerDate),
				model.Cite("v2/daily/"+ticker+"/", ticker, volDate),
			}
			out = append(out, f)
		}
	}
	if dates, nets, err := db.ForeignLast6(ticker); err == nil && len(nets) == 6 {
		if f, ok := ForeignReversal(ticker, nets); ok {
			f.Citations = []model.Citation{
				model.Cite("v2/foreign-flow/"+ticker+"/", ticker, dates[5]),
			}
			out = append(out, f)
		}
	}
	// Insider spike: real 2x-volume path (latest buy vs 30d avg) plus the
	// distinct-insider cluster path, both over stored filings.
	if avg30 := db.FilingAvg30(ticker); avg30 > 0 {
		if f, ok := InsiderSpike(ticker, db.LatestBuyVolume(ticker), avg30, db.DistinctInsiders7d(ticker)); ok {
			f.Citations = []model.Citation{model.Cite("v2/filings/", ticker, today)}
			out = append(out, f)
		}
	}
	if vols, dates, err := db.DailyVolumes(ticker, 21); err == nil && len(vols) >= 2 {
		n := len(vols)
		if a := avg(vols[:n-1]); a > 0 {
			if f, ok := UnusualVolume(ticker, lastOf(dates), vols[n-1]/a, isEarningsDate(db, ticker, lastOf(dates))); ok {
				f.Citations = []model.Citation{
					model.Cite("v2/daily/"+ticker+"/", ticker, lastOf(dates)),
				}
				out = append(out, f)
			}
		}
	}
	// Sector rotation from the stored sector-flow snapshot.
	var flow struct {
		Current  map[string]float64 `json:"current"`
		Previous map[string]float64 `json:"previous"`
	}
	if raw, d, err := db.LatestSnapshot("IDX", "sector-flow"); err == nil {
		_ = json.Unmarshal([]byte(raw), &flow)
		for sector, cur := range flow.Current {
			if f, ok := SectorRotation(sector, flow.Previous[sector], cur); ok {
				f.Citations = []model.Citation{model.Cite("v2/subsector/report/", sector, d)}
				out = append(out, f)
			}
		}
	}
	// Suspension: fire only on notices dated today or later (new-notice diff).
	if raw, d, err := db.LatestSnapshot(ticker, "suspensions"); err == nil && raw != "" {
		var notices struct {
			Results []struct {
				Symbol         string `json:"symbol"`
				SuspensionDate string `json:"suspension_date"`
				Reason         string `json:"reason"`
			} `json:"results"`
		}
		if json.Unmarshal([]byte(raw), &notices) == nil {
			for _, sn := range notices.Results {
				if sn.SuspensionDate >= today {
					if f, ok := SuspensionWatch(ticker, sn.SuspensionDate, sn.Reason, true); ok {
						f.Citations = []model.Citation{
							model.Cite("v2/suspensions/", ticker, sn.SuspensionDate),
						}
						_ = d
						out = append(out, f)
					}
				}
			}
		}
	}
	return out
}

// isEarningsDate reports whether date falls within 1d of a stored quarterly
// report date (rule-4 exclusion for unusual volume).
func isEarningsDate(db *store.DB, ticker, date string) bool {
	var qdates []struct {
		ReportDate string `json:"report_date"`
		Date       string `json:"date"`
	}
	raw, _, err := db.LatestSnapshot(ticker, "quarterly-dates")
	if err != nil {
		return false
	}
	if json.Unmarshal([]byte(raw), &qdates) != nil {
		return false
	}
	for _, q := range qdates {
		for _, d := range []string{q.ReportDate, q.Date} {
			if len(d) >= 10 && len(date) >= 10 && daysBetween(d[:10], date[:10]) <= 1 {
				return true
			}
		}
	}
	return false
}

func daysBetween(a, b string) int {
	pa, ea := parseDay(a)
	pb, eb := parseDay(b)
	if ea != nil || eb != nil {
		return 999
	}
	h := pa.Sub(pb).Hours() / 24
	if h < 0 {
		h = -h
	}
	return int(h + 0.5)
}

func parseDay(s string) (t time.Time, err error) {
	return time.Parse("2006-01-02", s)
}

// firedToday enforces first-fire-only per (rule, ticker, date) for every
// rule: any event for the same rule+ticker+date suppresses re-fire.
func firedToday(db *store.DB, rule, ticker, today string) bool {
	evts, err := db.AlertEventsSince(today, ticker, 50)
	if err != nil {
		return false
	}
	for _, e := range evts {
		msg, _ := e["message"].(string)
		if e["ticker"] == ticker && e["date"] == today && contains(msg, ruleHints(rule)) {
			return true
		}
	}
	return false
}

// ruleHints maps a rule id to the message fragment its findings carry.
func ruleHints(rule string) string {
	switch rule {
	case RuleAccumulation:
		return "accumulation"
	case RuleForeignRev:
		return "reversal"
	case RuleInsiderSpike:
		return "insider"
	case RuleUnusualVolume:
		return "unusual volume"
	case RuleRotation:
		return "rotation"
	case RuleSuspension:
		return "suspend"
	default:
		return rule
	}
}

func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func lastOf(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[len(xs)-1]
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
