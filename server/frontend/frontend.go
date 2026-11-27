// Package frontend serves the embedded React build with SPA fallback routing.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves static assets; unknown non-API paths return index.html so
// client-side routes survive a reload.
func Handler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	files := http.FileServer(http.FS(sub))
	index, indexErr := fs.ReadFile(sub, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if strings.HasPrefix(p, "api/") || p == "api" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"not_found","message":"No such API endpoint"}}`))
			return
		}
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(sub, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			// Missing asset files should 404 rather than return HTML.
			if path.Ext(p) != "" && !strings.HasSuffix(p, ".html") {
				http.NotFound(w, r)
				return
			}
		}
		if indexErr != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`<!doctype html><title>Family Holocron</title><p>The web interface has not been built. Run <code>npm run build</code> in <code>web/</code> and rebuild the binary.</p>`))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
