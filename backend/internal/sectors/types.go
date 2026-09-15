// Package sectors response types mirror docs.sectors.app/schema.json shapes.
package sectors

// BrokerTopRow is one top buyer/seller entry (net/buy/sell in IDR).
type BrokerTopRow struct {
	Rank       int    `json:"rank"`
	BrokerCode string `json:"broker_code"`
	NetIDR     int64  `json:"net_idr"`
	BuyIDR     int64  `json:"buy_idr"`
	SellIDR    int64  `json:"sell_idr"`
}

// BrokerSummaryTop is GET broker-summary/{symbol}/top/ response.
type BrokerSummaryTop struct {
	Symbol     string         `json:"symbol"`
	Start      string         `json:"start"`
	End        string         `json:"end"`
	Origin     string         `json:"origin"`
	Cohort     string         `json:"cohort"`
	TopBuyers  []BrokerTopRow `json:"top_buyers"`
	TopSellers []BrokerTopRow `json:"top_sellers"`
}

// BrokerRow is one per-broker daily row (blot/bval buy, slot/sval sell, nlot/nval net).
type BrokerRow struct {
	BrokerCode string  `json:"broker_code"`
	Symbol     string  `json:"symbol,omitempty"`
	BFreq      int     `json:"bfreq"`
	BLot       int64   `json:"blot"`
	BVal       int64   `json:"bval"`
	BAvg       float64 `json:"bavg_per_share"`
	SFreq      int     `json:"sfreq"`
	SLot       int64   `json:"slot"`
	SVal       int64   `json:"sval"`
	SAvg       float64 `json:"savg_per_share"`
	NLot       int64   `json:"nlot"`
	NVal       int64   `json:"nval"`
	NAvg       float64 `json:"navg_per_share"`
}

// BrokerRegistryRow classifies one broker code.
type BrokerRegistryRow struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	IsForeign   bool    `json:"is_foreign"`
	Cohort      *string `json:"cohort"`
	LicenseType *string `json:"license_type"`
}

// TopBrokerRow is one brokers/top/ ranking row.
type TopBrokerRow struct {
	Rank       int    `json:"rank"`
	BrokerCode string `json:"broker_code"`
	Gross      int64  `json:"gross"`
	Net        int64  `json:"net"`
}

// ForeignPoint is one day of net foreign inflow (IDR, signed).
type ForeignPoint struct {
	Date             string `json:"date"`
	NetForeignInflow int64  `json:"net_foreign_inflow"`
}

// ForeignFlow is GET foreign-flow/{symbol}/ response.
type ForeignFlow struct {
	Symbol string         `json:"symbol"`
	Start  string         `json:"start"`
	End    string         `json:"end"`
	Data   []ForeignPoint `json:"data"`
}

// DailyBar is one daily OHLCV row.
type DailyBar struct {
	Symbol    string `json:"symbol"`
	Date      string `json:"date"`
	Close     int64  `json:"close"`
	Open      int64  `json:"open"`
	High      int64  `json:"high"`
	Low       int64  `json:"low"`
	Volume    int64  `json:"volume"`
	MarketCap int64  `json:"market_cap"`
}

// MoverRow is one top-changes entry; PriceChange is decimal (0.05 = +5%).
type MoverRow struct {
	Name            string  `json:"name"`
	Symbol          string  `json:"symbol"`
	PriceChange     float64 `json:"price_change"`
	LastClosePrice  int64   `json:"last_close_price"`
	LatestCloseDate string  `json:"latest_close_date"`
}

// CloseRow is one close/ universe row.
type CloseRow struct {
	Symbol string `json:"symbol"`
	Date   string `json:"date"`
	Close  int64  `json:"close"`
}

// NewsArticle is one normalized news item.
type NewsArticle struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Source    string   `json:"source"`
	Timestamp string   `json:"timestamp"`
	Sector    string   `json:"sector"`
	SubSector []string `json:"sub_sector"`
	Tags      []string `json:"tags"`
	Symbols   []string `json:"symbols"`
}

// Filing is one insider/institution transaction.
type Filing struct {
	Title             string  `json:"title"`
	Symbol            string  `json:"symbol"`
	Timestamp         string  `json:"timestamp"`
	TransactionType   string  `json:"transaction_type"`
	HolderType        string  `json:"holder_type"`
	HolderName        string  `json:"holder_name"`
	AmountTransaction int64   `json:"amount_transaction"`
	Price             float64 `json:"price"`
	TransactionValue  float64 `json:"transaction_value"`
	Source            string  `json:"source"`
}

// Suspension is one IDX suspension notice.
type Suspension struct {
	Symbol         string `json:"symbol"`
	SuspensionDate string `json:"suspension_date"`
	Reason         string `json:"reason"`
	PDFURL         string `json:"pdf_url"`
}

// ScreenerRow is one companies/ result row.
type ScreenerRow struct {
	Symbol      string         `json:"symbol"`
	CompanyName string         `json:"company_name"`
	QueryValues map[string]any `json:"query_values"`
}

// QuarterRow is one financials/quarterly row.
type QuarterRow struct {
	Symbol   string   `json:"symbol"`
	Date     string   `json:"date"`
	Revenue  *float64 `json:"revenue"`
	Earnings *float64 `json:"earnings"`
	Assets   *float64 `json:"total_assets"`
	Equity   *float64 `json:"total_equity"`
	OpCash   *float64 `json:"operating_cash_flow"`
	Debt     *float64 `json:"total_debt"`
}

// DividendEvent is one dividend corporate-action event (raw map: schema untyped).
type DividendEvent map[string]any

// CorporateActions groups events by type.
type CorporateActions struct {
	Symbol           string          `json:"symbol"`
	Dividend         []DividendEvent `json:"dividend"`
	UpcomingDividend []DividendEvent `json:"upcoming_dividend"`
	AGM              []DividendEvent `json:"agm"`
	StockSplit       []DividendEvent `json:"stock_split"`
}

// QuarterlyDateRow is one companies/quarterly-financial-dates entry.
type QuarterlyDateRow struct {
	Symbol string `json:"symbol"`
	Date   string `json:"date"`
	Year   int    `json:"year"`
}

// FreeFloatRow is one free-float/ entry (decimal, 0.45 = 45%).
type FreeFloatRow struct {
	Symbol      string  `json:"symbol"`
	CompanyName string  `json:"company_name"`
	FreeFloat   float64 `json:"free_float"`
}

// IdxTotalPoint is one idx-total/ row (raw map: shape varies by date range).
type IdxTotalPoint map[string]any

// SegmentRow is one revenue-breakdown entry (Sankey-ready source/target).
type SegmentRow struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
	Target string  `json:"target"`
}

// Segments is GET company/get-segments/{symbol}/ response.
type Segments struct {
	Symbol           string       `json:"symbol"`
	FinancialYear    int          `json:"financial_year"`
	RevenueBreakdown []SegmentRow `json:"revenue_breakdown"`
}

// QuarterlyDate is one [report_date, quarter_label] pair from
// get_quarterly_financial_dates/{symbol}/.
type QuarterlyDate struct {
	ReportDate string `json:"report_date"`
	Quarter    string `json:"quarter"`
}

// ListingPerformance is GET listing-performance/{symbol}/ (IPO context).
type ListingPerformance struct {
	Symbol        string   `json:"symbol"`
	CompanyName   string   `json:"company_name"`
	ListingDate   string   `json:"listing_date"`
	Chg7d         *float64 `json:"chg_7d"`
	Chg30d        *float64 `json:"chg_30d"`
	Chg90d        *float64 `json:"chg_90d"`
	Chg365d       *float64 `json:"chg_365d"`
	OfferingPrice *int64   `json:"offering_price"`
}

// IndexBar is one index-daily/{index_code}/ row.
type IndexBar struct {
	IndexCode string  `json:"index_code"`
	Date      string  `json:"date"`
	Price     float64 `json:"price"`
}

// ShareholderRow is one shareholders-composition/{symbol}/ month row
// (local _l vs foreign _f holder mix across 9 categories).
type ShareholderRow struct {
	Date        string `json:"date"`
	TotalL      *int64 `json:"total_l"`
	TotalF      *int64 `json:"total_f"`
	IndividualL *int64 `json:"individual_l"`
	IndividualF *int64 `json:"individual_f"`
	CorporateL  *int64 `json:"corporate_l"`
	CorporateF  *int64 `json:"corporate_f"`
}

// ShareholdersComposition is GET company/shareholders-composition/{symbol}/.
type ShareholdersComposition struct {
	Symbol string           `json:"symbol"`
	Year   int              `json:"year"`
	Data   []ShareholderRow `json:"data"`
}
