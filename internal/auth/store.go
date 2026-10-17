// SPDX-License-Identifier: GPL-3.0-or-later
// Package auth implements OIDC authentication, sessions, and authorization.
package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
 id_hash BLOB PRIMARY KEY, sub TEXT, username TEXT, name TEXT, email TEXT,
 role TEXT NOT NULL, groups_json TEXT NOT NULL, created_at INTEGER NOT NULL,
 last_seen_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, ip TEXT,
 user_agent TEXT, id_token_hint BLOB
);
CREATE TABLE IF NOT EXISTS oidc_flows (
 state TEXT PRIMARY KEY, nonce TEXT NOT NULL, code_verifier TEXT NOT NULL,
 return_to TEXT NOT NULL, expires_at INTEGER NOT NULL
);`

type Session struct {
	ID          string
	Sub         string
	Username    string
	Name        string
	Email       string
	Role        string
	Groups      []string
	CreatedAt   time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
	IP          string
	UserAgent   string
	IDTokenHint string
}

type Flow struct {
	State, Nonce, CodeVerifier, ReturnTo string
	ExpiresAt                            time.Time
}

type Store struct {
	db     *sql.DB
	secret []byte
	now    func() time.Time
}

func OpenStore(ctx context.Context, stateDir string, secret []byte) (*Store, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must contain at least 32 bytes")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create auth state directory: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(stateDir, "auth.db")+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open auth database: %w", err)
	}
	store := &Store{db: db, secret: append([]byte(nil), secret...), now: time.Now}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create auth schema: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateFlow(ctx context.Context, flow Flow) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO oidc_flows(state,nonce,code_verifier,return_to,expires_at) VALUES(?,?,?,?,?)`, flow.State, flow.Nonce, flow.CodeVerifier, flow.ReturnTo, flow.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("create oidc flow: %w", err)
	}
	return nil
}

// ConsumeFlow atomically reads and deletes a flow, making callback replay fail.
func (s *Store) ConsumeFlow(ctx context.Context, state string) (Flow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Flow{}, fmt.Errorf("begin flow transaction: %w", err)
	}
	defer tx.Rollback()
	var flow Flow
	var expiry int64
	err = tx.QueryRowContext(ctx, `SELECT nonce,code_verifier,return_to,expires_at FROM oidc_flows WHERE state=?`, state).Scan(&flow.Nonce, &flow.CodeVerifier, &flow.ReturnTo, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return Flow{}, errors.New("oidc flow not found")
	}
	if err != nil {
		return Flow{}, fmt.Errorf("load oidc flow: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_flows WHERE state=?`, state); err != nil {
		return Flow{}, fmt.Errorf("consume oidc flow: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Flow{}, fmt.Errorf("commit oidc flow: %w", err)
	}
	flow.State, flow.ExpiresAt = state, time.Unix(expiry, 0)
	if !s.now().Before(flow.ExpiresAt) {
		return Flow{}, errors.New("oidc flow expired")
	}
	return flow, nil
}

func (s *Store) CreateSession(ctx context.Context, session Session) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	session.ID = base64.RawURLEncoding.EncodeToString(raw[:])
	groups, err := json.Marshal(session.Groups)
	if err != nil {
		return "", fmt.Errorf("encode session groups: %w", err)
	}
	hint, err := s.encrypt(session.IDTokenHint)
	if err != nil {
		return "", fmt.Errorf("encrypt id token hint: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(id_hash,sub,username,name,email,role,groups_json,created_at,last_seen_at,expires_at,ip,user_agent,id_token_hint) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.hash(session.ID), session.Sub, session.Username, session.Name, session.Email, session.Role, groups, session.CreatedAt.Unix(), session.LastSeenAt.Unix(), session.ExpiresAt.Unix(), session.IP, session.UserAgent, hint)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return session.ID, nil
}

func (s *Store) GetSession(ctx context.Context, id string, idle time.Duration) (Session, error) {
	var session Session
	var groups []byte
	var created, lastSeen, expires int64
	var hint []byte
	err := s.db.QueryRowContext(ctx, `SELECT sub,username,name,email,role,groups_json,created_at,last_seen_at,expires_at,ip,user_agent,id_token_hint FROM sessions WHERE id_hash=?`, s.hash(id)).Scan(&session.Sub, &session.Username, &session.Name, &session.Email, &session.Role, &groups, &created, &lastSeen, &expires, &session.IP, &session.UserAgent, &hint)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, errors.New("session not found")
	}
	if err != nil {
		return Session{}, fmt.Errorf("load session: %w", err)
	}
	if err := json.Unmarshal(groups, &session.Groups); err != nil {
		return Session{}, fmt.Errorf("decode session groups: %w", err)
	}
	session.CreatedAt, session.LastSeenAt, session.ExpiresAt = time.Unix(created, 0), time.Unix(lastSeen, 0), time.Unix(expires, 0)
	if !s.now().Before(session.ExpiresAt) || !s.now().Before(session.LastSeenAt.Add(idle)) {
		_ = s.DeleteSession(ctx, id)
		return Session{}, errors.New("session expired")
	}
	if len(hint) > 0 {
		if session.IDTokenHint, err = s.decrypt(hint); err != nil {
			return Session{}, fmt.Errorf("decrypt id token hint: %w", err)
		}
	}
	if s.now().Sub(session.LastSeenAt) >= time.Minute {
		if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at=? WHERE id_hash=?`, s.now().Unix(), s.hash(id)); err != nil {
			return Session{}, fmt.Errorf("touch session: %w", err)
		}
		session.LastSeenAt = s.now()
	}
	session.ID = id
	return session, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash=?`, s.hash(id)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) Janitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
			_, _ = s.db.ExecContext(ctx, `DELETE FROM oidc_flows WHERE expires_at <= ?`, now.Unix())
		}
	}
}

func (s *Store) hash(id string) []byte {
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte(id))
	return h.Sum(nil)
}

func (s *Store) key() ([]byte, error) {
	reader, err := hkdf.Key(sha256.New, s.secret, nil, "runwisp-manager session id token hint", 32)
	if err != nil {
		return nil, fmt.Errorf("derive hint key: %w", err)
	}
	return reader, nil
}

func (s *Store) encrypt(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(value), nil), nil
}

func (s *Store) decrypt(value []byte) (string, error) {
	key, err := s.key()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(value) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted hint")
	}
	plain, err := gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("invalid encrypted hint")
	}
	return string(plain), nil
}
