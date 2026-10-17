// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/cfgsave"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
	"github.com/runwisp/runwisp-manager/internal/gitx"
)

type GitAPI struct {
	Git           *gitx.Git
	Repo          gitx.Repository
	Watcher       *cfgtree.Watcher
	Auth          *auth.Authenticator
	Audit         audit.Emitter
	Options       gitx.CommitOptions
	Sync          *gitx.Sync
	Save          *cfgsave.Pipeline
	Remote        string
	PullAllow     []string
	PushBranches  []string
	AutoApplyPull bool
}

func (a *GitAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/status", a.status)
	r.Get("/diff", a.diff)
	r.Get("/log", a.log)
	r.Post("/commit", a.commit)
	r.Post("/fetch", a.fetch)
	r.Post("/pull", a.pull)
	r.Post("/push", a.push)
	r.Get("/conflicts", a.conflicts)
	r.Get("/conflicts/{file}", a.conflict)
	r.Post("/conflicts/{file}:resolve", a.resolve)
	r.Post("/merge:continue", a.continueMerge)
	r.Post("/merge:abort", a.abortMerge)
	r.Post("/pull:undo", a.undo)
	return r
}

func (a *GitAPI) admin(r *http.Request) bool {
	identity, ok := a.Auth.IdentityFromRequest(r)
	return ok && identity.Role == "admin"
}

func (a *GitAPI) fetch(w http.ResponseWriter, r *http.Request) {
	if a.Sync == nil {
		problem(w, http.StatusServiceUnavailable, "git sync unavailable")
		return
	}
	if result := a.Sync.Fetch(r.Context()); result.Error != "" {
		problem(w, http.StatusBadGateway, "git fetch failed")
		return
	} else {
		writeJSON(w, http.StatusOK, result)
	}
}

func (a *GitAPI) pull(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "pull requires admin")
		return
	}
	var request gitx.PullRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		problem(w, http.StatusBadRequest, "invalid pull request")
		return
	}
	operation := func() (gitx.PullResult, error) { return a.Git.Pull(r.Context(), a.Repo, request, a.PullAllow) }
	var result gitx.PullResult
	var operationErr error
	if a.Save != nil {
		operationErr = a.Save.WithSync(r.Context(), func() error { result, operationErr = operation(); return operationErr })
	} else {
		result, operationErr = operation()
	}
	if operationErr != nil {
		problem(w, http.StatusConflict, operationErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *GitAPI) push(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "push requires admin")
		return
	}
	var request gitx.PushRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		problem(w, http.StatusBadRequest, "invalid push request")
		return
	}
	request.Remote = a.Remote
	request.Branches = a.PushBranches
	result, err := a.Git.Push(r.Context(), a.Repo, request)
	if err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *GitAPI) conflicts(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "conflicts require admin")
		return
	}
	result, err := a.Git.Conflicts(r.Context())
	if err != nil {
		problem(w, http.StatusConflict, "conflicts unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": result})
}

func (a *GitAPI) conflict(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "conflicts require admin")
		return
	}
	path := chi.URLParam(r, "file")
	result, err := a.Git.Conflicts(r.Context())
	if err != nil {
		problem(w, http.StatusConflict, "conflicts unavailable")
		return
	}
	for _, file := range result {
		if file.Path == path {
			writeJSON(w, http.StatusOK, file)
			return
		}
	}
	problem(w, http.StatusNotFound, "conflict not found")
}

func (a *GitAPI) resolve(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "resolving conflicts requires admin")
		return
	}
	var request struct {
		Action  string `json:"action"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid resolution")
		return
	}
	if err := a.Git.Resolve(r.Context(), chi.URLParam(r, "file"), request.Action, request.Content); err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"resolved": true})
}

func (a *GitAPI) continueMerge(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "continuing merge requires admin")
		return
	}
	operation := r.URL.Query().Get("operation")
	if operation == "" {
		operation = a.Repo.InProgress
	}
	if err := a.Git.Continue(r.Context(), operation); err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"continued": true})
}

func (a *GitAPI) abortMerge(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "aborting merge requires admin")
		return
	}
	operation := r.URL.Query().Get("operation")
	if operation == "" {
		operation = a.Repo.InProgress
	}
	if err := a.Git.Abort(r.Context(), operation); err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"aborted": true})
}

func (a *GitAPI) undo(w http.ResponseWriter, r *http.Request) {
	if !a.admin(r) {
		problem(w, http.StatusForbidden, "undo requires admin")
		return
	}
	var request struct {
		OrigHead   string `json:"origHead"`
		PulledHead string `json:"pulledHead"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "invalid undo request")
		return
	}
	if err := a.Git.UndoPull(r.Context(), request.OrigHead, request.PulledHead); err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"undone": true})
}

func (a *GitAPI) status(w http.ResponseWriter, r *http.Request) {
	value, err := a.Git.Status(r.Context(), a.Repo, a.managed())
	if err != nil {
		problem(w, http.StatusBadGateway, "git status unavailable")
		return
	}
	if a.Sync != nil {
		fetch := a.Sync.Result()
		value.Remote = fetch.Remote
		value.LastFetch = fetch.FetchedAt
		value.LastFetchError = fetch.Error
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
