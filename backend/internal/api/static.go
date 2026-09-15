// Package api serves the FlowSight REST API (docs/API.md) on chi.
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// spaHandler serves the prebuilt web dist (WEB_DIST_DIR) with an SPA fallback:
// existing files (assets, index.html) are served directly, unknown paths fall
// back to index.html so client-side routes (/routines, /report/:ticker, ...)
// resolve. Only registered when StaticDir is set; API-only mode otherwise.
func (s *Server) spaHandler() http.HandlerFunc {
	root := s.Cfg.StaticDir
	fileSrv := http.FileServer(http.Dir(root))
	index := filepath.Join(root, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.ServeFile(w, r, index)
			return
		}
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if strings.Contains(clean, "..") {
			http.NotFound(w, r)
			return
		}
		if fi, err := os.Stat(filepath.Join(root, clean)); err == nil && !fi.IsDir() {
			fileSrv.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	}
}
