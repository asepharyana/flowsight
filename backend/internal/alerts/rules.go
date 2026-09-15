// Package alerts evaluates 6 detection rules (docs/PLAN.md §11) over stored
// snapshots. Rules are pure functions over typed inputs so fixture tests can
// prove each fires (or stays silent) deterministically.
package alerts

import (
	"strings"

	"flowsight/internal/model"
)

// Rule IDs for the 6 v1 detectors.
const (
	RuleAccumulation  = "accumulation"
	RuleForeignRev    = "foreign-reversal"
	RuleInsiderSpike  = "insider-spike"
	RuleUnusualVolume = "unusual-volume"
	RuleRotation      = "sector-rotation"
	RuleSuspension    = "suspension-watch"
)

// Finding is one rule hit with human text + evidence + real citations.
type Finding struct {
	Rule      string
	Ticker    string
	Message   string
	Context   map[string]any
	Citations []model.Citation
}

// Accumulation fires on >=3 brokers net-buy 5d + volume > 1.5x 20d avg.
func Accumulation(ticker string, netByBroker map[string]float64, volMult float64) (Finding, bool) {
	n := 0
	sum := 0.0
	for _, v := range netByBroker {
		if v > 0 {
			n++
			sum += v
		}
	}
	if n >= 3 && volMult > 1.5 {
		return Finding{Rule: RuleAccumulation, Ticker: ticker,
			Message: ticker + " accumulation: " + itoa(n) + " brokers net-buy, volume spike",
			Context: map[string]any{"brokers": n, "net_sum": sum, "vol_mult": volMult}}, true
	}
	return Finding{}, false
}

// ForeignReversal fires on 5d cumulative outflow then 1d inflow with the last
// day magnitude > 2x trailing 5d daily average (or the mirror).
func ForeignReversal(ticker string, last6 []float64) (Finding, bool) {
	if len(last6) < 6 {
		return Finding{}, false
	}
	prev5, last := last6[:5], last6[5]
	sum5 := 0.0
	absAvg := 0.0
	for _, v := range prev5 {
		sum5 += v
		absAvg += abs(v)
	}
	absAvg /= 5
	if absAvg == 0 {
		return Finding{}, false
	}
	if sum5 < 0 && last > 0 && last > 2*absAvg {
		return Finding{Rule: RuleForeignRev, Ticker: ticker,
			Message: ticker + " foreign reversal: outflow flipped to inflow",
			Context: map[string]any{"sum5": sum5, "last": last}}, true
	}
	if sum5 > 0 && last < 0 && -last > 2*absAvg {
		return Finding{Rule: RuleForeignRev, Ticker: ticker,
			Message: ticker + " foreign reversal: inflow flipped to outflow",
			Context: map[string]any{"sum5": sum5, "last": last}}, true
	}
	return Finding{}, false
}

// InsiderSpike fires on director/major buy volume > 2x 30d avg, or >=3
// distinct insiders buying in 7d.
func InsiderSpike(ticker string, buyVol, avg30 float64, distinct7d int) (Finding, bool) {
	if avg30 > 0 && buyVol > 2*avg30 {
		return Finding{Rule: RuleInsiderSpike, Ticker: ticker,
			Message: ticker + " insider spike: buy volume above 2x 30d avg",
			Context: map[string]any{"volume": buyVol, "avg30": avg30, "mult": buyVol / avg30}}, true
	}
	if distinct7d >= 3 {
		return Finding{Rule: RuleInsiderSpike, Ticker: ticker,
			Message: ticker + " insider cluster: 3+ insiders buying in 7d",
			Context: map[string]any{"distinct": distinct7d}}, true
	}
	return Finding{}, false
}

// UnusualVolume fires on >3x 20d avg when the date is not an earnings date.
func UnusualVolume(ticker, date string, mult float64, isEarningsDate bool) (Finding, bool) {
	if mult > 3 && !isEarningsDate {
		return Finding{Rule: RuleUnusualVolume, Ticker: ticker,
			Message: ticker + " unusual volume on " + date,
			Context: map[string]any{"mult": mult, "date": date}}, true
	}
	return Finding{}, false
}

// SectorRotation fires when a subsector net flow flips sign week-over-week.
func SectorRotation(sector string, prev, cur float64) (Finding, bool) {
	if (prev < 0 && cur > 0) || (prev > 0 && cur < 0) {
		dir := "inflow"
		if cur < 0 {
			dir = "outflow"
		}
		return Finding{Rule: RuleRotation, Ticker: sector,
			Message: "rotation: " + sector + " flipped to " + dir,
			Context: map[string]any{"prev": prev, "cur": cur}}, true
	}
	return Finding{}, false
}

// SuspensionWatch fires on any new suspension notice for a watchlist ticker.
func SuspensionWatch(ticker, date, reason string, onWatchlist bool) (Finding, bool) {
	if reason == "" && date == "" {
		return Finding{}, false
	}
	if onWatchlist {
		return Finding{Rule: RuleSuspension, Ticker: ticker,
			Message: ticker + " suspended: " + reason,
			Context: map[string]any{"date": date, "reason": reason}}, true
	}
	return Finding{}, false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// MatchRule reports whether a user rule JSON targets a finding (simple
// substring match on rule id or ticker; empty rule matches all).
func MatchRule(ruleJSON string, f Finding) bool {
	if ruleJSON == "" || ruleJSON == "{}" {
		return true
	}
	r := strings.ToLower(ruleJSON)
	return strings.Contains(r, strings.ToLower(f.Rule)) ||
		strings.Contains(r, strings.ToLower(f.Ticker)) ||
		strings.Contains(r, `"all"`)
}
