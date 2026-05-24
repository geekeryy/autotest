//go:build embedadmin

package main

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// webDist is populated at Docker build time from the Vite output copied to cmd/api/webdist.
//
//go:embed all:webdist
var webDist embed.FS

func registerAdminUI(r chi.Router) {
	sub, err := fs.Sub(webDist, "webdist")
	if err != nil {
		return
	}
	r.Handle("/*", spaHandler(sub))
}

func spaHandler(staticFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		serveStaticFile(w, r, staticFS, path)
	})
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, staticFS fs.FS, path string) {
	data, err := fs.ReadFile(staticFS, path)
	if err != nil {
		if path != "index.html" {
			serveStaticFile(w, r, staticFS, "index.html")
			return
		}
		http.NotFound(w, r)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
