// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/cfgsave"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
	"github.com/runwisp/runwisp-manager/internal/history"
	"github.com/runwisp/runwisp-manager/internal/tomledit"
)

type ConfigAPI struct {
	Watcher *cfgtree.Watcher
	Auth    *auth.Authenticator
	Audit   audit.Emitter
	Save    *cfgsave.Pipeline
	History *history.Service
	mu      sync.Mutex
	reveals []time.Time
}

func (a *ConfigAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/config/tree", a.tree)
	r.Get("/config/files/{id}", a.file)
	r.Get("/units", a.units)
	r.Get("/units/{kind}", a.singleton)
	r.Get("/units/{kind}/{name}", a.unit)
	r.Get("/units/{kind}/{name}/references", a.references)
	r.Get("/units/{kind}/{name}/history", a.historyList)
	r.Get("/units/{kind}/{name}/history/{versionId}", a.historyVersion)
	r.Get("/units/{kind}/{name}/history/{versionId}/diff", a.historyDiff)
	r.Post("/units/{kind}/{name}/history/{versionId}:restore", a.historyRestore)
	r.Get("/config/events", a.events)
	r.Post("/config/secrets:reveal", a.reveal)
	r.Post("/config/includes:preview", a.includesPreview)
	r.Post("/config/files", a.createFile)
	r.Post("/config/routes:reorder", a.reorderRoutes)
	r.Post("/config/routes:matches", a.routeMatches)
	r.Post("/config/units:move", a.moveUnit)
	r.Post("/config/changes:preview", a.changesPreview)
	r.Post("/config/changes", a.changes)
	r.Post("/config/reload", a.reload)
	r.Get("/config/backups", a.backups)
	return r
}

func (a *ConfigAPI) historyTarget(r *http.Request) (string, []string, bool) {
	if a.History == nil || a.Watcher == nil {
		return "", nil, false
	}
	kind, name := chi.URLParam(r, "kind"), chi.URLParam(r, "name")
	for _, unit := range a.Watcher.Tree().Units {
		if unit.Kind == kind && unit.Name == name {
			return unit.File, []string{kind, name}, true
		}
	}
	return "", nil, false
}
func (a *ConfigAPI) historyList(w http.ResponseWriter, r *http.Request) {
	file, path, ok := a.historyTarget(r)
	if !ok {
		problem(w, http.StatusNotFound, "unit history unavailable")
		return
	}
	values, err := a.History.List(r.Context(), file, path, false)
	if err != nil {
		problem(w, http.StatusInternalServerError, "history unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": values})
}
func (a *ConfigAPI) historyVersion(w http.ResponseWriter, r *http.Request) {
	file, path, ok := a.historyTarget(r)
	if !ok {
		problem(w, http.StatusNotFound, "unit history unavailable")
		return
	}
	identity, _ := a.Auth.IdentityFromRequest(r)
	v, err := a.History.Text(r.Context(), file, path, chi.URLParam(r, "versionId"), identity.Role == "admin")
	if err != nil {
		problem(w, http.StatusNotFound, "history version not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (a *ConfigAPI) historyDiff(w http.ResponseWriter, r *http.Request) {
	file, path, ok := a.historyTarget(r)
	if !ok {
		problem(w, http.StatusNotFound, "unit history unavailable")
		return
	}
	identity, _ := a.Auth.IdentityFromRequest(r)
	v, err := a.History.Text(r.Context(), file, path, chi.URLParam(r, "versionId"), identity.Role == "admin")
	if err != nil {
		problem(w, http.StatusNotFound, "history version not found")
		return
	}
	current := []byte{}
	if data, e := os.ReadFile(file); e == nil {
		if doc, e := tomledit.Parse(data); e == nil {
			current, _ = doc.ExtractUnit(path)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": cfgsave.UnifiedDiff([]byte(v.Text), current, file)})
}

func (a *ConfigAPI) historyRestore(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || (identity.Role != "editor" && identity.Role != "admin") {
		problem(w, http.StatusForbidden, "restoring history requires editor role")
		return
	}
	file, path, ok := a.historyTarget(r)
	if !ok || a.Save == nil {
		problem(w, http.StatusNotFound, "unit history unavailable")
		return
	}
	version, err := a.History.Text(r.Context(), file, path, chi.URLParam(r, "versionId"), true)
	if err != nil {
		problem(w, http.StatusNotFound, "history version not found")
		return
	}
	current, err := os.ReadFile(file)
	if err != nil {
		problem(w, http.StatusNotFound, "current file unavailable")
		return
	}
	doc, err := tomledit.Parse(current)
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "current file is invalid")
		return
	}
	updated, err := doc.ReplaceUnit(path, []byte(version.Text))
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "history version cannot be restored")
		return
	}
	sum := sha256.Sum256(current)
	change := cfgsave.Change{IdempotencyKey: r.Header.Get("Idempotency-Key"), Base: map[cfgsave.FileID]string{cfgsave.FileID(file): hex.EncodeToString(sum[:])}, Raw: []cfgsave.FileContent{{File: cfgsave.FileID(file), Content: updated}}, Apply: true, Rollback: true, Reason: "restore history version " + version.ID}
	result, err := a.Save.WithActor(audit.Event{Subject: identity.Sub, Username: identity.Username, Email: identity.Email, Role: identity.Role}).Save(r.Context(), change)
	if err != nil {
		writeChangeError(w, err, result)
		return
	}
	if a.Audit != nil {
		a.Audit.Emit(r.Context(), audit.Event{Action: "config.rollback", Subject: identity.Sub, Username: identity.Username, Email: identity.Email, Role: identity.Role, Outcome: "ok", Success: true, TargetKind: chi.URLParam(r, "kind"), TargetName: chi.URLParam(r, "name"), TargetFile: file, GitSHA: version.SHA, Reason: version.ID})
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *ConfigAPI) includesPreview(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Globs []string `json:"globs"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid include preview request")
		return
	}
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	preview, err := cfgtree.PreviewIncludes(a.Watcher.Tree().Root, request.Globs)
	if err != nil {
		problem(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *ConfigAPI) createFile(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "admin" {
		problem(w, http.StatusForbidden, "creating include files requires admin")
		return
	}
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	var request struct {
		Path     string `json:"path"`
		Template string `json:"template"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil || request.Path == "" {
		problem(w, http.StatusBadRequest, "path is required")
		return
	}
	root := a.Watcher.Tree().Root
	path, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(request.Path)))
	if err != nil || !withinRoot(root, path) || filepath.Ext(path) != ".toml" {
		problem(w, http.StatusBadRequest, "file must be a managed relative .toml path")
		return
	}
	if _, err := os.Stat(path); err == nil {
		problem(w, http.StatusConflict, "file already exists")
		return
	}
	content := "# Managed include file. Add RunWisp units below.\n"
	if request.Template == "task" {
		content += "\n[task.example]\n# schedule = \"0 * * * *\"\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		problem(w, http.StatusInternalServerError, "create include directory failed")
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		problem(w, http.StatusInternalServerError, "create include file failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"path": path, "message": "File created. Add its relative glob to daemon includes before saving."})
}

func (a *ConfigAPI) reorderRoutes(w http.ResponseWriter, r *http.Request) {
	if !a.requireEditor(w, r) {
		return
	}
	var request struct {
		File     cfgsave.FileID `json:"file"`
		Base     string         `json:"base"`
		From, To int
		Apply    bool `json:"apply"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil || request.File == "" || request.From < 0 || request.To < 0 {
		problem(w, http.StatusBadRequest, "file, base, from, and to are required")
		return
	}
	if a.Save == nil {
		problem(w, http.StatusServiceUnavailable, "save pipeline unavailable")
		return
	}
	change := cfgsave.Change{IdempotencyKey: r.Header.Get("Idempotency-Key"), Base: map[cfgsave.FileID]string{request.File: request.Base}, Apply: request.Apply, Reason: "reorder notification routes", Ops: []cfgsave.FileOps{{File: request.File, Ops: []tomledit.Op{tomledit.ReorderArrayTable{Path: []string{"route"}, From: request.From, To: request.To}}}}}
	result, err := a.Save.WithActor(actorFromRequest(a, r)).Save(r.Context(), change)
	if err != nil {
		writeChangeError(w, err, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *ConfigAPI) routeMatches(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Pattern string `json:"pattern"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid route match request")
		return
	}
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	matches := make([]string, 0)
	for _, unit := range a.Watcher.Tree().Units {
		if unit.Kind != "task" {
			continue
		}
		matched, err := path.Match(request.Pattern, unit.Name)
		if err == nil && matched {
			matches = append(matches, unit.Name)
		}
	}
	sort.Strings(matches)
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

func (a *ConfigAPI) moveUnit(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "admin" {
		problem(w, http.StatusForbidden, "moving units requires admin")
		return
	}
	if a.Watcher == nil || a.Save == nil {
		problem(w, http.StatusServiceUnavailable, "configuration save unavailable")
		return
	}
	var request struct {
		Source string                    `json:"source"`
		Target string                    `json:"target"`
		Path   []string                  `json:"path"`
		Base   map[cfgsave.FileID]string `json:"base"`
		Apply  bool                      `json:"apply"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil || request.Source == "" || request.Target == "" || len(request.Path) == 0 || request.Source == request.Target {
		problem(w, http.StatusBadRequest, "source, target, and unit path are required")
		return
	}
	sourcePath, sourceOK := managedFilePath(a.Watcher.Tree(), request.Source)
	targetPath, targetOK := managedFilePath(a.Watcher.Tree(), request.Target)
	if !sourceOK || !targetOK {
		problem(w, http.StatusForbidden, "source and target must be managed files")
		return
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		problem(w, http.StatusNotFound, "source file is unreadable")
		return
	}
	target, err := os.ReadFile(targetPath)
	if err != nil {
		problem(w, http.StatusNotFound, "target file is unreadable")
		return
	}
	sourceDoc, err := tomledit.Parse(source)
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "source TOML is invalid")
		return
	}
	unit, span := sourceDoc.ExtractUnit(request.Path)
	if unit == nil || span[0] == span[1] {
		problem(w, http.StatusNotFound, "unit not found")
		return
	}
	updatedSource, err := sourceDoc.Apply([]tomledit.Op{tomledit.DeleteTable{Path: request.Path}})
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "could not extract unit")
		return
	}
	separator := []byte("\n")
	if bytes.Contains(target, []byte("\r\n")) {
		separator = []byte("\r\n")
	}
	updatedTarget := append(append(bytes.Clone(target), separator...), unit...)
	change := cfgsave.Change{IdempotencyKey: r.Header.Get("Idempotency-Key"), Base: request.Base, Apply: request.Apply, Reason: "move configuration unit", Raw: []cfgsave.FileContent{{File: cfgsave.FileID(request.Source), Content: updatedSource}, {File: cfgsave.FileID(request.Target), Content: updatedTarget}}}
	result, err := a.Save.WithActor(actorFromRequest(a, r)).Save(r.Context(), change)
	if err != nil {
		writeChangeError(w, err, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func managedFilePath(tree cfgtree.Tree, id string) (string, bool) {
	for _, file := range tree.Files {
		if file.ID == id || file.Path == id {
			return file.Path, file.Readable && !file.Outside
		}
	}
	return "", false
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (a *ConfigAPI) singleton(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	kind := chi.URLParam(r, "kind")
	for _, unit := range a.Watcher.Tree().Units {
		if unit.Kind != kind || unit.Name != "" {
			continue
		}
		fileID := ""
		for _, file := range a.Watcher.Tree().Files {
			if file.Path == unit.File {
				fileID = file.ID
				break
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File, "fileId": fileID, "line": unit.Line, "hash": unit.Hash, "fileVersion": fileVersion(a.Watcher.Tree(), unit.File), "fields": cfgtree.MaskedFields(unit.Fields)})
		return
	}
	problem(w, http.StatusNotFound, "singleton unit not found")
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
		fileID, version := "", ""
		for _, file := range a.Watcher.Tree().Files {
			if file.Path == unit.File {
				fileID, version = file.ID, file.Version
				break
			}
		}
		units = append(units, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File, "fileId": fileID, "fileVersion": version, "line": unit.Line, "hash": unit.Hash, "duplicate": unit.Duplicate, "schedule": schedule})
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
			fileID := ""
			for _, file := range a.Watcher.Tree().Files {
				if file.Path == unit.File {
					fileID = file.ID
					break
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File, "fileId": fileID, "line": unit.Line, "hash": unit.Hash, "fileVersion": fileVersion(a.Watcher.Tree(), unit.File), "duplicate": unit.Duplicate, "fields": cfgtree.MaskedFields(unit.Fields)})
			return
		}
	}
	problem(w, http.StatusNotFound, "unit not found")
}

func (a *ConfigAPI) references(w http.ResponseWriter, r *http.Request) {
	if a.Watcher == nil {
		problem(w, http.StatusServiceUnavailable, "configuration tree unavailable")
		return
	}
	kind, name := chi.URLParam(r, "kind"), chi.URLParam(r, "name")
	if kind != "notifier" {
		writeJSON(w, http.StatusOK, map[string]any{"references": []any{}})
		return
	}
	refs := make([]map[string]any, 0)
	for _, unit := range a.Watcher.Tree().Units {
		if unit.Kind == "notifier" || !containsReference(unit.Fields, name) {
			continue
		}
		refs = append(refs, map[string]any{"kind": unit.Kind, "name": unit.Name, "file": unit.File})
	}
	writeJSON(w, http.StatusOK, map[string]any{"references": refs})
}

func containsReference(value any, target string) bool {
	switch v := value.(type) {
	case string:
		return v == target || strings.HasPrefix(v, target+":")
	case []any:
		for _, item := range v {
			if containsReference(item, target) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if containsReference(item, target) {
				return true
			}
		}
	}
	return false
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
