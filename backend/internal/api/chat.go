package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ChatRequest is POST /api/chat {message, scope?: {report_id}}.
type ChatRequest struct {
	Message string `json:"message" validate:"required"`
	Scope   *struct {
		ReportID int64 `json:"report_id"`
	} `json:"scope"`
}

// Chat serves POST /api/chat: cited answers. With scope.report_id the
// grounding is restricted to that report's citations (report interrogation);
// without scope it answers from latest snapshots. The LLM refines prose only
// — numbers always come from stored data, never from generation.
func (s *Server) Chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "message is required")
		return
	}
	// Scope grounding: report citations when scoped (owner-scoped).
	var ground, citesRaw string
	if req.Scope != nil && req.Scope.ReportID > 0 {
		var cites string
		var at string
		err := s.DB.QueryRow(`SELECT payload_json, citations_json, generated_at FROM reports WHERE id=? AND user_key=?`,
			req.Scope.ReportID, s.userKey(r)).Scan(&ground, &cites, &at)
		if err != nil {
			writeErr(w, http.StatusNotFound, "report not found")
			return
		}
		citesRaw = cites
	} else {
		// Unscoped: ground on the caller's own briefing + watchlist (never the
		// global briefing, which may embed another user's watchlist numbers).
		uk := s.userKey(r)
		_, gPayload, gCites, _, gErr := s.DB.LatestBriefing()
		if p, cc, err := s.Engine.BriefingFor(r.Context(), uk); err == nil {
			ccJSON, _ := json.Marshal(cc)
			ground, citesRaw = p, string(ccJSON)
		} else if gErr == nil {
			ground, citesRaw = gPayload, gCites
		} else {
			ground = "no briefing or report data yet"
		}
	}
	answer := "Based on stored data: " + head(ground, 600)
	if s.LLM.Available() {
		if text, err := s.LLM.Complete(r.Context(), s.Cfg.LLMTriage,
			"You answer questions about Indonesian stocks using ONLY the grounded data below. "+
				"Every number in your answer must cite its source. If the data lacks the answer, say so.",
			"Question: "+req.Message+"\n\nGrounded data:\n"+head(ground, 3000), 400); err == nil {
			answer = strings.TrimSpace(text)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"answer": answer, "grounding": head(ground, 600), "citations": citesRaw,
	})
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
