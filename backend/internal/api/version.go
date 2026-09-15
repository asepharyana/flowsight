package api

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// Version serves GET /api/version: deployed commit + process start time so CI
// can prove the live process is the freshly deployed one (anti-stale).
// Commit is read LIVE per request (not cached): CI writes version.txt before
// restarting, and a stale process is caught because its started_at predates
// the deploy. Both fields together = full proof.
func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"commit":            readCommitLive(),
		"started_at":        s.StartedAt.UTC().Format(time.RFC3339),
		"google_configured": s.Cfg.HasGoogle(),
	})
}

// readCommitLive resolves the deployed commit per request: version.txt
// (written by CI before every restart) wins, then startup env.
func readCommitLive() string {
	for _, p := range []string{"/var/lib/flowsight/version.txt", "version.txt"} {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("FLOWSIGHT_COMMIT")); v != "" {
		return v
	}
	return "unknown"
}
