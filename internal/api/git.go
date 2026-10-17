// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
	"github.com/runwisp/runwisp-manager/internal/gitx"
)

type GitAPI struct {
	Git     *gitx.Git
	Repo    gitx.Repository
	Watcher *cfgtree.Watcher
	Auth    *auth.Authenticator
	Audit   audit.Emitter
	Options gitx.CommitOptions
}

func (a *GitAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/status", a.status)
	r.Get("/diff", a.diff)
	r.Get("/log", a.log)
	r.Post("/commit", a.commit)
	return r
}

func (a *GitAPI) status(w http.ResponseWriter, r *http.Request) {
	value, err := a.Git.Status(r.Context(), a.Repo, a.managed())
	if err != nil {
		problem(w, http.StatusBadGateway, "git status unavailable")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *GitAPI) diff(w http.ResponseWriter, r *http.Request) {
	path, ok := a.resolvePath(r.URL.Query().Get("path"))
	if !ok {
		problem(w, http.StatusForbidden, "path is not managed")
		return
	}
	diff, err := a.Git.Diff(r.Context(), path, r.URL.Query().Get("staged") == "true")
	if err != nil {
		problem(w, http.StatusBadGateway, "git diff unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": r.URL.Query().Get("path"), "diff": diff})
}

func (a *GitAPI) log(w http.ResponseWriter, r *http.Request) {
	path := ""
	if raw := r.URL.Query().Get("path"); raw != "" {
		var ok bool
		path, ok = a.resolvePath(raw)
		if !ok {
			problem(w, http.StatusForbidden, "path is not managed")
			return
		}
	}
	entries, err := a.Git.Log(r.Context(), path, 200)
	if err != nil {
		problem(w, http.StatusBadGateway, "git log unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (a *GitAPI) commit(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "editor" && identity.Role != "admin" {
		problem(w, http.StatusForbidden, "committing requires editor role")
		return
	}
	var request struct {
		Paths    []string `json:"paths"`
		Message  string   `json:"message"`
		Override *struct {
			Reason string `json:"reason"`
		} `json:"overrideSecretGuard,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid commit request")
		return
	}
	paths := make([]string, 0, len(request.Paths))
	for _, id := range request.Paths {
		path, ok := a.resolvePath(id)
		if !ok {
			problem(w, http.StatusForbidden, "path is not managed")
			return
		}
		rel, err := filepath.Rel(a.Git.Repo(), path)
		if err != nil {
			problem(w, http.StatusForbidden, "path is outside repository")
			return
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	status, err := a.Git.Status(r.Context(), a.Repo, a.managed())
	if err != nil {
		problem(w, http.StatusBadGateway, "git status unavailable")
		return
	}
	if !hasChanges(status, paths) {
		problem(w, http.StatusConflict, "nothing staged")
		return
	}
	reason := ""
	if request.Override != nil {
		if identity.Role != "admin" || strings.TrimSpace(request.Override.Reason) == "" {
			problem(w, http.StatusForbidden, "admin override requires a written reason")
			return
		}
		reason = strings.TrimSpace(request.Override.Reason)
	}
	if err := a.Git.CheckGuards(r.Context(), a.Repo, paths, a.Options, reason); err != nil {
		status := http.StatusConflict
		if errors.Is(err, gitx.ErrScopeGuard) || errors.Is(err, gitx.ErrInProgress) {
			status = http.StatusForbidden
		}
		problem(w, status, err.Error())
		return
	}
	result, err := a.Git.Commit(r.Context(), gitx.CommitRequest{Paths: paths, Message: request.Message, OverrideReason: reason, Actor: gitx.Actor{Username: identity.Username, Name: identity.Name, Email: identity.Email}}, a.Options)
	if err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	a.Audit.Emit(r.Context(), audit.Event{Action: "git.commit", Subject: result.SHA, Username: identity.Username, Role: identity.Role, Success: true, Reason: reason})
	writeJSON(w, http.StatusOK, result)
}

func (a *GitAPI) managed() []gitx.ManagedPath {
	if a.Watcher == nil {
		return nil
	}
	files := a.Watcher.Tree().Files
	result := make([]gitx.ManagedPath, 0, len(files))
	for _, file := range files {
		if !file.Outside && file.Readable {
			result = append(result, gitx.ManagedPath{ID: file.ID, Path: file.Path})
		}
	}
	return result
}

func (a *GitAPI) resolvePath(id string) (string, bool) {
	for _, file := range a.managed() {
		if file.ID == id || file.Path == id {
			return file.Path, true
		}
	}
	return "", false
}

func hasChanges(status gitx.Status, paths []string) bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[path] = true
	}
	for _, file := range status.Files {
		if set[file.Path] && (file.Staged || file.Unstaged || file.Untracked) {
			return true
		}
	}
	return false
}
