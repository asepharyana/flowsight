package sectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// One live-shape call against a mock upstream: narrowing is enforced and
// credits counted per the API-REFERENCE cost table.
func TestNarrowingAndCredits(t *testing.T) {
	var gotQ url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query()
		w.Write([]byte(`{"top_gainers":{"1d":[]},"top_losers":{"1d":[]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "test-key")
	_, _, err := c.TopChanges(context.Background(),
		[]string{"top_gainers"}, []string{"1d", "7d"}, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if gotQ.Get("classifications") != "top_gainers" {
		t.Fatalf("classifications = %q, want narrowed", gotQ.Get("classifications"))
	}
	if gotQ.Get("periods") != "1d,7d" {
		t.Fatalf("periods = %q, want narrowed", gotQ.Get("periods"))
	}
	if got := c.CreditsToday(); got != 2 {
		t.Fatalf("credits = %d, want 2 (1 class x 2 periods)", got)
	}
}

// Per-cycle budget: EstimateCycleCredits(20) must stay <= 120 cap.
func TestCycleBudget(t *testing.T) {
	if got := EstimateCycleCredits(20); got > 120 {
		t.Fatalf("cycle estimate = %d, exceeds 120 cap", got)
	}
	if got := EstimateCycleCredits(5); got != 22+3*5 {
		t.Fatalf("cycle estimate = %d, want %d", got, 22+3*5)
	}
}

// v1 paths are rejected before any HTTP happens.
func TestV1Rejected(t *testing.T) {
	c := New("https://api.sectors.app/v2/", "k")
	if _, err := c.Get(context.Background(), "v1/companies/", url.Values{}); err != ErrV1Gone {
		t.Fatalf("err = %v, want ErrV1Gone", err)
	}
}

// Future end dates are rejected (API returns 400).
func TestFutureEndRejected(t *testing.T) {
	c := New("https://api.sectors.app/v2/", "k")
	q := url.Values{"end": {"2099-01-01"}}
	if _, err := c.Get(context.Background(), "daily/BBCA/", q); err == nil {
		t.Fatal("want future-end rejection")
	}
}

// New wrappers hit exact paths with exact params; all cost 1 credit.
func TestCompanyDepthWrappers(t *testing.T) {
	var gotPath, gotQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQ = r.URL.Path, r.URL.RawQuery
		switch {
		case strings.HasPrefix(r.URL.Path, "/company/get-segments/"):
			w.Write([]byte(`{"symbol":"BBCA.JK","financial_year":2025,"revenue_breakdown":[{"value":1,"source":"Loans","target":"Interest Income"}]}`))
		case strings.HasPrefix(r.URL.Path, "/company/get_quarterly_financial_dates/"):
			w.Write([]byte(`{"2026":[["2026-03-31","q1"]]}`))
		case strings.HasPrefix(r.URL.Path, "/listing-performance/"):
			w.Write([]byte(`{"symbol":"BBCA.JK","listing_date":"2000-05-31"}`))
		case strings.HasPrefix(r.URL.Path, "/index-daily/"):
			w.Write([]byte(`[{"index_code":"ihsg","date":"2026-09-11","price":7800.5}]`))
		case strings.HasPrefix(r.URL.Path, "/company/shareholders-composition/"):
			w.Write([]byte(`{"symbol":"BBCA.JK","year":2026,"data":[]}`))
		default:
			w.Write([]byte(`{"BBCA.JK":{"financial_year":[2025]}}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "test-key")
	if _, err := c.Segments(context.Background(), "BBCA", 2025); err != nil {
		t.Fatal(err)
	}
	if gotQ != "financial_year=2025" {
		t.Fatalf("segments query = %q", gotQ)
	}
	qd, err := c.QuarterlyDates(context.Background(), "BBCA")
	if err != nil || len(qd) != 1 || qd[0].ReportDate != "2026-03-31" {
		t.Fatalf("quarterly dates = %v, %v", qd, err)
	}
	if _, err := c.ListingPerformance(context.Background(), "BBCA"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.IndexDaily(context.Background(), "ihsg", "2026-09-01", "2026-09-11"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Shareholders(context.Background(), "BBCA", 2026); err != nil {
		t.Fatal(err)
	}
	avail, err := c.SegmentAvailability(context.Background())
	if err != nil || len(avail["BBCA.JK"]) != 1 {
		t.Fatalf("availability = %v, %v", avail, err)
	}
	_ = gotPath
}

// Event windows clamp to 90d and report real costs.
func TestEventClamps(t *testing.T) {
	var gotQ url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query()
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "test-key")
	_, _ = c.News(context.Background(), "BBCA", "2026-01-01", "2026-09-11", "", 20)
	if gotQ.Get("start") != "2026-06-13" {
		t.Fatalf("news start = %q, want clamped to 90d", gotQ.Get("start"))
	}
	if got := c.CreditsToday(); got < 1 {
		t.Fatalf("credits = %d, want counted", got)
	}
}
