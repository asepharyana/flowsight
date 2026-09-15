package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Interrogate serves POST /api/report/:ticker/ask {question, report_id?}:
// follow-up Q&A grounded ONLY in that report's persisted citations.
// Without report_id it uses the latest report for the ticker.
func (s *Server) Interrogate(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	var req struct {
		Question string `json:"question" validate:"required"`
		ReportID int64  `json:"report_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "question is required")
		return
	}
	payload, citesRaw, at, id, err := s.loadReport(ticker, req.ReportID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no report for "+ticker+" yet — POST /api/report/"+ticker+" first")
		return
	}
	var cites []map[string]any
	_ = json.Unmarshal([]byte(citesRaw), &cites)
	answer := groundedAnswer(req.Question, payload)
	if s.LLM.Available() {
		if text, err := s.LLM.Complete(r.Context(), s.Cfg.LLMTriage,
			"Answer ONLY from the report JSON below. Every number must quote its cited value. "+
				"If the report lacks the answer, say exactly: not in this report.",
			"Question: "+req.Question+"\n\nReport:\n"+head(payload, 3000), 400); err == nil && text != "" {
			answer = strings.TrimSpace(text)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"answer": answer, "ticker": ticker, "report_id": id,
		"generated_at": at, "citations": cites,
	})
}

// loadReport fetches (payload, citations, generated_at, id) for an explicit
// report id or the latest report for a ticker.
func (s *Server) loadReport(ticker string, id int64) (string, string, string, int64, error) {
	if id > 0 {
		var t, p, c, at string
		var rid int64
		err := s.DB.QueryRow(`SELECT id, ticker, payload_json, citations_json, generated_at
			FROM reports WHERE id=?`, id).Scan(&rid, &t, &p, &c, &at)
		if err != nil {
			return "", "", "", 0, err
		}
		if t != ticker {
			return "", "", "", 0, fmt.Errorf("report %d belongs to %s", id, t)
		}
		return p, c, at, rid, nil
	}
	p, c, at, err := s.DB.LatestReport(ticker)
	if err != nil {
		return "", "", "", 0, err
	}
	var rid int64
	_ = s.DB.QueryRow(`SELECT id FROM reports WHERE ticker=? ORDER BY id DESC LIMIT 1`,
		ticker).Scan(&rid)
	return p, c, at, rid, nil
}

// groundedAnswer is the offline fallback: it extracts the recommendation +
// conviction + cited lines matching question keywords from the payload.
func groundedAnswer(question, payload string) string {
	var rep struct {
		Synthesis struct {
			Recommendation string `json:"recommendation"`
			Conviction     int    `json:"conviction"`
			Thesis         string `json:"thesis"`
		} `json:"synthesis"`
		Sections []struct {
			Name string `json:"name"`
			Body string `json:"body"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(payload), &rep); err != nil {
		return "not in this report"
	}
	q := strings.ToLower(question)
	if strings.Contains(q, "conviction") || strings.Contains(q, "kenapa") || strings.Contains(q, "why") {
		return fmt.Sprintf("%s with conviction %d/5: %s",
			rep.Synthesis.Recommendation, rep.Synthesis.Conviction, rep.Synthesis.Thesis)
	}
	for _, sec := range rep.Sections {
		if strings.Contains(q, strings.ToLower(sec.Name)) {
			return sec.Body
		}
	}
	return fmt.Sprintf("%s (conviction %d/5): %s",
		rep.Synthesis.Recommendation, rep.Synthesis.Conviction, rep.Synthesis.Thesis)
}
