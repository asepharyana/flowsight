package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// BuildReport serves POST /api/report/:ticker?format=json|html|pdf|md.
// Runs A1..A6 + A7 live over stored snapshots, persists, and renders.
func (s *Server) BuildReport(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	if len(ticker) < 3 || len(ticker) > 6 {
		writeErr(w, http.StatusUnprocessableEntity, "ticker must be 3-6 letters")
		return
	}
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		profile = "moderate"
	}
	rep, id, err := s.Builder.Build(r.Context(), ticker, profile)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "report: "+err.Error())
		return
	}
	_ = id
	switch strings.ToLower(r.URL.Query().Get("format")) {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rep.ToHTML()))
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rep.ToMarkdown()))
	case "pdf":
		raw, err := rep.ToPDF()
		if err != nil {
			writeErr(w, http.StatusBadGateway, "pdf: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	default:
		writeJSON(w, http.StatusOK, rep)
	}
	// Push a live agent-panel event for the dashboard SSE feed.
	s.Hub.Publish("agents", `{"ticker":"`+ticker+`","recommendation":"`+
		rep.Synthesis.Recommendation+`"}`)
}
