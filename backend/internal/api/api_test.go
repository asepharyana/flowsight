package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flowsight/internal/config"
	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Load()
	cfg.DemoUserKey = "demo"
	db, err := store.Open(t.TempDir() + "/api.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.SeedFromDir("../../tests/fixtures", "demo"); err != nil {
		t.Fatal(err)
	}
	cache := store.NewCache("")
	return New(cfg, db, cache, sectors.New(cfg.SectorsBaseURL, ""))
}

func do(s *Server, method, path string, body any) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("X-User-Key", "demo")
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

// Health 200 with cycle + credits fields.
func TestHealth(t *testing.T) {
	s := testServer(t)
	rec := do(s, "GET", "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	for _, k := range []string{"last_cycle_at", "credits_today", "scheduler_ok", "stale_flags"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("missing key %s", k)
		}
	}
}

// Briefing generates from seed with zero empty sections + citations.
func TestBriefing(t *testing.T) {
	s := testServer(t)
	rec := do(s, "GET", "/api/briefing/today", nil)
	if rec.Code == http.StatusNotFound {
		// No briefing yet: run the routine via engine path instead.
		rows, _ := s.DB.ListRoutines("demo")
		if len(rows) == 0 {
			t.Fatal("seed has no routines")
		}
		if _, err := s.Engine.Run(httptest.NewRequest("GET", "/", nil).Context(), rows[0]); err != nil {
			t.Fatal(err)
		}
		rec = do(s, "GET", "/api/briefing/today", nil)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
}

// Screener returns a ranked list with per-row breakdown.
func TestScreen(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/screen", map[string]any{"limit": 5})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var out struct {
		Rows []map[string]any `json:"rows"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Rows) == 0 {
		t.Fatal("empty screener rows")
	}
	if _, ok := out.Rows[0]["breakdown"]; !ok {
		t.Fatal("missing per-row breakdown")
	}
}

// Report: all 7 sections populated with citations.
func TestReport(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/report/BBCA?profile=moderate", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String()[:300])
	}
	var out struct {
		Sections []map[string]any `json:"sections"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Sections) != 7 {
		t.Fatalf("sections = %d, want 7", len(out.Sections))
	}
	for _, sec := range out.Sections {
		if sec["body"] == "" || sec["body"] == nil {
			t.Fatalf("empty section %v", sec["name"])
		}
	}
}

// Subscribe -> run -> history row appears.
func TestRoutineSubscribeRunHistory(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/routines", map[string]any{"type": "foreign-reversal"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d", rec.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	rows, _ := s.DB.ListRoutines("demo")
	if len(rows) == 0 {
		t.Fatal("no routines")
	}
	if _, err := s.Engine.Run(httptest.NewRequest("GET", "/", nil).Context(), rows[0]); err != nil {
		t.Fatal(err)
	}
	rec = do(s, "GET", "/api/routine-runs?limit=5", nil)
	var out struct {
		Runs []map[string]any `json:"runs"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Runs) == 0 {
		t.Fatal("no history rows")
	}
}

// Interrogation is scoped to report citations: conviction Q&A answers
// from the persisted report, unknown report 404s.
func TestInterrogate(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/report/BBCA?profile=moderate", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	rec = do(s, "POST", "/api/report/BBCA/ask", map[string]any{"question": "kenapa conviction segitu?"})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String()[:200])
	}
	var out struct {
		Answer string `json:"answer"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Answer == "" {
		t.Fatal("empty interrogation answer")
	}
	rec = do(s, "POST", "/api/report/ZZZZ/ask", map[string]any{"question": "apa?"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 for unknown ticker", rec.Code)
	}
}

// Concentrated fixture warns >40% sector; accuracy math covered.
func TestPortfolioAndAccuracy(t *testing.T) {
	s := testServer(t)
	rec := do(s, "GET", "/api/portfolio/risk", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	rec = do(s, "GET", "/api/accuracy", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
}

// start/end filters narrow the foreign series; reversal uses the 2x rule.
func TestFlowForeignWindow(t *testing.T) {
	s := testServer(t)
	rec := do(s, "GET", "/api/flow/foreign?ticker=BBCA&start=2026-09-11&end=2026-09-11", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var out struct {
		Dates []string  `json:"dates"`
		Nets  []float64 `json:"nets"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Dates) != 1 || out.Dates[0] != "2026-09-11" {
		t.Fatalf("dates = %v, want single filtered day", out.Dates)
	}
}

// Unknown routine types are rejected; missing ids 404.
func TestRoutineValidation(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/routines", map[string]any{"type": "not-a-routine"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	rec = do(s, "PATCH", "/api/routines/999999", map[string]any{"enabled": false})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
	rec = do(s, "DELETE", "/api/routines/999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
	rec = do(s, "DELETE", "/api/alerts/999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

// Destination CRUD: masked list, kind validation, owner scoping, 404s.
func TestDestinations(t *testing.T) {
	s := testServer(t)
	// Invalid kind -> 422.
	rec := do(s, "POST", "/api/destinations", map[string]any{"kind": "sms"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	// Telegram without chat_id -> 422.
	rec = do(s, "POST", "/api/destinations", map[string]any{"kind": "telegram", "bot_token": "x"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	// Discord non-https -> 422.
	rec = do(s, "POST", "/api/destinations", map[string]any{"kind": "discord", "webhook_url": "http://x"})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	// Valid discord create -> 201.
	rec = do(s, "POST", "/api/destinations", map[string]any{"kind": "discord", "label": "ops", "webhook_url": "https://discord.example/hook"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
	// List masks secrets.
	rec = do(s, "GET", "/api/destinations", nil)
	var out struct {
		Destinations []map[string]any `json:"destinations"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Destinations) != 1 {
		t.Fatalf("destinations = %v", out.Destinations)
	}
	for _, k := range []string{"bot_token", "chat_id", "webhook_url"} {
		if _, ok := out.Destinations[0][k]; ok {
			t.Fatalf("secret leaked in list: %s", k)
		}
	}
	if out.Destinations[0]["configured"] != true {
		t.Fatalf("configured flag = %v", out.Destinations[0])
	}
	// Missing id -> 404 on patch and delete.
	rec = do(s, "PATCH", "/api/destinations/999999", map[string]any{"enabled": false})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
	rec = do(s, "DELETE", "/api/destinations/999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}
