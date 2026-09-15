// Package sectors wraps the Sectors API v2 with retry, credit counting, and
// parameter-narrowing guards. Every outbound Sectors call in FlowSight goes
// through SectorsClient — no raw HTTP to api.sectors.app elsewhere.
//
// Auth: `Authorization: <raw-key>` from env SECTORS_API_KEY. v1 is gone (410);
// only v2 paths are used.
package sectors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrNoAPIKey is returned when no SECTORS_API_KEY is configured (offline mode).
var ErrNoAPIKey = errors.New("sectors: SECTORS_API_KEY not set (offline/seed mode)")

// ErrV1Gone is returned if a v1 path is ever requested (v1 returns 410).
var ErrV1Gone = errors.New("sectors: v1 API discontinued (410 Gone), use v2 only")

// Spend tracks credit usage per endpoint for one day.
type Spend struct {
	Calls   int
	Credits int
}

// Client is a credit-counting Sectors v2 HTTP client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

	mu      sync.Mutex
	day     string
	spend   map[string]*Spend
	onSpend func(endpoint string, calls, credits int)
}

// New builds a client. baseURL should end with a slash.
func New(baseURL, apiKey string) *Client {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	tr := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Transport: tr, Timeout: 30 * time.Second},
		day:     time.Now().Format("2006-01-02"),
		spend:   map[string]*Spend{},
	}
}

// OnSpend registers a callback fired after every counted call (used by the
// scheduler to persist rows into credit_ledger).
func (c *Client) OnSpend(fn func(endpoint string, calls, credits int)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSpend = fn
}

// SpendToday returns a copy of today's per-endpoint spend.
func (c *Client) SpendToday() map[string]Spend {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]Spend, len(c.spend))
	for k, v := range c.spend {
		out[k] = *v
	}
	return out
}

// CreditsToday returns total credits spent today.
func (c *Client) CreditsToday() int {
	total := 0
	for _, s := range c.SpendToday() {
		total += s.Credits
	}
	return total
}

func (c *Client) count(endpoint string, calls, credits int) {
	c.mu.Lock()
	today := time.Now().Format("2006-01-02")
	if today != c.day {
		c.day, c.spend = today, map[string]*Spend{}
	}
	s := c.spend[endpoint]
	if s == nil {
		s = &Spend{}
		c.spend[endpoint] = s
	}
	s.Calls += calls
	s.Credits += credits
	fn := c.onSpend
	c.mu.Unlock()
	if fn != nil {
		fn(endpoint, calls, credits)
	}
}

// costFor implements the credit-cost table from docs/API-REFERENCE.md.
func costFor(path string, q url.Values) int {
	p := strings.Trim(path, "/")
	switch {
	case p == "v2/companies/top-changes" || p == "companies/top-changes":
		classes := splitCSV(q.Get("classifications"))
		periods := splitCSV(q.Get("periods"))
		if len(classes) == 0 {
			classes = []string{"a", "b"} // default is expensive: 2 classes
		}
		if len(periods) == 0 {
			periods = []string{"a", "b", "c", "d", "e"} // default: 5 periods
		}
		return len(classes) * len(periods)
	case strings.HasPrefix(p, "v2/company/report") || strings.HasPrefix(p, "company/report"):
		if s := splitCSV(q.Get("sections")); len(s) > 0 {
			return len(s)
		}
		return 8 // default all sections
	case strings.HasPrefix(p, "v2/subsector/report") || strings.HasPrefix(p, "subsector/report"):
		if s := splitCSV(q.Get("sections")); len(s) > 0 {
			return len(s)
		}
		return 6
	case strings.HasPrefix(p, "v2/financials/quarterly") || strings.HasPrefix(p, "financials/quarterly"):
		if n := atoi(q.Get("n_quarters"), 1); n > 0 {
			return n
		}
		return 1
	case p == "v2/most-traded" || p == "most-traded",
		p == "v2/brokers/top" || p == "brokers/top",
		strings.HasSuffix(p, "/top") && strings.Contains(p, "broker-activity"),
		strings.HasSuffix(p, "/top") && strings.Contains(p, "broker-summary"):
		return 2
	default:
		return 1
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoi(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

// maxBrokerWindowDays and maxSeriesDays encode the API window limits.
const (
	maxBrokerWindowDays = 14
	maxSeriesDays       = 90
)

// ClampBrokerWindow clamps a date range to the 14-day broker-endpoint limit.
func ClampBrokerWindow(start, end time.Time) (time.Time, time.Time) {
	if end.Sub(start).Hours()/24 > maxBrokerWindowDays {
		start = end.AddDate(0, 0, -maxBrokerWindowDays)
	}
	return start, end
}

// ClampSeriesWindow clamps a date range to the 90-day series limit.
func ClampSeriesWindow(start, end time.Time) (time.Time, time.Time) {
	if end.Sub(start).Hours()/24 > maxSeriesDays {
		start = end.AddDate(0, 0, -maxSeriesDays)
	}
	return start, end
}

func isoDate(t time.Time) string { return t.Format("2006-01-02") }

// Get performs a GET against a v2 path, enforcing auth, retry, and credit
// counting. Callers pass paths like "v2/brokers/top/" or "brokers/top/".
func (c *Client) Get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, ErrNoAPIKey
	}
	if strings.Contains(path, "/v1/") || strings.HasPrefix(path, "v1/") {
		return nil, ErrV1Gone
	}
	if e := q.Get("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil && t.After(time.Now().Add(24*time.Hour)) {
			return nil, fmt.Errorf("sectors: future end date %s rejected (API returns 400)", e)
		}
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(path, "/"), "v2/")
	endpoint := "v2/" + strings.Trim(rel, "/") + "/"
	credits := costFor(path, q)

	full := c.baseURL + rel
	if len(q) > 0 {
		full += "?" + q.Encode()
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * 500 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.apiKey)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			c.count(endpoint, 1, credits)
			return body, nil
		case resp.StatusCode == http.StatusGone:
			return nil, ErrV1Gone
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("sectors: %s -> HTTP %d (retrying)", endpoint, resp.StatusCode)
			continue
		default:
			c.count(endpoint, 1, credits)
			return nil, fmt.Errorf("sectors: %s -> HTTP %d: %s", endpoint, resp.StatusCode, truncate(string(body), 300))
		}
	}
	c.count(endpoint, 1, credits)
	return nil, fmt.Errorf("sectors: %s failed after retries: %w", endpoint, lastErr)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// EstimateCycleCredits returns the expected credit spend of one 30-min
// ingestion cycle for W watchlist tickers (docs/API-REFERENCE.md §budget).
func EstimateCycleCredits(w int) int {
	return 10 + 2 + 2 + 1 + 2 + 3*w + 3 + 2 // ~= 22 + 3W
}

// MorningBriefingExtra returns (min, max) extra credits for the briefing run.
func MorningBriefingExtra() (int, int) { return 35, 40 }
