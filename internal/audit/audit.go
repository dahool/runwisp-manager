// SPDX-License-Identifier: GPL-3.0-or-later
// Package audit defines the authentication audit boundary.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

type Event struct {
	ID         int64           `json:"id"`
	Time       time.Time       `json:"ts"`
	RequestID  string          `json:"requestId,omitempty"`
	Subject    string          `json:"subject,omitempty"`
	Username   string          `json:"username,omitempty"`
	Email      string          `json:"email,omitempty"`
	Role       string          `json:"role,omitempty"`
	RemoteIP   string          `json:"ip,omitempty"`
	UserAgent  string          `json:"userAgent,omitempty"`
	Action     string          `json:"action"`
	TargetKind string          `json:"targetKind,omitempty"`
	TargetName string          `json:"targetName,omitempty"`
	TargetFile string          `json:"targetFile,omitempty"`
	Outcome    string          `json:"outcome"`
	Success    bool            `json:"-"` // legacy callers; Outcome is used when set
	Reason     string          `json:"reason,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
	CfgBefore  string          `json:"cfgBefore,omitempty"`
	CfgAfter   string          `json:"cfgAfter,omitempty"`
	GitSHA     string          `json:"gitSha,omitempty"`
	PrevHash   string          `json:"prevHash,omitempty"`
	Hash       string          `json:"hash,omitempty"`
}

type Emitter interface{ Emit(context.Context, Event) }

type SlogSink struct{ Logger *slog.Logger }

func (s SlogSink) Emit(_ context.Context, event Event) {
	if s.Logger == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	s.Logger.Info("audit event", "action", event.Action, "subject", event.Subject, "username", event.Username, "role", event.Role, "remote_ip", event.RemoteIP, "success", event.Success, "reason", event.Reason)
}
