// SPDX-License-Identifier: GPL-3.0-or-later
// Package audit defines the authentication audit boundary.
package audit

import (
	"context"
	"log/slog"
	"time"
)

type Event struct {
	Time     time.Time
	Action   string
	Subject  string
	Username string
	Role     string
	RemoteIP string
	Success  bool
	Reason   string
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
