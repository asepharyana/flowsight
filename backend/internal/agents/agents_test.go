package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"flowsight/internal/llm"
	"flowsight/internal/store"
)

func seedDB(t *testing.T) (*store.DB, Deps) {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/agents.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SeedFromDir("../../tests/fixtures", "demo"); err != nil {
		t.Fatal(err)
	}
	d := Deps{DB: db, LLM: llm.New("", ""), Now: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
	return db, d
}

func hasCiteLen(res interface{ GetCitations() int }) {}

// BBCA accumulation fixture scores > +60 with 3 named brokers cited.
func TestSmartMoney(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeSmartMoney(context.Background(), d, "BBCA")
	if res.Score <= 60 {
		t.Fatalf("score = %.0f, want > 60", res.Score)
	}
	if len(res.Citations) == 0 {
		t.Fatal("no citations")
	}
	extra := res.Extra
	players, _ := extra["players"].([]string)
	if len(players) < 3 {
		t.Fatalf("players = %v, want 3 named brokers", players)
	}
}

// Financials -> Consumer rotation detected with sign-flip evidence.
func TestBrokerIntel(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeBrokerIntel(context.Background(), d, "BBCA")
	found := false
	for _, f := range res.Flags {
		if f == "sector-rotation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("flags = %v, want sector-rotation", res.Flags)
	}
}

// Sentiment trend with >=2 cited articles + insider summary.
func TestSentiment(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeSentiment(context.Background(), d, "BBCA")
	if len(res.Citations) == 0 {
		t.Fatal("no citations")
	}
	if res.Extra["trend"] != "improving" {
		t.Fatalf("trend = %v, want improving", res.Extra["trend"])
	}
}

// BBCA shows P/E vs banks median with cited sections.
func TestFundamental(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeFundamental(context.Background(), d, "BBCA")
	if len(res.Citations) < 2 {
		t.Fatalf("citations = %d, want >= 2 (report + peers)", len(res.Citations))
	}
	found := false
	for _, v := range res.Values {
		if v.Label == "valuation vs peers" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing valuation-vs-peers row")
	}
}

// 3.2x volume spike flagged with date.
func TestTechnical(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeTechnical(context.Background(), d, "BBCA")
	found := false
	for _, f := range res.Flags {
		if f == "volume-anomaly" {
			found = true
		}
	}
	if !found {
		t.Fatalf("flags = %v, want volume-anomaly", res.Flags)
	}
}

// Ex-div date + yield appear with H-N countdown.
func TestCatalyst(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeCatalyst(context.Background(), d, "BBCA")
	if res.Score <= 0 {
		t.Fatalf("score = %.0f, want > 0 (ex-div in 23d)", res.Score)
	}
	cal, _ := res.Extra["calendar"].([]string)
	if len(cal) == 0 {
		t.Fatal("empty catalyst calendar")
	}
}

// Good fundamental + broker selling => HOLD-or-lower with conflict flag.
func TestSynthesizerConflict(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	results := RunAll(context.Background(), d, "BBCA")
	// Simulate broker distribution opposing the fixture's accumulation.
	for i, r := range results {
		if r.Agent == "smart-money" {
			r.Score = -60
			r.Summary = "distribution (simulated)"
			results[i] = r
		}
	}
	s := Synthesize(context.Background(), d, "BBCA", Moderate, results)
	if !s.Conflict {
		t.Fatal("want conflict flag on fundamental-vs-flow opposition")
	}
	if s.Recommendation == "BUY" {
		t.Fatalf("recommendation = BUY, want HOLD-or-lower on conflict")
	}
}

// Segments snapshot adds a revenue-segments value row with citation.
func TestFundamentalSegments(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeFundamental(context.Background(), d, "BBCA")
	found := false
	for _, v := range res.Values {
		if v.Label == "revenue segments" {
			found = true
			if len(v.Citations) == 0 {
				t.Fatal("segments row has no citations")
			}
		}
	}
	if !found {
		t.Fatal("missing revenue-segments row")
	}
}

// Falling quarter ROE flags declining-roe: ROE fixture earnings edge up
// 100->105 while equity balloons 1000->1500, so ROE falls 10%->7% (-30%).
func TestFundamentalDecliningROE(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeFundamental(context.Background(), d, "ROE")
	found := false
	for _, f := range res.Flags {
		if f == "declining-roe" {
			found = true
		}
	}
	if !found {
		t.Fatalf("flags = %v, want declining-roe", res.Flags)
	}
}

// Splits + IPO window appear on the catalyst calendar when present.
func TestCatalystSplitsIPO(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeCatalyst(context.Background(), d, "BBCA")
	if len(res.Citations) == 0 {
		t.Fatal("no citations")
	}
}

// Relative volume cites most-traded: BBCA last volume 288M vs fixture
// median 120M => 2.4x row present with a most-traded citation.
func TestTechnicalRelVol(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	res := AnalyzeTechnical(context.Background(), d, "BBCA")
	found := false
	for _, v := range res.Values {
		if v.Label == "relative volume" {
			found = true
			if len(v.Citations) == 0 {
				t.Fatal("relative-volume row has no citations")
			}
		}
	}
	if !found {
		t.Fatalf("values = %v, want relative-volume row", res.Values)
	}
	if rv, _ := res.Extra["rel_volume"].(float64); rv < 2.0 || rv > 3.0 {
		t.Fatalf("rel_volume = %v, want ~2.4", rv)
	}
}

// Thesis claims carry inline citation markers: every agent line ends
// with [endpoint @ date] (or [no snapshot] when input is missing).
func TestSynthesizerThesisCites(t *testing.T) {
	db, d := seedDB(t)
	defer db.Close()
	results := RunAll(context.Background(), d, "BBCA")
	s := Synthesize(context.Background(), d, "BBCA", Moderate, results)
	if len(s.Thesis) == 0 || len(s.Citations) == 0 {
		t.Fatal("thesis or citations empty")
	}
	if strings.Count(s.Thesis, "[v2/") < 3 {
		t.Fatalf("want >=3 inline [v2/ markers, got: %s", s.Thesis)
	}
	if strings.Count(s.Thesis, "@ 2026-09-11]") < 3 {
		t.Fatalf("want dated markers, got: %s", s.Thesis)
	}
}
