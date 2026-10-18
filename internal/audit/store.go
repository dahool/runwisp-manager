// SPDX-License-Identifier: GPL-3.0-or-later
package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const auditSchema = `
CREATE TABLE IF NOT EXISTS audit_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL, request_id TEXT NOT NULL,
 subject TEXT NOT NULL, actor_username TEXT NOT NULL, actor_email TEXT NOT NULL, role TEXT NOT NULL,
 ip TEXT NOT NULL, user_agent TEXT NOT NULL, action TEXT NOT NULL, target_kind TEXT NOT NULL,
 target_name TEXT NOT NULL, target_file TEXT NOT NULL, outcome TEXT NOT NULL, details_json BLOB NOT NULL,
 cfg_before TEXT NOT NULL, cfg_after TEXT NOT NULL, git_sha TEXT NOT NULL, prev_hash TEXT NOT NULL, hash TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS audit_events_ts ON audit_events(ts);
CREATE INDEX IF NOT EXISTS audit_events_actor_ts ON audit_events(actor_username, ts);
CREATE INDEX IF NOT EXISTS audit_events_action_ts ON audit_events(action, ts);
CREATE INDEX IF NOT EXISTS audit_events_target_ts ON audit_events(target_kind, target_name, ts);`

type Store struct {
	db        *sql.DB
	queue     chan queued
	done      chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	lastHash  string
	dropped   uint64
	logger    func(Event)
	closeDB   bool
}

type queued struct {
	ctx    context.Context
	event  Event
	result chan error
}

type Filter struct {
	From, To                       time.Time
	Actor, Action, Target, Outcome string
	Limit                          int
}

func (s *Store) List(ctx context.Context, filter Filter) ([]Event, error) {
	if filter.Limit <= 0 || filter.Limit > 10000 {
		filter.Limit = 100
	}
	query := `SELECT id,ts,request_id,subject,actor_username,actor_email,role,ip,user_agent,action,target_kind,target_name,target_file,outcome,details_json,cfg_before,cfg_after,git_sha,prev_hash,hash FROM audit_events WHERE 1=1`
	args := []any{}
	if !filter.From.IsZero() {
		query += " AND ts >= ?"
		args = append(args, filter.From.UnixNano())
	}
	if !filter.To.IsZero() {
		query += " AND ts < ?"
		args = append(args, filter.To.UnixNano())
	}
	if filter.Actor != "" {
		query += " AND actor_username = ?"
		args = append(args, filter.Actor)
	}
	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.Target != "" {
		query += " AND (target_name = ? OR target_file = ?)"
		args = append(args, filter.Target, filter.Target)
	}
	if filter.Outcome != "" {
		query += " AND outcome = ?"
		args = append(args, filter.Outcome)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, filter.Limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var ts int64
		if err := rows.Scan(&e.ID, &ts, &e.RequestID, &e.Subject, &e.Username, &e.Email, &e.Role, &e.RemoteIP, &e.UserAgent, &e.Action, &e.TargetKind, &e.TargetName, &e.TargetFile, &e.Outcome, &e.Details, &e.CfgBefore, &e.CfgAfter, &e.GitSHA, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		e.Time = time.Unix(0, ts).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// OpenStore creates the audit database under stateDir with restrictive permissions.
func OpenStore(ctx context.Context, stateDir string) (*Store, error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create audit state directory: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(stateDir, "audit.db")+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open audit database: %w", err)
	}
	s, err := openDB(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.closeDB = true
	return s, nil
}

// OpenStoreDB attaches audit tables and its writer to an existing SQLite DB,
// allowing sessions and audit history to share one WAL and backup boundary.
func OpenStoreDB(ctx context.Context, db *sql.DB) (*Store, error) { return openDB(ctx, db) }

func openDB(ctx context.Context, db *sql.DB) (*Store, error) {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if _, err := db.ExecContext(ctx, auditSchema); err != nil {
		return nil, fmt.Errorf("create audit schema: %w", err)
	}
	s := &Store{db: db, queue: make(chan queued, 4096), done: make(chan struct{}), closed: make(chan struct{})}
	_ = db.QueryRowContext(ctx, `SELECT hash FROM audit_events ORDER BY id DESC LIMIT 1`).Scan(&s.lastHash)
	go s.writer()
	return s, nil
}

func (s *Store) SetMirror(fn func(Event)) { s.logger = fn }
func (s *Store) DB() *sql.DB              { return s.db }
func (s *Store) Dropped() uint64          { s.mu.Lock(); defer s.mu.Unlock(); return s.dropped }
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		<-s.closed
		if s.closeDB {
			_ = s.db.Close()
		}
	})
	return nil
}

func (s *Store) Emit(ctx context.Context, event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	critical := isCritical(event.Action)
	q := queued{ctx: ctx, event: event}
	if critical {
		q.result = make(chan error, 1)
	}
	select {
	case s.queue <- q:
	case <-ctx.Done():
		return
	default:
		if !critical {
			s.mu.Lock()
			s.dropped++
			s.mu.Unlock()
			return
		}
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case s.queue <- q:
		case <-timer.C:
			return
		case <-ctx.Done():
			return
		}
	}
	if q.result != nil {
		<-q.result
	}
}

func (s *Store) writer() {
	defer close(s.closed)
	for {
		select {
		case <-s.done:
			return
		case q := <-s.queue:
			err := s.write(q.ctx, q.event)
			if q.result != nil {
				q.result <- err
			}
			if s.logger != nil {
				s.logger(q.event)
			}
		}
	}
}

func (s *Store) write(ctx context.Context, event Event) error {
	details := Redact(event.Details)
	if len(details) == 0 {
		details = []byte("{}")
	}
	outcome := event.Outcome
	if outcome == "" {
		if event.Success {
			outcome = "ok"
		} else {
			outcome = "error"
		}
	}
	s.mu.Lock()
	prev := s.lastHash
	s.mu.Unlock()
	event.PrevHash = prev
	event.Outcome = outcome
	event.Details = details
	canonical, _ := json.Marshal(struct {
		TS                                                                                                            int64 `json:"ts"`
		RequestID, Subject, Username, Email, Role, IP, UserAgent, Action, TargetKind, TargetName, TargetFile, Outcome string
		Details                                                                                                       json.RawMessage
		Before, After, GitSHA, Prev                                                                                   string
	}{event.Time.UnixNano(), event.RequestID, event.Subject, event.Username, event.Email, event.Role, event.RemoteIP, event.UserAgent, event.Action, event.TargetKind, event.TargetName, event.TargetFile, outcome, details, event.CfgBefore, event.CfgAfter, event.GitSHA, prev})
	sum := sha256.Sum256(append([]byte(prev), canonical...))
	hash := hex.EncodeToString(sum[:])
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_events(ts,request_id,subject,actor_username,actor_email,role,ip,user_agent,action,target_kind,target_name,target_file,outcome,details_json,cfg_before,cfg_after,git_sha,prev_hash,hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, event.Time.UnixNano(), event.RequestID, event.Subject, event.Username, event.Email, event.Role, event.RemoteIP, truncate(event.UserAgent, 200), event.Action, event.TargetKind, event.TargetName, event.TargetFile, outcome, details, event.CfgBefore, event.CfgAfter, event.GitSHA, prev, hash)
	if err == nil {
		s.mu.Lock()
		s.lastHash = hash
		s.mu.Unlock()
	}
	return err
}

func (s *Store) Verify(ctx context.Context) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,ts,request_id,subject,actor_username,actor_email,role,ip,user_agent,action,target_kind,target_name,target_file,outcome,details_json,cfg_before,cfg_after,git_sha,prev_hash,hash FROM audit_events ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	prev := ""
	for rows.Next() {
		var e Event
		var ts int64
		var details []byte
		if err := rows.Scan(&e.ID, &ts, &e.RequestID, &e.Subject, &e.Username, &e.Email, &e.Role, &e.RemoteIP, &e.UserAgent, &e.Action, &e.TargetKind, &e.TargetName, &e.TargetFile, &e.Outcome, &details, &e.CfgBefore, &e.CfgAfter, &e.GitSHA, &e.PrevHash, &e.Hash); err != nil {
			return 0, err
		}
		e.Time = time.Unix(0, ts).UTC()
		canonical, _ := json.Marshal(struct {
			TS                                                                                                            int64 `json:"ts"`
			RequestID, Subject, Username, Email, Role, IP, UserAgent, Action, TargetKind, TargetName, TargetFile, Outcome string
			Details                                                                                                       json.RawMessage
			Before, After, GitSHA, Prev                                                                                   string
		}{ts, e.RequestID, e.Subject, e.Username, e.Email, e.Role, e.RemoteIP, e.UserAgent, e.Action, e.TargetKind, e.TargetName, e.TargetFile, e.Outcome, details, e.CfgBefore, e.CfgAfter, e.GitSHA, e.PrevHash})
		sum := sha256.Sum256(append([]byte(prev), canonical...))
		expected := hex.EncodeToString(sum[:])
		if e.PrevHash != prev || e.Hash != expected {
			return e.ID, fmt.Errorf("audit chain broken at id %d", e.ID)
		}
		prev = e.Hash
	}
	return 0, rows.Err()
}

func (s *Store) Prune(ctx context.Context, before time.Time, batch int) (int64, error) {
	if batch <= 0 {
		batch = 500
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin audit prune: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM audit_events WHERE id IN (SELECT id FROM audit_events WHERE ts < ? ORDER BY id LIMIT ?)`, before.UnixNano(), batch)
	if err != nil {
		return 0, err
	}
	// Re-anchor the retained suffix so verification remains valid after pruning.
	rows, err := tx.QueryContext(ctx, `SELECT id,ts,request_id,subject,actor_username,actor_email,role,ip,user_agent,action,target_kind,target_name,target_file,outcome,details_json,cfg_before,cfg_after,git_sha FROM audit_events ORDER BY id`)
	if err != nil {
		return 0, err
	}
	type row struct {
		e       Event
		ts      int64
		details []byte
	}
	var retained []row
	for rows.Next() {
		var v row
		if err := rows.Scan(&v.e.ID, &v.ts, &v.e.RequestID, &v.e.Subject, &v.e.Username, &v.e.Email, &v.e.Role, &v.e.RemoteIP, &v.e.UserAgent, &v.e.Action, &v.e.TargetKind, &v.e.TargetName, &v.e.TargetFile, &v.e.Outcome, &v.details, &v.e.CfgBefore, &v.e.CfgAfter, &v.e.GitSHA); err != nil {
			rows.Close()
			return 0, err
		}
		retained = append(retained, v)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	prev := ""
	for _, v := range retained {
		canonical, _ := json.Marshal(struct {
			TS                                                                                                            int64 `json:"ts"`
			RequestID, Subject, Username, Email, Role, IP, UserAgent, Action, TargetKind, TargetName, TargetFile, Outcome string
			Details                                                                                                       json.RawMessage
			Before, After, GitSHA, Prev                                                                                   string
		}{v.ts, v.e.RequestID, v.e.Subject, v.e.Username, v.e.Email, v.e.Role, v.e.RemoteIP, v.e.UserAgent, v.e.Action, v.e.TargetKind, v.e.TargetName, v.e.TargetFile, v.e.Outcome, v.details, v.e.CfgBefore, v.e.CfgAfter, v.e.GitSHA, prev})
		sum := sha256.Sum256(append([]byte(prev), canonical...))
		hash := hex.EncodeToString(sum[:])
		if _, err := tx.ExecContext(ctx, `UPDATE audit_events SET prev_hash=?,hash=? WHERE id=?`, prev, hash, v.e.ID); err != nil {
			return 0, err
		}
		prev = hash
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.lastHash = prev
	s.mu.Unlock()
	n, _ := result.RowsAffected()
	return n, nil
}
func isCritical(action string) bool {
	return strings.HasPrefix(action, "config.") || strings.HasPrefix(action, "git.") || strings.HasPrefix(action, "secret.") || strings.HasPrefix(action, "auth.") || action == "session.revoke" || action == "admin.export"
}
func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		return string(r[:n])
	}
	return v
}

var secretKeys = map[string]bool{"token": true, "secret": true, "password": true, "cookie": true, "authorization": true, "id_token": true, "access_token": true, "refresh_token": true}

func Redact(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return []byte("{}")
	}
	redactValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	if len(out) > 8192 {
		return []byte(`{"redacted":"details too large"}`)
	}
	return out
}
func redactValue(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			kl := strings.ToLower(strings.ReplaceAll(k, "-", "_"))
			if secretKeys[kl] || strings.Contains(kl, "token") || strings.Contains(kl, "password") || strings.Contains(kl, "cookie") {
				x[k] = "[REDACTED]"
			} else {
				redactValue(val)
			}
		}
	case []any:
		for _, val := range x {
			redactValue(val)
		}
	}
}

var _ Emitter = (*Store)(nil)
