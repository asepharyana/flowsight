package api

import (
	"net/http"
	"time"

	"flowsight/internal/model"
)

// Health serves GET /api/health: last cycle time + credits spent today +
// scheduler state + stale flags (docs/API.md).
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("force") == "1" {
		_ = s.Sched.RunCycle(r.Context()) // synchronous probe cycle
	}
	lastCycle, schedOK := s.Sched.Status()
	today := time.Now().Format("2006-01-02")
	credits := s.DB.CreditsToday(today)
	lastDate, _, _, _ := s.DB.LatestBriefing()
	cutoff := model.StaleSession(time.Now())
	stale := []string{}
	if lastDate != "" {
		if t, err := time.Parse("2006-01-02", lastDate[:10]); err == nil && t.Before(cutoff) {
			stale = append(stale, "briefing older than one session ("+lastDate+")")
		} else if lastDate < today {
			stale = append(stale, "briefing older than today ("+lastDate+")")
		}
	}
	if !s.Cfg.HasSectorsKey() {
		stale = append(stale, "offline mode: SECTORS_API_KEY unset, serving seed data")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"last_cycle_at": lastCycle,
		"credits_today": credits,
		"scheduler_ok":  schedOK,
		"stale_flags":   stale,
		"citations":     []model.Citation{},
	})
}

// Accuracy serves GET /api/accuracy: per-agent {calls, resolved, hits, hit_rate}.
func (s *Server) Accuracy(w http.ResponseWriter, r *http.Request) {
	stats, err := s.DB.AccuracyStats()
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if stats == nil {
		stats = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": stats})
}
