package sectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// param helpers keep every wrapper narrow by default (credit traps in
// docs/API-REFERENCE.md): sections explicit, classifications+periods minimal,
// n_quarters bounded, free-float filters exclusive.

func dateRange(start, end string, clamp func(time.Time, time.Time) (time.Time, time.Time)) (string, string) {
	if start == "" || end == "" {
		e := time.Now()
		s := e.AddDate(0, 0, -7)
		if start == "" && end != "" {
			if t, err := time.Parse("2006-01-02", end); err == nil {
				e, s = t, t.AddDate(0, 0, -7)
			}
		}
		return s.Format("2006-01-02"), e.Format("2006-01-02")
	}
	ts, err1 := time.Parse("2006-01-02", start)
	te, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return start, end
	}
	ts, te = clamp(ts, te)
	return ts.Format("2006-01-02"), te.Format("2006-01-02")
}

func decode(data []byte, v any, endpoint string) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("sectors: decode %s: %w", endpoint, err)
	}
	return nil
}

// --- Brokers: the moat ---

// BrokersRegistry returns the broker registry (cached daily by the scheduler).
func (c *Client) BrokersRegistry(ctx context.Context, cohort, origin string) ([]BrokerRegistryRow, error) {
	q := url.Values{}
	if cohort != "" && cohort != "all" {
		q.Set("cohort", cohort)
	}
	if origin != "" && origin != "all" {
		q.Set("origin", origin)
	}
	raw, err := c.Get(ctx, "brokers/", q)
	if err != nil {
		return nil, err
	}
	var rows []BrokerRegistryRow
	if err := decode(raw, &rows, "v2/brokers/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// BrokersTop returns the daily broker ranking.
func (c *Client) BrokersTop(ctx context.Context, date, metric string, n int, origin, cohort string) ([]TopBrokerRow, error) {
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	if metric == "" {
		metric = "net"
	}
	q.Set("metric", metric)
	if n <= 0 || n > 90 {
		n = 20
	}
	q.Set("n_brokers", fmt.Sprint(n))
	if origin != "" {
		q.Set("origin", origin)
	}
	if cohort != "" {
		q.Set("cohort", cohort)
	}
	raw, err := c.Get(ctx, "brokers/top/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []TopBrokerRow `json:"results"`
	}
	if err := decode(raw, &resp, "v2/brokers/top/"); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// BrokerActivity returns all (stock,day) rows for one broker (window ≤14d).
func (c *Client) BrokerActivity(ctx context.Context, brokerCode, symbol, start, end string) ([]BrokerRow, error) {
	start, end = dateRange(start, end, ClampBrokerWindow)
	q := url.Values{"start": {start}, "end": {end}}
	if symbol != "" {
		q.Set("symbol", strings.ToUpper(symbol))
	}
	raw, err := c.Get(ctx, "broker-activity/"+strings.ToUpper(brokerCode)+"/", q)
	if err != nil {
		return nil, err
	}
	var rows []BrokerRow
	if err := decode(raw, &rows, "v2/broker-activity/{broker_code}/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// BrokerActivityTop returns top accumulations/distributions for one broker.
func (c *Client) BrokerActivityTop(ctx context.Context, brokerCode, start, end string, n int) (accum, distrib []BrokerRow, err error) {
	start, end = dateRange(start, end, ClampBrokerWindow)
	q := url.Values{"start": {start}, "end": {end}}
	if n > 0 {
		q.Set("n_brokers", fmt.Sprint(n))
	}
	raw, err := c.Get(ctx, "broker-activity/"+strings.ToUpper(brokerCode)+"/top/", q)
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		TopAccumulations []BrokerRow `json:"top_accumulations"`
		TopDistributions []BrokerRow `json:"top_distributions"`
		TopAccumulation  []BrokerRow `json:"top_accumulation"`
		TopDistribution  []BrokerRow `json:"top_distribution"`
		Accumulations    []BrokerRow `json:"accumulations"`
		Distributions    []BrokerRow `json:"distributions"`
		Data             []BrokerRow `json:"data"`
		Results          []BrokerRow `json:"results"`
	}
	if err := decode(raw, &resp, "v2/broker-activity/{code}/top/"); err != nil {
		return nil, nil, err
	}
	accum = append(append(append(resp.TopAccumulations, resp.TopAccumulation...), resp.Accumulations...), resp.Data...)
	distrib = append(append(resp.TopDistributions, resp.TopDistribution...), resp.Distributions...)
	if len(accum) == 0 && len(resp.Results) > 0 {
		accum = resp.Results
	}
	return accum, distrib, nil
}

// BrokerSummary returns per-broker daily rows for one ticker (window ≤14d).
func (c *Client) BrokerSummary(ctx context.Context, symbol, brokerCode, start, end string) ([]BrokerRow, error) {
	start, end = dateRange(start, end, ClampBrokerWindow)
	q := url.Values{"start": {start}, "end": {end}}
	if brokerCode != "" {
		q.Set("broker_code", strings.ToUpper(brokerCode))
	}
	raw, err := c.Get(ctx, "broker-summary/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	// Shape varies (rows array or {dates: {broker: rows}}); normalize both.
	var rows []BrokerRow
	if err := json.Unmarshal(raw, &rows); err == nil {
		return rows, nil
	}
	var byDate map[string]map[string][]BrokerRow
	if err := json.Unmarshal(raw, &byDate); err == nil {
		for _, brokers := range byDate {
			for _, r := range brokers {
				rows = append(rows, r...)
			}
		}
		return rows, nil
	}
	var wrapped struct {
		Data    []BrokerRow `json:"data"`
		Results []BrokerRow `json:"results"`
	}
	if err := decode(raw, &wrapped, "v2/broker-summary/{symbol}/"); err != nil {
		return nil, err
	}
	return append(wrapped.Data, wrapped.Results...), nil
}

// BrokerSummaryTop returns top buyers/sellers for one ticker (2 credits).
func (c *Client) BrokerSummaryTop(ctx context.Context, symbol, start, end string, n int, origin, cohort string) (*BrokerSummaryTop, error) {
	start, end = dateRange(start, end, ClampBrokerWindow)
	q := url.Values{"start": {start}, "end": {end}}
	if n > 0 {
		q.Set("n_brokers", fmt.Sprint(n))
	}
	if origin != "" {
		q.Set("origin", origin)
	}
	if cohort != "" {
		q.Set("cohort", cohort)
	}
	raw, err := c.Get(ctx, "broker-summary/"+strings.ToUpper(symbol)+"/top/", q)
	if err != nil {
		return nil, err
	}
	var resp BrokerSummaryTop
	if err := decode(raw, &resp, "v2/broker-summary/{symbol}/top/"); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ForeignFlow returns the daily net foreign inflow series (window ≤90d).
func (c *Client) ForeignFlow(ctx context.Context, symbol, start, end string) (*ForeignFlow, error) {
	start, end = dateRange(start, end, ClampSeriesWindow)
	q := url.Values{"start": {start}, "end": {end}}
	raw, err := c.Get(ctx, "foreign-flow/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var resp ForeignFlow
	if err := decode(raw, &resp, "v2/foreign-flow/{symbol}/"); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Company core ---

// CompanyReport fetches explicit sections only (1 credit/section).
func (c *Client) CompanyReport(ctx context.Context, symbol string, sections []string) (map[string]any, error) {
	if len(sections) == 0 {
		sections = []string{"overview", "valuation"}
	}
	q := url.Values{"sections": {strings.Join(sections, ",")}}
	raw, err := c.Get(ctx, "company/report/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := decode(raw, &out, "v2/company/report/{symbol}/"); err != nil {
		return nil, err
	}
	return out, nil
}

// Quarterly fetches up to n financial quarters (1 credit/quarter, n≤8).
func (c *Client) Quarterly(ctx context.Context, symbol string, n int) ([]QuarterRow, error) {
	if n <= 0 {
		n = 4
	}
	if n > 8 {
		n = 8
	}
	q := url.Values{"n_quarters": {fmt.Sprint(n)}}
	raw, err := c.Get(ctx, "financials/quarterly/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var rows []QuarterRow
	if err := decode(raw, &rows, "v2/financials/quarterly/{symbol}/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// CorporateActions fetches splits/rights/warrants/AGM/dividends for one ticker.
func (c *Client) CorporateActions(ctx context.Context, symbol string) (*CorporateActions, error) {
	raw, err := c.Get(ctx, "company/corporate-actions/"+strings.ToUpper(symbol)+"/", url.Values{})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Symbol           string `json:"symbol"`
		CorporateActions struct {
			Dividend         []DividendEvent `json:"dividend"`
			UpcomingDividend []DividendEvent `json:"upcoming_dividend"`
			AGM              []DividendEvent `json:"agm"`
			StockSplit       []DividendEvent `json:"stock_split"`
		} `json:"corporate_actions"`
	}
	if err := decode(raw, &resp, "v2/company/corporate-actions/{symbol}/"); err != nil {
		return nil, err
	}
	return &CorporateActions{
		Symbol:           resp.Symbol,
		Dividend:         resp.CorporateActions.Dividend,
		UpcomingDividend: resp.CorporateActions.UpcomingDividend,
		AGM:              resp.CorporateActions.AGM,
		StockSplit:       resp.CorporateActions.StockSplit,
	}, nil
}

// --- Universe / market ---

// ClosePage fetches one page of the full-universe close sweep.
func (c *Client) ClosePage(ctx context.Context, date string, limit, offset int) ([]CloseRow, int, error) {
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	q.Set("limit", fmt.Sprint(limit))
	q.Set("offset", fmt.Sprint(offset))
	raw, err := c.Get(ctx, "close/", q)
	if err != nil {
		return nil, 0, err
	}
	var resp struct {
		Results    []CloseRow `json:"results"`
		Pagination struct {
			Total int `json:"total"`
			Count int `json:"count"`
		} `json:"pagination"`
	}
	if err := decode(raw, &resp, "v2/close/"); err != nil {
		return nil, 0, err
	}
	return resp.Results, resp.Pagination.Total, nil
}

// QuarterlyDatesSince polls newly-reported companies incrementally.
func (c *Client) QuarterlyDatesSince(ctx context.Context, since string, limit, offset int) ([]QuarterlyDateRow, error) {
	q := url.Values{}
	if since != "" {
		q.Set("since", since)
	}
	if limit <= 0 || limit > 30 {
		limit = 30
	}
	q.Set("limit", fmt.Sprint(limit))
	q.Set("offset", fmt.Sprint(offset))
	raw, err := c.Get(ctx, "companies/quarterly-financial-dates/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []QuarterlyDateRow `json:"results"`
		Data    []QuarterlyDateRow `json:"data"`
	}
	if err := decode(raw, &resp, "v2/companies/quarterly-financial-dates/"); err != nil {
		return nil, err
	}
	return append(resp.Results, resp.Data...), nil
}

// Daily returns the price+volume+MCap series for one ticker (window ≤90d).
func (c *Client) Daily(ctx context.Context, symbol, start, end string) ([]DailyBar, error) {
	start, end = dateRange(start, end, ClampSeriesWindow)
	q := url.Values{"start": {start}, "end": {end}}
	raw, err := c.Get(ctx, "daily/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var rows []DailyBar
	if err := decode(raw, &rows, "v2/daily/{symbol}/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// IdxTotal returns the IHSG total-MCap trend (macro context).
func (c *Client) IdxTotal(ctx context.Context, start, end string) ([]IdxTotalPoint, error) {
	start, end = dateRange(start, end, ClampSeriesWindow)
	q := url.Values{"start": {start}, "end": {end}}
	raw, err := c.Get(ctx, "idx-total/", q)
	if err != nil {
		return nil, err
	}
	var rows []IdxTotalPoint
	if err := decode(raw, &rows, "v2/idx-total/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// TopChanges requests minimal class×period combos only (1 credit per combo).
func (c *Client) TopChanges(ctx context.Context, classifications, periods []string, subSector string, n int) (gainers, losers map[string][]MoverRow, err error) {
	if len(classifications) == 0 {
		classifications = []string{"top_gainers"}
	}
	if len(periods) == 0 {
		periods = []string{"1d"}
	}
	q := url.Values{
		"classifications": {strings.Join(classifications, ",")},
		"periods":         {strings.Join(periods, ",")},
	}
	if subSector != "" {
		q.Set("sub_sector", subSector)
	}
	if n > 0 {
		q.Set("n_stock", fmt.Sprint(n))
	}
	raw, err := c.Get(ctx, "companies/top-changes/", q)
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		TopGainers map[string][]MoverRow `json:"top_gainers"`
		TopLosers  map[string][]MoverRow `json:"top_losers"`
	}
	if err := decode(raw, &resp, "v2/companies/top-changes/"); err != nil {
		return nil, nil, err
	}
	return resp.TopGainers, resp.TopLosers, nil
}

// MostTraded returns relative volume leaders.
func (c *Client) MostTraded(ctx context.Context, start, end, subSector string, n int) ([]map[string]any, error) {
	start, end = dateRange(start, end, ClampSeriesWindow)
	q := url.Values{"start": {start}, "end": {end}}
	if subSector != "" {
		q.Set("sub_sector", subSector)
	}
	if n > 0 {
		q.Set("n_stock", fmt.Sprint(n))
	}
	raw, err := c.Get(ctx, "most-traded/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []map[string]any `json:"results"`
		Data    []map[string]any `json:"data"`
	}
	if err := decode(raw, &resp, "v2/most-traded/"); err != nil {
		return nil, err
	}
	return append(resp.Results, resp.Data...), nil
}

// --- Events (incremental) ---

// News fetches articles incrementally (extension idx/mining exclusive).
// Windows clamp to 90d like other series endpoints.
func (c *Client) News(ctx context.Context, symbols, start, end, keyword string, limit int) ([]NewsArticle, error) {
	if start != "" || end != "" {
		start, end = dateRange(start, end, ClampSeriesWindow)
	}
	q := url.Values{"extension": {"idx"}}
	if symbols != "" {
		q.Set("symbols", symbols)
	}
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprint(limit))
	}
	raw, err := c.Get(ctx, "news/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []NewsArticle `json:"results"`
	}
	if err := decode(raw, &resp, "v2/news/"); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// Filings fetches insider/institution transactions incrementally.
// Windows clamp to 90d like other series endpoints.
func (c *Client) Filings(ctx context.Context, symbol, txnType, holderType, start, end string) ([]Filing, error) {
	if start != "" || end != "" {
		start, end = dateRange(start, end, ClampSeriesWindow)
	}
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", strings.ToUpper(symbol))
	}
	if txnType != "" {
		q.Set("transaction_type", txnType)
	}
	if holderType != "" {
		q.Set("holder_type", holderType)
	}
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	raw, err := c.Get(ctx, "filings/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []Filing `json:"results"`
		Data    []Filing `json:"data"`
	}
	if err := decode(raw, &resp, "v2/filings/"); err != nil {
		return nil, err
	}
	return append(resp.Results, resp.Data...), nil
}

// Suspensions fetches new IDX suspension notices.
// Windows clamp to 90d like other series endpoints.
func (c *Client) Suspensions(ctx context.Context, symbol, start, end string) ([]Suspension, error) {
	if start != "" || end != "" {
		start, end = dateRange(start, end, ClampSeriesWindow)
	}
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", strings.ToUpper(symbol))
	}
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	raw, err := c.Get(ctx, "suspensions/", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []Suspension `json:"results"`
		Data    []Suspension `json:"data"`
	}
	if err := decode(raw, &resp, "v2/suspensions/"); err != nil {
		return nil, err
	}
	return append(resp.Results, resp.Data...), nil
}

// --- Screener / taxonomy ---

// Screen runs the companies/ structured screener (q overrides all).
func (c *Client) Screen(ctx context.Context, where, q string, limit, offset int) ([]ScreenerRow, error) {
	qq := url.Values{}
	if q != "" {
		qq.Set("q", q)
	} else if where != "" {
		qq.Set("where", where)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	qq.Set("limit", fmt.Sprint(limit))
	qq.Set("offset", fmt.Sprint(offset))
	raw, err := c.Get(ctx, "companies/", qq)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []ScreenerRow `json:"results"`
		Data    []ScreenerRow `json:"data"`
	}
	if err := decode(raw, &resp, "v2/companies/"); err != nil {
		return nil, err
	}
	return append(resp.Results, resp.Data...), nil
}

// FreeFloat fetches one exclusive filter per request (API rejects mixed).
func (c *Client) FreeFloat(ctx context.Context, sector, subSector, industry, subIndustry string) ([]FreeFloatRow, error) {
	q := url.Values{}
	set := 0
	for k, v := range map[string]string{"sector": sector, "sub_sector": subSector, "industry": industry, "sub_industry": subIndustry} {
		if v != "" {
			q.Set(k, v)
			set++
		}
	}
	if set != 1 {
		return nil, fmt.Errorf("sectors: free-float needs exactly one filter (got %d)", set)
	}
	raw, err := c.Get(ctx, "free-float/", q)
	if err != nil {
		return nil, err
	}
	var rows []FreeFloatRow
	if err := decode(raw, &rows, "v2/free-float/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// Taxonomy fetches one slug list (subsectors/industries/subindustries/tags).
func (c *Client) Taxonomy(ctx context.Context, kind string) ([]map[string]any, error) {
	raw, err := c.Get(ctx, kind+"/", url.Values{})
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := decode(raw, &rows, "v2/"+kind+"/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// SubsectorReport fetches explicit sections only (1 credit/section).
func (c *Client) SubsectorReport(ctx context.Context, subSector string, sections []string) (map[string]any, error) {
	if len(sections) == 0 {
		sections = []string{"statistics"}
	}
	q := url.Values{"sections": {strings.Join(sections, ",")}}
	raw, err := c.Get(ctx, "subsector/report/"+subSector+"/", q)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := decode(raw, &out, "v2/subsector/report/{sub_sector}/"); err != nil {
		return nil, err
	}
	return out, nil
}

// --- Company depth: segments, quarterly dates, IPO, holders, index ---

// Segments fetches the revenue breakdown (Sankey-ready) for one ticker.
func (c *Client) Segments(ctx context.Context, symbol string, year int) (*Segments, error) {
	q := url.Values{}
	if year >= 1900 && year <= 2026 {
		q.Set("financial_year", fmt.Sprint(year))
	}
	raw, err := c.Get(ctx, "company/get-segments/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var out Segments
	if err := decode(raw, &out, "v2/company/get-segments/{symbol}/"); err != nil {
		return nil, err
	}
	return &out, nil
}

// QuarterlyDates returns valid [report_date, quarter] pairs for one ticker
// (use to resolve report_date before Quarterly calls).
func (c *Client) QuarterlyDates(ctx context.Context, symbol string) ([]QuarterlyDate, error) {
	raw, err := c.Get(ctx, "company/get_quarterly_financial_dates/"+strings.ToUpper(symbol)+"/", url.Values{})
	if err != nil {
		return nil, err
	}
	// Shape: {"YYYY-MM-DD": [["report_date","quarter"], ...], ...} flattened.
	var grouped map[string][][]string
	if err := decode(raw, &grouped, "v2/company/get_quarterly_financial_dates/{symbol}/"); err != nil {
		return nil, err
	}
	var out []QuarterlyDate
	for _, pairs := range grouped {
		for _, p := range pairs {
			if len(p) >= 2 {
				out = append(out, QuarterlyDate{ReportDate: p[0], Quarter: p[1]})
			}
		}
	}
	return out, nil
}

// ListingPerformance fetches IPO context (post-May-2005 listings only).
func (c *Client) ListingPerformance(ctx context.Context, symbol string) (*ListingPerformance, error) {
	raw, err := c.Get(ctx, "listing-performance/"+strings.ToUpper(symbol)+"/", url.Values{})
	if err != nil {
		return nil, err
	}
	var out ListingPerformance
	if err := decode(raw, &out, "v2/listing-performance/{symbol}/"); err != nil {
		return nil, err
	}
	return &out, nil
}

// IndexDaily returns the benchmark series for beta/correlation (window ≤90d).
func (c *Client) IndexDaily(ctx context.Context, indexCode, start, end string) ([]IndexBar, error) {
	start, end = dateRange(start, end, ClampSeriesWindow)
	q := url.Values{"start": {start}, "end": {end}}
	raw, err := c.Get(ctx, "index-daily/"+strings.ToLower(indexCode)+"/", q)
	if err != nil {
		return nil, err
	}
	var rows []IndexBar
	if err := decode(raw, &rows, "v2/index-daily/{index_code}/"); err != nil {
		return nil, err
	}
	return rows, nil
}

// Shareholders fetches the local-vs-foreign holder mix (data from 2021).
func (c *Client) Shareholders(ctx context.Context, symbol string, year int) (*ShareholdersComposition, error) {
	q := url.Values{}
	if year >= 2021 {
		q.Set("year", fmt.Sprint(year))
	}
	raw, err := c.Get(ctx, "company/shareholders-composition/"+strings.ToUpper(symbol)+"/", q)
	if err != nil {
		return nil, err
	}
	var out ShareholdersComposition
	if err := decode(raw, &out, "v2/company/shareholders-composition/{symbol}/"); err != nil {
		return nil, err
	}
	return &out, nil
}

// SegmentAvailability checks which symbols have segment data (cache weekly).
func (c *Client) SegmentAvailability(ctx context.Context) (map[string][]int, error) {
	raw, err := c.Get(ctx, "companies/list_companies_with_segments/", url.Values{})
	if err != nil {
		return nil, err
	}
	var out map[string]struct {
		Years []int `json:"financial_year"`
	}
	if err := decode(raw, &out, "v2/companies/list_companies_with_segments/"); err != nil {
		return nil, err
	}
	res := make(map[string][]int, len(out))
	for k, v := range out {
		res[k] = v.Years
	}
	return res, nil
}
