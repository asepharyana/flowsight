// Package api serves the FlowSight REST API (docs/API.md) on chi: flow
// summary/broker/foreign, screener, routines, briefing, alerts, reports,
// watchlist, portfolio risk, accuracy, chat (report-scoped), health, and the
// SSE stream (agents/alerts/activity, 15s heartbeat). Demo auth: X-User-Key.
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"

	"flowsight/internal/agents"
	"flowsight/internal/config"
	"flowsight/internal/llm"
	"flowsight/internal/reports"
	"flowsight/internal/routines"
	"flowsight/internal/scheduler"
	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

// Server wires all handlers.
type Server struct {
	Cfg      config.Config
	DB       *store.DB
	Sectors  *sectors.Client
	Sched    *scheduler.Scheduler
	Engine   *routines.Engine
	Builder  *reports.Builder
	LLM      *llm.Client
	Validate *validator.Validate
	Hub      *Hub
}

// New builds a Server with all dependencies wired.
func New(cfg config.Config, db *store.DB, cache *store.Cache, s *sectors.Client) *Server {
	llmc := llm.New(cfg.LLMBaseURL, cfg.LLMAPIKey)
	sched := scheduler.New(cfg, db, cache, s)
	srv := &Server{
		Cfg: cfg, DB: db, Sectors: s, Sched: sched, LLM: llmc,
		Validate: validator.New(),
		Hub:      NewHub(),
	}
	srv.Engine = &routines.Engine{DB: db, Notifier: sched.Notifier, UserKey: cfg.DemoUserKey,
		Publish: srv.Hub.Publish}
	sched.Publish = srv.Hub.Publish
	srv.Builder = &reports.Builder{DB: db, Deps: agents.Deps{
		DB: db, LLM: llmc, TriageModel: cfg.LLMTriage, SynthModel: cfg.LLMSynth,
		Now: time.Now(),
	}}
	return srv
}

// Router returns the chi mux with all routes.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer, middleware.Heartbeat("/ping"))
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.Health)
		r.Get("/stream", s.Stream)
		r.Get("/flow/summary", s.FlowSummary)
		r.Get("/flow/broker", s.FlowBroker)
		r.Get("/flow/foreign", s.FlowForeign)
		r.Post("/screen", s.Screen)
		r.Get("/routines", s.ListRoutines)
		r.Post("/routines", s.CreateRoutine)
		r.Patch("/routines/{id}", s.UpdateRoutine)
		r.Delete("/routines/{id}", s.DeleteRoutine)
		r.Get("/routine-runs", s.RunHistory)
		r.Get("/briefing/today", s.BriefingToday)
		r.Get("/alerts", s.ListAlerts)
		r.Post("/alerts", s.CreateAlert)
		r.Delete("/alerts/{id}", s.DeleteAlert)
		r.Get("/alert-events", s.AlertEvents)
		r.Get("/destinations", s.ListDestinations)
		r.Post("/destinations", s.CreateDestination)
		r.Patch("/destinations/{id}", s.UpdateDestination)
		r.Delete("/destinations/{id}", s.DeleteDestination)
		r.Post("/report/{ticker}", s.BuildReport)
		r.Post("/report/{ticker}/ask", s.Interrogate)
		r.Get("/watchlist", s.GetWatchlist)
		r.Post("/watchlist", s.AddWatch)
		r.Delete("/watchlist/{ticker}", s.RemoveWatch)
		r.Get("/portfolio/risk", s.PortfolioRisk)
		r.Get("/accuracy", s.Accuracy)
		r.Post("/chat", s.Chat)
	})
	return r
}

// userKey resolves the demo auth header (single demo key for hackathon).
func (s *Server) userKey(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-User-Key")); k != "" {
		return k
	}
	return s.Cfg.DemoUserKey
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": map[string]string{"code": http.StatusText(code), "message": msg}})
}
