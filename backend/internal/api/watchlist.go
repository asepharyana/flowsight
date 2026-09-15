package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// GetWatchlist serves GET /api/watchlist.
func (s *Server) GetWatchlist(w http.ResponseWriter, r *http.Request) {
	wl, err := s.DB.Watchlist(s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"watchlist": wl})
}

// AddWatch serves POST /api/watchlist {ticker}.
func (s *Server) AddWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ticker string `json:"ticker" validate:"required,len=4"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Ticker = strings.ToUpper(strings.TrimSpace(req.Ticker))
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "ticker (4 letters) required")
		return
	}
	if err := s.DB.AddWatch(s.userKey(r), req.Ticker); err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ticker": req.Ticker})
}

// RemoveWatch serves DELETE /api/watchlist/:ticker.
func (s *Server) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	if err := s.DB.RemoveWatch(s.userKey(r), ticker); err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticker": ticker, "ok": true})
}
