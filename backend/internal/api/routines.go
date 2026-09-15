package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"flowsight/internal/routines"
	"flowsight/internal/store"
)

// ListRoutines serves GET /api/routines with last-run status.
func (s *Server) ListRoutines(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListRoutines(s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	type rowOut struct {
		store.Routine
		LastRun any `json:"last_run"`
	}
	out := make([]rowOut, 0, len(rows))
	for _, row := range rows {
		hist, _ := s.DB.RunHistory(row.ID, 1)
		var last any
		if len(hist) > 0 {
			last = hist[0]
		}
		out = append(out, rowOut{row, last})
	}
	writeJSON(w, http.StatusOK, map[string]any{"routines": out})
}

// CreateRoutine serves POST /api/routines {type, schedule_cron?, channels[]}.
func (s *Server) CreateRoutine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string   `json:"type" validate:"required"`
		Schedule string   `json:"schedule_cron"`
		Channels []string `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "type is required")
		return
	}
	if !routines.KnownType(req.Type) {
		writeErr(w, http.StatusUnprocessableEntity, "unknown routine type")
		return
	}
	if req.Schedule == "" {
		req.Schedule = routines.DefaultSchedule(req.Type)
	}
	id, err := s.DB.CreateRoutine(s.userKey(r), req.Type, req.Schedule, req.Channels)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "type": req.Type, "schedule_cron": req.Schedule})
}

// UpdateRoutine serves PATCH /api/routines/:id.
func (s *Server) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Enabled  *bool    `json:"enabled"`
		Schedule string   `json:"schedule_cron"`
		Channels []string `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ok, err := s.DB.UpdateRoutine(id, s.userKey(r), req.Enabled, req.Schedule, req.Channels)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "routine not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// RunHistory serves GET /api/routine-runs?routine_id=&limit=.
func (s *Server) RunHistory(w http.ResponseWriter, r *http.Request) {
	rid, _ := strconv.ParseInt(r.URL.Query().Get("routine_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	hist, err := s.DB.RunHistory(rid, limit, s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": hist})
}

// BriefingToday serves GET /api/briefing/today: latest payload + citations.
func (s *Server) BriefingToday(w http.ResponseWriter, r *http.Request) {
	date, payload, cites, err := s.DB.LatestBriefing()
	if err != nil {
		writeErr(w, http.StatusNotFound, "no briefing yet — run the morning-briefing routine")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date": date, "payload": payload, "citations": cites,
	})
}

// DeleteRoutine serves DELETE /api/routines/:id.
func (s *Server) DeleteRoutine(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ok, err := s.DB.DeleteRoutine(id, s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "routine not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}
