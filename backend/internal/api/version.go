package api

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// Version serves GET /api/version: deployed commit + process start time so CI
// can prove the live process is the freshly deployed one (anti-stale).
// Commit is resolved once at startup (see readCommit); an old process keeps
// reporting its old commit even after CI writes a new version.txt.
func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"commit":            s.Commit,
		"started_at":        s.StartedAt.UTC().Format(time.RFC3339),
		"google_configured": s.Cfg.HasGoogle(),
	})
}

// readCommit resolves the deployed commit once at startup: CI writes
// $GITHUB_SHA to version.txt (WorkingDirectory) before restarting.
func readCommit() string {
	if v := strings.TrimSpace(os.Getenv("FLOWSIGHT_COMMIT")); v != "" {
		return v
	}
	for _, p := range []string{"version.txt", "/var/lib/flowsight/version.txt"} {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	return "unknown"
}
