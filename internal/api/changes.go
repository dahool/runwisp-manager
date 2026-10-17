package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/cfgsave"
	"github.com/runwisp/runwisp-manager/internal/tomledit"
)

type changeRequest struct {
	IdempotencyKey string                    `json:"idempotencyKey"`
	Base           map[cfgsave.FileID]string `json:"base"`
	Ops            []struct {
		File cfgsave.FileID    `json:"file"`
		Ops  []json.RawMessage `json:"ops"`
	} `json:"ops,omitempty"`
	Raw []struct {
		File    cfgsave.FileID `json:"file"`
		Content string         `json:"content"`
	} `json:"raw,omitempty"`
	Apply  bool   `json:"apply"`
	Reason string `json:"reason,omitempty"`
}

func (a *ConfigAPI) changesPreview(w http.ResponseWriter, r *http.Request) {
	if !a.requireEditor(w, r) {
		return
	}
	change, err := decodeChange(r)
	if err != nil {
		problem(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.Save == nil {
		problem(w, http.StatusServiceUnavailable, "save pipeline unavailable")
		return
	}
	preview, err := a.Save.WithActor(actorFromRequest(a, r)).Preview(r.Context(), change)
	if err != nil {
		writeChangeError(w, err, preview)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *ConfigAPI) changes(w http.ResponseWriter, r *http.Request) {
	if !a.requireEditor(w, r) {
		return
	}
	change, err := decodeChange(r)
	if err != nil {
		problem(w, http.StatusBadRequest, err.Error())
		return
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		change.IdempotencyKey = key
	}
	if a.Save == nil {
		problem(w, http.StatusServiceUnavailable, "save pipeline unavailable")
		return
	}
	result, err := a.Save.WithActor(actorFromRequest(a, r)).Save(r.Context(), change)
	if err != nil {
		writeChangeError(w, err, result)
		return
	}
	status := http.StatusOK
	if len(result.Warnings) > 0 && result.Saved && !result.Applied {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (a *ConfigAPI) reload(w http.ResponseWriter, r *http.Request) {
	if !a.requireEditor(w, r) {
		return
	}
	if a.Save == nil || a.Save.Daemon() == nil {
		problem(w, http.StatusServiceUnavailable, "daemon reload unavailable")
		return
	}
	result, err := a.Save.Reload(r.Context())
	if err != nil {
		writeChangeError(w, err, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *ConfigAPI) backups(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "admin" {
		problem(w, http.StatusForbidden, "backup listing requires admin")
		return
	}
	if a.Save == nil {
		problem(w, http.StatusServiceUnavailable, "save pipeline unavailable")
		return
	}
	backups, err := a.Save.Backups(r.URL.Query().Get("file"))
	if err != nil {
		problem(w, http.StatusInternalServerError, "list backups failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (a *ConfigAPI) requireEditor(w http.ResponseWriter, r *http.Request) bool {
	identity, ok := a.Auth.IdentityFromRequest(r)
	if !ok || identity.Role != "editor" && identity.Role != "admin" {
		problem(w, http.StatusForbidden, "editing requires editor role")
		return false
	}
	return true
}

func decodeChange(r *http.Request) (cfgsave.Change, error) {
	var request changeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&request); err != nil {
		return cfgsave.Change{}, errors.New("invalid change request")
	}
	change := cfgsave.Change{IdempotencyKey: request.IdempotencyKey, Base: request.Base, Apply: request.Apply, Reason: request.Reason}
	for _, group := range request.Ops {
		fileOps := cfgsave.FileOps{File: group.File}
		for _, raw := range group.Ops {
			op, err := decodeOp(raw)
			if err != nil {
				return cfgsave.Change{}, err
			}
			fileOps.Ops = append(fileOps.Ops, op)
		}
		change.Ops = append(change.Ops, fileOps)
	}
	for _, raw := range request.Raw {
		change.Raw = append(change.Raw, cfgsave.FileContent{File: raw.File, Content: []byte(raw.Content)})
	}
	return change, nil
}

func decodeOp(raw json.RawMessage) (tomledit.Op, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("invalid edit operation")
	}
	var op tomledit.Op
	switch envelope.Type {
	case "setValue":
		var v struct {
			Path  []string `json:"path"`
			Value any      `json:"value"`
		}
		op = tomledit.SetValue{Path: v.Path, Value: v.Value}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid setValue operation")
		}
		op = tomledit.SetValue{Path: v.Path, Value: v.Value}
	case "addKey":
		var v tomledit.AddKey
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid addKey operation")
		}
		op = v
	case "deleteKey":
		var v tomledit.DeleteKey
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid deleteKey operation")
		}
		op = v
	case "addTable":
		var v tomledit.AddTable
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid addTable operation")
		}
		op = v
	case "deleteTable":
		var v tomledit.DeleteTable
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid deleteTable operation")
		}
		op = v
	case "renameTable":
		var v tomledit.RenameTable
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid renameTable operation")
		}
		op = v
	case "reorderArrayTable":
		var v tomledit.ReorderArrayTable
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, errors.New("invalid reorderArrayTable operation")
		}
		op = v
	default:
		return nil, errors.New("unknown edit operation")
	}
	return op, nil
}

func writeChangeError(w http.ResponseWriter, err error, value any) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, cfgsave.ErrEditDisabled), errors.Is(err, cfgsave.ErrUnauthorized), errors.Is(err, cfgsave.ErrManagedPath):
		status = http.StatusForbidden
	case errors.Is(err, cfgsave.ErrBusy), errors.Is(err, cfgsave.ErrConflict):
		status = http.StatusConflict
	case strings.Contains(err.Error(), "validation failed"), strings.Contains(err.Error(), "reload rejected"):
		status = http.StatusUnprocessableEntity
	}
	if status == http.StatusConflict && !errors.Is(err, cfgsave.ErrConflict) {
		value = map[string]any{"detail": err.Error()}
	}
	if value == nil {
		value = map[string]any{"detail": err.Error()}
	}
	writeJSON(w, status, value)
}

func actorFromRequest(a *ConfigAPI, r *http.Request) (event audit.Event) {
	identity, _ := a.Auth.IdentityFromRequest(r)
	return audit.Event{Subject: identity.Sub, Username: identity.Username, Role: identity.Role}
}
