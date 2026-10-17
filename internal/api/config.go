// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
)

type ConfigAPI struct {
	Watcher *cfgtree.Watcher
	Auth    *auth.Authenticator
	Audit   audit.Emitter
	mu      sync.Mutex
	reveals []time.Time
}

func (a *ConfigAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/config/tree", a.tree)
	r.Get("/config/files/{id}", a.file)
	r.Get("/units", a.units)
	r.Get("/units/{kind}/{name}", a.unit)
	r.Get("/config/events", a.events)
	r.Post("/config/secrets:reveal", a.reveal)
	return r
}

func (a *ConfigAPI) tree(w http.ResponseWriter, _ *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	tree := a.Watcher.Tree()
	for i := range tree.Units {
		tree.Units[i].Fields = cfgtree.MaskedFields(tree.Units[i].Fields)
	}
	writeJSON(w, http.StatusOK, tree)
}

func (a *ConfigAPI) file(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	for _, file := range a.Watcher.Tree().Files {
		if file.ID != id {
			continue
		}
		response := file
		if r.URL.Query().Get("raw") == "true" {
			if identity, ok := a.Auth.IdentityFromRequest(r); !ok || identity.Role != "admin" {
				problem(w, http.StatusForbidden, "raw configuration requires admin")
				return
			}
			if file.Outside || !file.Readable {
				problem(w, http.StatusForbidden, "file is outside managed area")
				return
			}
			data, err := os.ReadFile(file.Path)
			if err != nil {
				problem(w, http.StatusNotFound, "file is no longer readable")
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write(data)
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	problem(w, http.StatusNotFound, "file not found")
}

func (a *ConfigAPI) units(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	kind, query := r.URL.Query().Get("kind"), strings.ToLower(r.URL.Query().Get("q"))
	units := make([]map[string]any, 0)
	for _, unit := range a.Watcher.Tree().Units {
		if kind != "" && unit.Kind != kind {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(unit.Name), query) && !strings.Contains(strings.ToLower(unit.File), query) {
			continue
		}
		schedule := ""
		if value, ok := unit.Fields["schedule"].(string); ok {
			schedule = value
		}
		units = append(units, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File, "line": unit.Line, "hash": unit.Hash, "duplicate": unit.Duplicate, "schedule": schedule})
	}
	writeJSON(w, http.StatusOK, map[string]any{"units": units})
}

func (a *ConfigAPI) unit(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	kind, name := chi.URLParam(r, "kind"), chi.URLParam(r, "name")
	for _, unit := range a.Watcher.Tree().Units {
		if unit.Kind == kind && unit.Name == name {
			writeJSON(w, http.StatusOK, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File, "line": unit.Line, "hash": unit.Hash, "fileVersion": fileVersion(a.Watcher.Tree(), unit.File), "duplicate": unit.Duplicate, "fields": cfgtree.MaskedFields(unit.Fields)})
			return
		}
	}
	problem(w, http.StatusNotFound, "unit not found")
}

func (a *ConfigAPI) reveal(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "admin" {
		problem(w, http.StatusForbidden, "secret reveal requires admin")
		return
	}
	if !a.allowReveal() {
		problem(w, http.StatusTooManyRequests, "secret reveal rate limit exceeded")
		return
	}
	var request struct{ File, KeyPath string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid reveal request")
		return
	}
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	for _, unit := range a.Watcher.Tree().Units {
		if unit.File != request.File {
			continue
		}
		value, err := cfgtree.Reveal(unit, request.KeyPath)
		if err != nil {
			continue
		}
		w.Header().Set("Cache-Control", "no-store")
		if a.Audit != nil {
			a.Audit.Emit(r.Context(), audit.Event{Action: "secret.reveal", Subject: identity.Sub, Username: identity.Username, Role: identity.Role, Success: true})
		}
		writeJSON(w, http.StatusOK, map[string]any{"value": value})
		return
	}
	problem(w, http.StatusNotFound, "secret field not found")
}

func (a *ConfigAPI) events(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		problem(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	writeEvent := func(event cfgtree.Event) {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: config\ndata: %s\n\n", data)
		flusher.Flush()
	}
	writeEvent(cfgtree.Event{TreeVersion: a.Watcher.Tree().TreeVersion, Stale: false})
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-a.Watcher.Events():
			if !ok {
				return
			}
			writeEvent(event)
		case <-ticker.C:
			if _, ok := a.Auth.IdentityFromRequest(r); !ok {
				return
			}
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (a *ConfigAPI) allowReveal() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := a.reveals[:0]
	for _, when := range a.reveals {
		if when.After(cutoff) {
			kept = append(kept, when)
		}
	}
	a.reveals = append(kept, now)
	return len(a.reveals) <= 10
}
func fileVersion(tree cfgtree.Tree, path string) string {
	for _, file := range tree.Files {
		if file.Path == path {
			return file.Version
		}
	}
	return ""
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}
