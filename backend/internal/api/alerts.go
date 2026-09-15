package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListAlerts serves GET /api/alerts.
func (s *Server) ListAlerts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListAlerts(s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": rows})
}

// CreateAlert serves POST /api/alerts {name, rule, channels[]}.
func (s *Server) CreateAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name" validate:"required"`
		Rule     any      `json:"rule" validate:"required"`
		Channels []string `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "name and rule are required")
		return
	}
	ruleRaw, _ := json.Marshal(req.Rule)
	id, err := s.DB.CreateAlert(s.userKey(r), req.Name, string(ruleRaw), req.Channels)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// DeleteAlert serves DELETE /api/alerts/:id.
func (s *Server) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ok, err := s.DB.DeleteAlert(id, s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "alert not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// AlertEvents serves GET /api/alert-events?since=&ticker=.
func (s *Server) AlertEvents(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = "2000-01-01"
	}
	evts, err := s.DB.AlertEventsSince(since, r.URL.Query().Get("ticker"), 50, s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": evts})
}
