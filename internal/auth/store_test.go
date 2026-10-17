package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStoreSessionExpiryAndHashing(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_000, 0)
	store, err := OpenStore(ctx, t.TempDir(), []byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.now = func() time.Time { return now }
	id, err := store.CreateSession(ctx, Session{Sub: "sub", Username: "user", Role: "viewer", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), Groups: []string{"g"}, IDTokenHint: "token"})
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := store.db.QueryRow(`SELECT COALESCE(CAST(id_hash AS TEXT), '') FROM sessions`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw == id || strings.Contains(raw, id) {
		t.Fatal("raw session id was persisted")
	}
	got, err := store.GetSession(ctx, id, time.Hour)
	if err != nil || got.IDTokenHint != "token" {
		t.Fatalf("session = %#v, err = %v", got, err)
	}
	now = now.Add(time.Hour + time.Second)
	if _, err := store.GetSession(ctx, id, time.Hour); err == nil {
		t.Fatal("expected absolute expiry")
	}
}

func TestConsumeFlowIsSingleUse(t *testing.T) {
	store, err := OpenStore(context.Background(), t.TempDir(), []byte(strings.Repeat("f", 32)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Unix(2_000, 0)
	store.now = func() time.Time { return now }
	flow := Flow{State: "state", Nonce: "nonce", CodeVerifier: "verifier", ReturnTo: "/", ExpiresAt: now.Add(time.Minute)}
	if err := store.CreateFlow(context.Background(), flow); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeFlow(context.Background(), "state"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeFlow(context.Background(), "state"); err == nil {
		t.Fatal("expected replay failure")
	}
}
