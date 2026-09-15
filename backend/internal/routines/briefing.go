package routines

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"flowsight/internal/model"
)

// Briefing (R1, 07:30 WIB daily): top-5 accumulation, foreign net per
// watchlist ticker, earnings + ex-div agenda next 7d, biggest 1d mover.
// Zero empty sections: every section renders, citing its snapshots.
func (e *Engine) Briefing(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	var cites []model.Citation
	var b strings.Builder
	fmt.Fprintf(&b, "FlowSight Morning Briefing — %s\n\n", t)

	type acc struct {
		ticker string
		net    float64
		n      int
	}
	var accs []acc
	for _, tk := range e.tickers() {
		nets, err := e.DB.NetBuySum5d(tk)
		if err != nil || len(nets) == 0 {
			continue
		}
		sum, n := 0.0, 0
		for _, v := range nets {
			if v > 0 {
				n++
				sum += v
			}
		}
		accs = append(accs, acc{tk, sum, n})
		cites = append(cites, model.Cite("v2/broker-summary/"+tk+"/top/", tk, t))
	}
	sort.Slice(accs, func(i, j int) bool { return accs[i].net > accs[j].net })
	b.WriteString("Top accumulation:\n")
	if len(accs) == 0 {
		b.WriteString("- no accumulation snapshots yet\n")
	} else {
		for i, a := range accs {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "%d. %s net %s (%d brokers)\n", i+1, a.ticker, fmtIDR(a.net), a.n)
		}
	}

	b.WriteString("\nForeign flow (last close):\n")
	for _, tk := range e.tickers() {
		dates, nets, err := e.DB.ForeignLast6(tk)
		if err != nil || len(nets) == 0 {
			fmt.Fprintf(&b, "- %s: no data\n", tk)
			continue
		}
		cites = append(cites, model.Cite("v2/foreign-flow/"+tk+"/", tk, dates[len(dates)-1]))
		fmt.Fprintf(&b, "- %s: %s on %s\n", tk, fmtIDR(nets[len(nets)-1]), dates[len(dates)-1])
	}

	e.agenda7d(&b, &cites)
	moverOfDay(&b, &cites, e.DB)

	out := capLines(b.String(), 25)
	payload := map[string]any{"date": t, "text": out}
	raw, _ := json.Marshal(payload)
	_ = e.DB.SaveBriefing(t, string(raw), mustJSON(cites))
	return out, cites, nil
}

// capLines enforces the ≤25-line delivery cap (header + rows survive).
func capLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n… (capped at 25 lines)\n"
}

// agenda7d appends the earnings + ex-div agenda for the next 7 days from
// stored quarterly-dates and corporate-actions snapshots.
func (e *Engine) agenda7d(b *strings.Builder, cites *[]model.Citation) {
	t := today()
	b.WriteString("\nAgenda (7d):\n")
	empty := true
	for _, tk := range e.tickers() {
		var qd []struct {
			ReportDate string `json:"report_date"`
			Date       string `json:"date"`
		}
		if raw, d, err := e.DB.LatestSnapshot(tk, "quarterly-dates"); err == nil {
			_ = json.Unmarshal([]byte(raw), &qd)
			for _, q := range qd {
				for _, rd := range []string{q.ReportDate, q.Date} {
					if nd := nextAfter(rd, 90); nd != "" && withinDays(t, nd, 7) {
						b.WriteString("- " + tk + " earnings ~" + nd + " (last " + rd[:10] + ")\n")
						*cites = append(*cites, model.Cite("v2/company/get_quarterly_financial_dates/"+tk+"/", tk, d))
						empty = false
					}
				}
			}
		}
		var ca struct {
			CorporateActions struct {
				UpcomingDividend []map[string]any `json:"upcoming_dividend"`
			} `json:"corporate_actions"`
			UpcomingDividend []map[string]any `json:"upcoming_dividend"`
		}
		if raw, d, err := e.DB.LatestSnapshot(tk, "corporate-actions"); err == nil {
			_ = json.Unmarshal([]byte(raw), &ca)
			evs := append(ca.CorporateActions.UpcomingDividend, ca.UpcomingDividend...)
			for _, ev := range evs {
				if dt := firstStr(ev, "ex_date", "exDate", "date"); withinDays(t, dt, 7) {
					b.WriteString("- " + tk + " ex-div " + dt[:10] + "\n")
					*cites = append(*cites, model.Cite("v2/company/corporate-actions/"+tk+"/", tk, d))
					empty = false
				}
			}
		}
	}
	if empty {
		b.WriteString("- no earnings/ex-div in the next 7d\n")
	}
}

// moverOfDay appends the biggest 1d mover with a one-line news cause.
func moverOfDay(b *strings.Builder, cites *[]model.Citation, db interface {
	LatestSnapshot(ticker, source string) (string, string, error)
	NewsSince(ticker, since string) ([]map[string]any, error)
}) {
	var mv struct {
		TopGainers map[string][]struct {
			Symbol string  `json:"symbol"`
			Name   string  `json:"name"`
			Change float64 `json:"price_change"`
		} `json:"top_gainers"`
	}
	raw, d, err := db.LatestSnapshot("IDX", "top-changes")
	if err != nil {
		b.WriteString("\nMover of the day: no snapshots yet\n")
		return
	}
	_ = json.Unmarshal([]byte(raw), &mv)
	best, bchg := "", 0.0
	for _, rows := range mv.TopGainers {
		for _, r := range rows {
			if r.Change > bchg {
				best, bchg = r.Symbol, r.Change
			}
		}
	}
	if best == "" {
		b.WriteString("\nMover of the day: no gainers stored\n")
		return
	}
	*cites = append(*cites, model.Cite("v2/companies/top-changes/", "IDX", d))
	cause := "no linked headline"
	tk := strings.ToUpper(strings.TrimSuffix(best, ".JK"))
	if arts, err := db.NewsSince(tk, "2000-01-01"); err == nil && len(arts) > 0 {
		if title, _ := arts[0]["title"].(string); title != "" {
			cause = title
		}
		*cites = append(*cites, model.Cite("v2/news/", tk, d))
	}
	b.WriteString("\nMover of the day: " + best + " +" + pct1(bchg) + " — " + cause + "\n")
}

func nextAfter(date string, days int) string {
	if len(date) < 10 {
		return ""
	}
	t, err := time.Parse("2006-01-02", date[:10])
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

func withinDays(today, date string, n int) bool {
	if len(today) < 10 || len(date) < 10 {
		return false
	}
	t0, e0 := time.Parse("2006-01-02", today[:10])
	t1, e1 := time.Parse("2006-01-02", date[:10])
	if e0 != nil || e1 != nil {
		return false
	}
	d := int(t1.Sub(t0).Hours() / 24)
	return d >= 0 && d <= n
}

func firstStr(ev map[string]any, keys ...string) string {
	for _, k := range keys {
		for ek, v := range ev {
			if strings.EqualFold(ek, k) {
				if s, ok := v.(string); ok && len(s) >= 10 {
					return s
				}
			}
		}
	}
	return ""
}

func pct1(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v*100), "0"), ".") + "%"
}

// Radar (R2): accumulation rule hits across the watchlist. First-fire
// only per (ticker, day): tickers with an accumulation event already
// recorded today are skipped.
func (e *Engine) Radar(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	var cites []model.Citation
	var lines []string
	fired := e.firedToday("accumulation", t)
	for _, tk := range e.tickers() {
		if fired[tk] {
			continue
		}
		nets, err := e.DB.NetBuySum5d(tk)
		if err != nil || len(nets) == 0 {
			continue
		}
		n, sum := 0, 0.0
		for _, v := range nets {
			if v > 0 {
				n++
				sum += v
			}
		}
		volMult := 0.0
		if vols, _, err := e.DB.DailyVolumes(tk, 21); err == nil && len(vols) > 0 {
			if n := len(vols); n >= 2 {
				if a := avgF(vols[:n-1]); a > 0 {
					volMult = vols[n-1] / a
				}
			}
		}
		if n >= 3 && volMult > 1.5 {
			lines = append(lines, fmt.Sprintf("%s: %d brokers net %s, vol %.1fx — /report/%s",
				tk, n, fmtIDR(sum), volMult, tk))
			cites = append(cites, model.Cite("v2/broker-summary/"+tk+"/top/", tk, t))
		}
	}
	if len(lines) == 0 {
		return "radar: no accumulation signals this cycle", cites, nil
	}
	return "Accumulation Radar:\n- " + strings.Join(lines, "\n- "), cites, nil
}

// ReversalWatch (R3): 5d cumulative one way, last day the other way at
// >2x the trailing 5d daily average (both directions, per ROUTINES.md).
func (e *Engine) ReversalWatch(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	var cites []model.Citation
	var lines []string
	for _, tk := range e.tickers() {
		dates, nets, err := e.DB.ForeignLast6(tk)
		if err != nil || len(nets) < 6 {
			continue
		}
		sum5, absAvg := 0.0, 0.0
		for _, v := range nets[:5] {
			sum5 += v
			absAvg += absF(v)
		}
		absAvg /= 5
		last := nets[5]
		switch {
		case absAvg > 0 && sum5 < 0 && last > 0 && last > 2*absAvg:
			lines = append(lines, fmt.Sprintf("%s: first inflow %s after 5 selling days (%s)",
				tk, fmtIDR(last), dates[5]))
			cites = append(cites, model.Cite("v2/foreign-flow/"+tk+"/", tk, dates[5]))
		case absAvg > 0 && sum5 > 0 && last < 0 && -last > 2*absAvg:
			lines = append(lines, fmt.Sprintf("%s: outflow %s after 5 buying days (%s)",
				tk, fmtIDR(-last), dates[5]))
			cites = append(cites, model.Cite("v2/foreign-flow/"+tk+"/", tk, dates[5]))
		}
	}
	if len(lines) == 0 {
		return "reversal watch: no flips this cycle", cites, nil
	}
	return "Foreign Reversal Watch:\n- " + strings.Join(lines, "\n- "), cites, nil
}

// InsiderTape (R4): buy volume > 2x 30d avg, or >=3 distinct insiders in
// 7d (per ROUTINES.md rule 3).
func (e *Engine) InsiderTape(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	var cites []model.Citation
	var lines []string
	for _, tk := range e.tickers() {
		avg30 := e.DB.FilingAvg30(tk)
		last := e.DB.LatestBuyVolume(tk)
		cluster := e.DB.DistinctInsiders7d(tk)
		switch {
		case avg30 > 0 && last > 2*avg30:
			lines = append(lines, fmt.Sprintf("%s: insider buy %.0f vs 30d avg %.0f (%.1fx)",
				tk, last, avg30, last/avg30))
			cites = append(cites, model.Cite("v2/filings/", tk, t))
		case cluster >= 3:
			lines = append(lines, fmt.Sprintf("%s: %d distinct insiders buying in 7d", tk, cluster))
			cites = append(cites, model.Cite("v2/filings/", tk, t))
		}
	}
	if len(lines) == 0 {
		return "insider tape: no filings in window", cites, nil
	}
	return "Insider Tape:\n- " + strings.Join(lines, "\n- "), cites, nil
}

// firedToday returns tickers with an event whose message contains rule
// already recorded today (first-fire guard).
func (e *Engine) firedToday(rule, today string) map[string]bool {
	out := map[string]bool{}
	evts, err := e.DB.AlertEventsSince(today, "", 100)
	if err != nil {
		return out
	}
	for _, ev := range evts {
		msg, _ := ev["message"].(string)
		if tk, _ := ev["ticker"].(string); ev["date"] == today && strings.Contains(msg, rule) {
			out[tk] = true
		}
	}
	return out
}

// EarningsCountdown (R5): fires at H-7/H-3/H-1 from stored quarterly dates,
// with the 8-quarter revenue/earnings mini-trend attached.
func (e *Engine) EarningsCountdown(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	var cites []model.Citation
	var lines []string
	for _, tk := range e.tickers() {
		raw, d, err := e.DB.LatestSnapshot(tk, "quarterly-dates")
		if err != nil {
			continue
		}
		var qd []struct {
			ReportDate string `json:"report_date"`
			Date       string `json:"date"`
		}
		_ = json.Unmarshal([]byte(raw), &qd)
		for _, q := range qd {
			for _, rd := range []string{q.ReportDate, q.Date} {
				next := nextAfter(rd, 90)
				if next == "" {
					continue
				}
				h := daysUntil(t, next)
				if h == 7 || h == 3 || h == 1 {
					lines = append(lines, fmt.Sprintf("%s earnings H-%d (%s)%s", tk, h, next, e.trend8Q(tk)))
					cites = append(cites, model.Cite("v2/company/get_quarterly_financial_dates/"+tk+"/", tk, d))
				}
			}
		}
	}
	if len(lines) == 0 {
		return "earnings countdown: nothing at H-7/H-3/H-1 today", cites, nil
	}
	return "Earnings Countdown:\n- " + strings.Join(lines, "\n- "), cites, nil
}

// trend8Q renders the 8-quarter revenue/earnings mini-trend for one ticker.
func (e *Engine) trend8Q(tk string) string {
	raw, _, err := e.DB.LatestSnapshot(tk, "financials-quarterly")
	if err != nil {
		return ""
	}
	var rows []struct {
		Revenue  *float64 `json:"revenue"`
		Earnings *float64 `json:"earnings"`
	}
	if json.Unmarshal([]byte(raw), &rows) != nil || len(rows) < 2 {
		return ""
	}
	if len(rows) > 8 {
		rows = rows[len(rows)-8:]
	}
	f := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	rev, earn := "", ""
	if a, b := f(rows[0].Revenue), f(rows[len(rows)-1].Revenue); a > 0 {
		rev = fmt.Sprintf(" rev %+.0f%%", (b-a)/a*100)
	}
	if a, b := f(rows[0].Earnings), f(rows[len(rows)-1].Earnings); a != 0 {
		earn = fmt.Sprintf(" earn %+.0f%%", (b-a)/absF(a)*100)
	}
	return " [" + strings.TrimSpace(rev+earn) + "]"
}

// DividendCalendar (R6): fires at H-14/H-3 with DPS, est. yield, payout flag.
func (e *Engine) DividendCalendar(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	var cites []model.Citation
	var lines []string
	for _, tk := range e.tickers() {
		raw, d, err := e.DB.LatestSnapshot(tk, "corporate-actions")
		if err != nil {
			continue
		}
		var ca struct {
			CorporateActions struct {
				UpcomingDividend []map[string]any `json:"upcoming_dividend"`
			} `json:"corporate_actions"`
			UpcomingDividend []map[string]any `json:"upcoming_dividend"`
		}
		_ = json.Unmarshal([]byte(raw), &ca)
		evs := append(ca.CorporateActions.UpcomingDividend, ca.UpcomingDividend...)
		closePx, _, _ := e.DB.LatestClose(tk)
		payout := e.payoutOf(tk)
		for _, ev := range evs {
			dt := firstStr(ev, "ex_date", "exDate", "date")
			if dt == "" {
				continue
			}
			h := daysUntil(t, dt[:10])
			if h != 14 && h != 3 {
				continue
			}
			dps := numOf(ev, "dividend_per_share", "dps", "cash_dividend")
			y := ""
			if dps > 0 && closePx > 0 {
				y = fmt.Sprintf(" DPS %.0f yield %.1f%%", dps, dps/closePx*100)
			}
			flag := ""
			if payout > 0.8 {
				flag = " aggressive-payout"
			}
			lines = append(lines, fmt.Sprintf("%s ex-div H-%d (%s)%s%s", tk, h, dt[:10], y, flag))
			cites = append(cites, model.Cite("v2/company/corporate-actions/"+tk+"/", tk, d))
		}
	}
	if len(lines) == 0 {
		return "dividend calendar: nothing at H-14/H-3 today", cites, nil
	}
	return "Dividend Calendar:\n- " + strings.Join(lines, "\n- "), cites, nil
}

// payoutOf reads the dividend payout ratio from the stored company report.
func (e *Engine) payoutOf(tk string) float64 {
	raw, _, err := e.DB.LatestSnapshot(tk, "company-report")
	if err != nil {
		return 0
	}
	var rep map[string]any
	_ = json.Unmarshal([]byte(raw), &rep)
	p := numOf(rep, "payout_ratio", "dividend_payout")
	if p > 1 {
		p /= 100
	}
	return p
}

// numOf digs the first present numeric key (case-insensitive, nested).
func numOf(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := findNum(m, k); ok {
			return v
		}
	}
	return 0
}

func findNum(m map[string]any, key string) (float64, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			if f, ok := asFloat(v); ok {
				return f, true
			}
		}
		if sub, ok := v.(map[string]any); ok {
			if f, ok := findNum(sub, key); ok {
				return f, true
			}
		}
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// daysUntil returns whole days from today to date (negative if past).
func daysUntil(today, date string) int {
	if len(today) < 10 || len(date) < 10 {
		return -999
	}
	t0, e0 := time.Parse("2006-01-02", today[:10])
	t1, e1 := time.Parse("2006-01-02", date[:10])
	if e0 != nil || e1 != nil {
		return -999
	}
	return int(t1.Sub(t0).Hours() / 24)
}

// WeekendReview (R7, Sat 09:00): week movers + drivers + open risks + agenda.
func (e *Engine) WeekendReview(ctx context.Context) (string, []model.Citation, error) {
	_ = ctx
	t := today()
	text, cites, err := e.Briefing(ctx)
	if err != nil {
		return "", cites, err
	}
	out := "Weekend Review — week of " + t + "\n\n" + text
	return out, cites, nil
}

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

func avgF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
