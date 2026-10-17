// SPDX-License-Identifier: GPL-3.0-or-later
package ui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var assets embed.FS

// Handler serves the manager SPA under a router that has already stripped
// /manager. It deliberately keeps API paths separate from the SPA fallback.
func Handler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic("manager UI assets are malformed: " + err.Error())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." || name == "" || name == "/" {
			name = "index.html"
		}
		if strings.HasPrefix(name, "api/") {
			problem(w, http.StatusNotFound, "manager API route not found")
			return
		}
		if strings.HasPrefix(name, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if _, err := fs.Stat(root, name); err == nil {
			serveAsset(w, root, name)
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "text/html") && path.Ext(name) == "" {
			w.Header().Set("Cache-Control", "no-cache")
			serveAsset(w, root, "index.html")
			return
		}
		http.NotFound(w, r)
	})
}

func serveAsset(w http.ResponseWriter, root fs.FS, name string) {
	data, err := fs.ReadFile(root, name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if contentType := http.DetectContentType(data); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"type":"about:blank","title":"Not Found","status":404,"detail":"` + detail + `"}`))
}
