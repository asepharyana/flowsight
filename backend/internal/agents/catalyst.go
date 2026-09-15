package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// AnalyzeCatalyst (A6) builds the catalyst calendar: ex-div, earnings, AGM.
// Opportunity score = yield x certainty - earnings-risk, 0-100.
func AnalyzeCatalyst(ctx context.Context, d Deps, ticker string) model.AgentResult {
	ticker = strings.ToUpper(ticker)
	now := d.Now
	if now.IsZero() {
		now = time.Now()
	}
	res := model.AgentResult{Summary: "no catalyst snapshots available"}

	var wrapped struct {
		Symbol           string                   `json:"symbol"`
		CorporateActions sectors.CorporateActions `json:"corporate_actions"`
		// Unwrapped shape (client return) also accepted.
		Dividend         []sectors.DividendEvent `json:"dividend"`
		UpcomingDividend []sectors.DividendEvent `json:"upcoming_dividend"`
		AGM              []sectors.DividendEvent `json:"agm"`
		StockSplit       []sectors.DividendEvent `json:"stock_split"`
	}
	var ipo sectors.ListingPerformance
	ipoDate, ipoOK := payload(d.DB, ticker, "listing-performance", &ipo)
	actDate, actOK := payload(d.DB, ticker, "corporate-actions", &wrapped)
	actions := wrapped.CorporateActions
	if len(actions.UpcomingDividend) == 0 {
		actions.UpcomingDividend = wrapped.UpcomingDividend
	}
	if len(actions.Dividend) == 0 {
		actions.Dividend = wrapped.Dividend
	}
	if len(actions.AGM) == 0 {
		actions.AGM = wrapped.AGM
	}
	if len(actions.StockSplit) == 0 {
		actions.StockSplit = wrapped.StockSplit
	}
	if ipoOK {
		res.Citations = append(res.Citations, model.Cite("v2/listing-performance/"+ticker+"/", ticker, ipoDate))
	}
	var qdates []sectors.QuarterlyDate
	qdDate, qdOK := payload(d.DB, ticker, "quarterly-dates", &qdates)
	if !qdOK || len(qdates) == 0 {
		// Legacy universe shape: [{symbol, date, year}] from
		// companies/quarterly-financial-dates.
		var uni []sectors.QuarterlyDateRow
		if ud, uok := payload(d.DB, ticker, "quarterly-dates", &uni); uok {
			qdDate, qdOK = ud, true
			for _, r := range uni {
				qdates = append(qdates, sectors.QuarterlyDate{ReportDate: r.Date})
			}
		}
	}
	if !actOK && !qdOK {
		return res
	}
	if actOK {
		res.Citations = append(res.Citations, model.Cite("v2/company/corporate-actions/"+ticker+"/", ticker, actDate))
	}
	if qdOK {
		res.Citations = append(res.Citations, model.Cite("v2/company/get_quarterly_financial_dates/"+ticker+"/", ticker, qdDate))
	}

	type cal struct {
		event string
		date  string
		days  int
		extra string
	}
	var rows []cal

	closePx, _, _ := d.DB.LatestClose(ticker)
	for _, ev := range actions.UpcomingDividend {
		if dt := strAt(ev, "ex_date", "exDate", "date"); len(dt) >= 10 {
			if t, err := time.Parse("2006-01-02", dt[:10]); err == nil {
				days := int(t.Sub(now).Hours() / 24)
				rows = append(rows, cal{"ex-div", dt[:10], days, yieldLine(ev, closePx)})
			}
		}
	}
	for _, ev := range actions.Dividend {
		if dt := strAt(ev, "ex_date", "exDate", "date"); len(dt) >= 10 {
			if t, err := time.Parse("2006-01-02", dt[:10]); err == nil && t.After(now.AddDate(0, 0, -370)) {
				days := int(t.Sub(now).Hours() / 24)
				if days >= -30 { // recent history for payout context
					rows = append(rows, cal{"div-paid", dt[:10], days, yieldLine(ev, closePx)})
				}
			}
		}
	}
	for _, ev := range actions.AGM {
		if dt := strAt(ev, "date", "agm_date"); len(dt) >= 10 {
			if t, err := time.Parse("2006-01-02", dt[:10]); err == nil {
				if days := int(t.Sub(now).Hours() / 24); days >= 0 {
					rows = append(rows, cal{"AGM", dt[:10], days, ""})
				}
			}
		}
	}
	for _, ev := range actions.StockSplit {
		if dt := strAt(ev, "date", "ex_date", "split_date"); len(dt) >= 10 {
			if t, err := time.Parse("2006-01-02", dt[:10]); err == nil {
				if days := int(t.Sub(now).Hours() / 24); days >= -30 {
					rows = append(rows, cal{"split", dt[:10], days, ratioLine(ev)})
				}
			}
		}
	}
	// IPO-window context for recent listings (<=365d): anniversary + 30d drift.
	if ipoOK && len(ipo.ListingDate) >= 10 {
		if t, err := time.Parse("2006-01-02", ipo.ListingDate[:10]); err == nil {
			age := int(now.Sub(t).Hours() / 24)
			if age >= 0 && age <= 365 {
				rows = append(rows, cal{"IPO-window", ipo.ListingDate[:10], -age, ipoLine(&ipo)})
			}
		}
	}
	// Next earnings estimate: last report + ~90d unless universe dates show newer.
	if len(qdates) > 0 {
		sort.Slice(qdates, func(i, j int) bool { return qdates[i].ReportDate > qdates[j].ReportDate })
		last := qdates[0].ReportDate
		if len(last) >= 10 {
			if t, err := time.Parse("2006-01-02", last[:10]); err == nil {
				next := t.AddDate(0, 0, 90)
				rows = append(rows, cal{"earnings-est", next.Format("2006-01-02"), int(next.Sub(now).Hours() / 24), "from last " + last[:10]})
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].days < rows[j].days })

	opp := 0.0
	var lines []string
	for _, r := range rows {
		h := fmt.Sprintf("H%+d", r.days)
		if r.days >= 0 {
			h = fmt.Sprintf("H-%d", r.days)
		}
		line := fmt.Sprintf("%s %s %s", r.event, r.date, h)
		if r.extra != "" {
			line += " (" + r.extra + ")"
		}
		lines = append(lines, line)
		// Near-term certain events lift the opportunity score.
		if r.days >= 0 && r.days <= 30 {
			w := 30.0
			if r.event == "ex-div" {
				w = 45
			}
			opp += w * (1 - float64(r.days)/30)
		}
	}
	// Earnings within 7d adds risk (results can invalidate the thesis).
	for _, r := range rows {
		if r.event == "earnings-est" && r.days >= 0 && r.days <= 7 {
			opp -= 15
			res.Flags = append(res.Flags, "earnings-risk")
		}
	}
	res.Score = clampScore(opp, 0, 100)

	if len(lines) == 0 {
		res.Summary = "no upcoming catalysts in window"
	} else {
		res.Values = []model.Value{{
			Label:     "catalyst calendar",
			Display:   strings.Join(lines, " | "),
			Citations: res.Citations,
		}}
		res.Summary = fmt.Sprintf("%d catalysts, opportunity %.0f", len(lines), res.Score)
	}
	res.Extra = map[string]any{"calendar": lines}
	return res
}

// strAt returns the first present string key.
func strAt(ev map[string]any, keys ...string) string {
	for _, k := range keys {
		for ek, v := range ev {
			if strings.EqualFold(ek, k) {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// ratioLine renders "a-for-b" when a split ratio is present.
func ratioLine(ev map[string]any) string {
	a := numAt(ev, "ratio", "split_ratio", "ratio_from")
	b := numAt(ev, "ratio_to", "ratio_denominator", "new_shares")
	if a > 0 && b > 0 {
		return fmt.Sprintf("%.0f-for-%.0f", a, b)
	}
	return "split"
}

// ipoLine renders listing age + 30d drift when present.
func ipoLine(ipo *sectors.ListingPerformance) string {
	if ipo.Chg30d != nil {
		return fmt.Sprintf("30d %+.1f%%", *ipo.Chg30d*100)
	}
	return "recent listing"
}

// yieldLine renders "DPS x, yield y%" when figures are present.
func yieldLine(ev map[string]any, closePx float64) string {
	dps := numAt(ev, "dividend_per_share", "dps", "cash_dividend")
	if dps <= 0 {
		return ""
	}
	if closePx > 0 {
		return fmt.Sprintf("DPS %.0f, yield %.1f%%", dps, dps/closePx*100)
	}
	return fmt.Sprintf("DPS %.0f", dps)
}
