// SPDX-License-Identifier: GPL-3.0-or-later
package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestStore_RedactsAndVerifiesChain(t *testing.T) {
	t.Parallel()
	s, err := OpenStore(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Emit(context.Background(), Event{Action: "config.save", Username: "alice", Details: json.RawMessage(`{"password":"dont-store","nested":{"access_token":"also-secret"},"visible":"ok"}`), Success: true})
	s.Emit(context.Background(), Event{Action: "notification.read", Username: "alice", Details: json.RawMessage(`{"visible":"fine"}`), Success: true})
	if id, err := s.Verify(context.Background()); err != nil || id != 0 {
		t.Fatalf("verify = %d, %v", id, err)
	}
	var details string
	if err := s.DB().QueryRow(`SELECT details_json FROM audit_events WHERE id=1`).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if details == "" || contains(details, "dont-store") || contains(details, "also-secret") {
		t.Fatalf("secret reached database: %s", details)
	}
	if _, err := s.DB().Exec(`UPDATE audit_events SET action='tampered' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if id, err := s.Verify(context.Background()); id != 1 || err == nil {
		t.Fatalf("tamper verify = %d, %v", id, err)
	}
}

func TestRedact_InvalidAndOversizedDetails(t *testing.T) {
	t.Parallel()
	if got := string(Redact([]byte("not-json"))); got != "{}" {
		t.Fatalf("invalid redaction = %q", got)
	}
	if got := Redact([]byte(`{"token":"secret"}`)); contains(string(got), "secret") {
		t.Fatal("token was not redacted")
	}
	if got := Redact([]byte(`{"value":"` + string(make([]byte, 9000)) + `"}`)); len(got) > 128 {
		t.Fatalf("oversized details was not capped: %d", len(got))
	}
}

func contains(value, needle string) bool { return strings.Contains(value, needle) }
