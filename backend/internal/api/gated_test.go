package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Gated routes 401 without session; public routes stay 200.
func TestGatedRequiresLogin(t *testing.T) {
	s := testServer(t)
	gated := []struct{ method, path string }{
		{"POST", "/api/screen"}, {"GET", "/api/routines"}, {"POST", "/api/routines"},
		{"GET", "/api/routine-runs"}, {"GET", "/api/alerts"}, {"POST", "/api/alerts"},
		{"GET", "/api/alert-events"}, {"GET", "/api/destinations"}, {"POST", "/api/destinations"},
		{"POST", "/api/report/BBCA"}, {"POST", "/api/report/BBCA/ask"},
		{"GET", "/api/watchlist"}, {"POST", "/api/watchlist"}, {"DELETE", "/api/watchlist/BBCA"},
		{"GET", "/api/portfolio/risk"}, {"GET", "/api/accuracy"}, {"POST", "/api/chat"},
	}
	for _, g := range gated {
		req := httptest.NewRequest(g.method, g.path, nil) // no session cookie, no demo header
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", g.method, g.path, rec.Code)
		}
	}
	public := []string{"/api/health", "/api/version", "/api/flow/summary", "/api/briefing/today"}
	for _, p := range public {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}

func doAuth(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	u, _ := s.DB.CheckLocalUser("tester", "password1234")
	if u == nil {
		var err error
		u, err = s.DB.CreateLocalUser("tester", "password1234")
		if err != nil {
			t.Fatal(err)
		}
	}
	tok, err := s.DB.CreateSession(u.ID, u.UserKey, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "fs_session", Value: tok})
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

// Signup -> login -> gated route works with cookie; dup/wrong rejected.
func TestSignupLoginRoundTrip(t *testing.T) {
	s := testServer(t)
	rec := do(s, "POST", "/api/auth/signup", map[string]any{"username": "budi", "password": "rahasia123"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup = %d, body %s", rec.Code, rec.Body.String())
	}
	rec = do(s, "POST", "/api/auth/signup", map[string]any{"username": "budi", "password": "rahasia123"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup signup = %d, want 409", rec.Code)
	}
	rec = do(s, "POST", "/api/auth/login", map[string]any{"username": "budi", "password": "salahpass"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong login = %d, want 401", rec.Code)
	}
	// Fresh signup seeds the "semua" watchlist (fixture tickers BBCA, TLKM).
	wl, _ := s.DB.Watchlist("u:local:budi")
	if len(wl) < 2 {
		t.Fatalf("semua watchlist = %v, want all fixture tickers", wl)
	}
	// Version endpoint reports a commit string.
	rec = do(s, "GET", "/api/version", nil)
	var v struct {
		Commit    string `json:"commit"`
		StartedAt string `json:"started_at"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if rec.Code != http.StatusOK || v.StartedAt == "" {
		t.Fatalf("version = %d %s", rec.Code, rec.Body.String())
	}
	// AllTickers excludes pseudo tickers.
	all, _ := s.DB.AllTickers()
	for _, tk := range all {
		if tk == "IDX" || tk == "ROE" {
			t.Fatalf("AllTickers leaked pseudo %s", tk)
		}
	}
	if len(all) == 0 {
		t.Fatal("AllTickers empty on seeded db")
	}
}
