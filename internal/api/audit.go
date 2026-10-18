// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
)

type AuditAPI struct {
	Store *audit.Store
	Auth  *auth.Authenticator
}

func (a *AuditAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", a.list)
	r.Get("/verify", a.verify)
	r.Get("/export", a.export)
	return r
}
func (a *AuditAPI) list(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, http.StatusServiceUnavailable, "audit unavailable")
		return
	}
	identity, _ := a.Auth.IdentityFromRequest(r)
	filter := audit.Filter{Actor: r.URL.Query().Get("actor"), Action: r.URL.Query().Get("action"), Target: r.URL.Query().Get("target"), Outcome: r.URL.Query().Get("outcome")}
	if identity.Role != "admin" {
		filter.Actor = identity.Username
	}
	if n, _ := strconv.Atoi(r.URL.Query().Get("limit")); n > 0 {
		filter.Limit = n
	}
	events, err := a.Store.List(r.Context(), filter)
	if err != nil {
		problem(w, http.StatusInternalServerError, "audit query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
func (a *AuditAPI) verify(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, http.StatusServiceUnavailable, "audit unavailable")
		return
	}
	id, err := a.Store.Verify(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"valid": err == nil, "firstBrokenID": id, "error": errorText(err)})
}
func (a *AuditAPI) export(w http.ResponseWriter, r *http.Request) {
	identity, _ := a.Auth.IdentityFromRequest(r)
	if identity.Role != "admin" {
		problem(w, http.StatusForbidden, "audit export requires admin")
		return
	}
	limit := 500
	if parsed, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && parsed > 0 {
		limit = parsed
	}
	filter := audit.Filter{Limit: limit}
	if value := r.URL.Query().Get("from"); value != "" {
		if t, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
			filter.From = t
		} else {
			problem(w, http.StatusBadRequest, "invalid from timestamp")
			return
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		if t, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
			filter.To = t
		} else {
			problem(w, http.StatusBadRequest, "invalid to timestamp")
			return
		}
	}
	events, err := a.Store.List(r.Context(), filter)
	if err != nil {
		problem(w, http.StatusInternalServerError, "audit export failed")
		return
	}
	if len(events) >= limit {
		w.Header().Set("X-Truncated", "true")
	}
	a.Store.Emit(r.Context(), audit.Event{Action: "admin.export", Username: identity.Username, Role: identity.Role, Success: true})
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		out := csv.NewWriter(w)
		_ = out.Write([]string{"id", "ts", "actor", "action", "outcome", "target"})
		for _, e := range events {
			_ = out.Write([]string{strconv.FormatInt(e.ID, 10), e.Time.UTC().Format("2006-01-02T15:04:05Z07:00"), e.Username, e.Action, e.Outcome, e.TargetName})
		}
		out.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	for _, e := range events {
		_ = enc.Encode(e)
	}
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type SessionsAPI struct {
	Store *auth.Store
	Audit audit.Emitter
	Auth  *auth.Authenticator
}

func (a *SessionsAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", a.list)
	r.Delete("/{id}", a.revoke)
	return r
}
func (a *SessionsAPI) list(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	v, err := a.Store.ListSessions(r.Context())
	if err != nil {
		problem(w, http.StatusInternalServerError, "sessions unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": v})
}
func (a *SessionsAPI) revoke(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, http.StatusServiceUnavailable, "sessions unavailable")
		return
	}
	if err := a.Store.RevokeSessionHash(r.Context(), chi.URLParam(r, "id")); err != nil {
		problem(w, http.StatusInternalServerError, "session revoke failed")
		return
	}
	identity, _ := a.Auth.IdentityFromRequest(r)
	if a.Audit != nil {
		a.Audit.Emit(r.Context(), audit.Event{Action: "session.revoke", Username: identity.Username, Role: identity.Role, Success: true, Reason: "session " + chi.URLParam(r, "id")})
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}
