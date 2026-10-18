package cfgsave

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/daemon"
	"github.com/runwisp/runwisp-manager/internal/validate"
)

type Options struct {
	Root        string
	StateDir    string
	EditEnabled bool
	BackupKeep  int
	LockTimeout time.Duration
	Resolve     Resolver
	Validator   Validator
	Daemon      daemon.Client
	Audit       audit.Emitter
	Authorize   Authorizer
	Actor       audit.Event
}

type Pipeline struct {
	opts  Options
	lock  *Lock
	cache *idempotencyCache
}

type idempotencyCache struct {
	mu      sync.Mutex
	results map[string]cachedResult
}
type cachedResult struct {
	result Result
	at     time.Time
}

const idempotencyCacheTTL = 24 * time.Hour
const idempotencyCacheLimit = 1024

// WithSync serializes a repository operation with configuration saves. Git
// sync callers use the same process and file lock as saves, so neither side
// can mutate the managed tree concurrently.
func (p *Pipeline) WithSync(ctx context.Context, fn func() error) error {
	if p == nil || fn == nil {
		return errors.New("sync operation is unavailable")
	}
	unlock, err := p.lock.Acquire(ctx, p.opts.StateDir, p.opts.LockTimeout)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func (p *Pipeline) cacheKey(key string) string {
	return p.opts.Actor.Subject + "\x00" + p.opts.Actor.Username + "\x00" + key
}
func (p *Pipeline) cachedResult(key string) (Result, bool) {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()
	cacheKey := p.cacheKey(key)
	for k, item := range p.cache.results {
		if time.Since(item.at) > idempotencyCacheTTL {
			delete(p.cache.results, k)
		}
	}
	item, ok := p.cache.results[cacheKey]
	return item.result, ok
}

func (p *Pipeline) Daemon() daemon.Client { return p.opts.Daemon }

func (p *Pipeline) Backups(file string) ([]Backup, error) {
	if p.opts.Resolve == nil {
		return nil, ErrManagedPath
	}
	if _, err := p.opts.Resolve(FileID(file)); err != nil {
		return nil, ErrManagedPath
	}
	return ListBackups(p.opts.StateDir, FileID(file))
}

// Reload exposes the standalone editor reload endpoint.
func (p *Pipeline) Reload(ctx context.Context) (Result, error) {
	if p.opts.Daemon == nil {
		return Result{}, errors.New("daemon reload unavailable")
	}
	reload, err := p.opts.Daemon.Reload(ctx)
	if err != nil {
		return Result{}, err
	}
	if !reload.OK {
		return Result{}, fmt.Errorf("daemon reload rejected: %s", reload.Message)
	}
	return Result{Applied: true}, nil
}

type ReloadSummary struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Changed []string `json:"changed,omitempty"`
}

type Result struct {
	Saved       bool            `json:"saved"`
	Applied     bool            `json:"applied"`
	Stale       bool            `json:"stale"`
	TreeVersion string          `json:"treeVersion,omitempty"`
	Reload      ReloadSummary   `json:"reload,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	Diff        []PreviewFile   `json:"diff,omitempty"`
	Validation  validate.Result `json:"validation,omitempty"`
}

func New(opts Options) *Pipeline {
	if opts.BackupKeep <= 0 {
		opts.BackupKeep = 20
	}
	return &Pipeline{opts: opts, lock: &Lock{}, cache: &idempotencyCache{results: make(map[string]cachedResult)}}
}

// WithActor returns a pipeline sharing configuration but using the supplied
// identity for authorization and audit attribution. The shared cache is scoped
// by actor so derived pipelines retain request deduplication without sharing results.
func (p *Pipeline) WithActor(actor audit.Event) *Pipeline {
	copy := &Pipeline{opts: p.opts, lock: p.lock, cache: p.cache}
	copy.opts.Actor = actor
	return copy
}

func (p *Pipeline) Preview(ctx context.Context, change Change) (Preview, error) {
	if err := p.authorize(ctx, change); err != nil {
		return Preview{}, err
	}
	if p.opts.Resolve == nil {
		return Preview{}, errors.New("configuration resolver is unavailable")
	}
	files, err := currentFiles(change, p.opts.Resolve)
	if err != nil {
		return Preview{}, err
	}
	if conflicts := CheckConflicts(change, files); len(conflicts) > 0 {
		return Preview{Conflicts: conflicts}, fmt.Errorf("%w: %d file(s)", ErrConflict, len(conflicts))
	}
	candidates, validation, err := Compute(ctx, change, p.opts.Resolve, p.opts.Root, p.opts.Validator)
	if err != nil {
		return Preview{}, err
	}
	preview := makePreview(candidates, validation)
	if !validation.Valid {
		return preview, fmt.Errorf("validation failed")
	}
	return preview, nil
}

func (p *Pipeline) Save(ctx context.Context, change Change) (Result, error) {
	result, err := p.save(ctx, change)
	p.emitAudit(ctx, change, result, err)
	return result, err
}

func (p *Pipeline) save(ctx context.Context, change Change) (Result, error) {
	if change.IdempotencyKey != "" {
		item, ok := p.cachedResult(change.IdempotencyKey)
		if ok {
			return item, nil
		}
	}
	if err := p.authorize(ctx, change); err != nil {
		return Result{}, err
	}
	if p.opts.Resolve == nil {
		return Result{}, errors.New("configuration resolver is unavailable")
	}
	unlock, err := p.lock.Acquire(ctx, p.opts.StateDir, p.opts.LockTimeout)
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if change.IdempotencyKey != "" {
		if previous, ok := p.cachedResult(change.IdempotencyKey); ok {
			return previous, nil
		}
	}
	files, err := currentFiles(change, p.opts.Resolve)
	if err != nil {
		return Result{}, err
	}
	if conflicts := CheckConflicts(change, files); len(conflicts) > 0 {
		return Result{}, fmt.Errorf("%w: %d file(s)", ErrConflict, len(conflicts))
	}
	candidates, validation, err := Compute(ctx, change, p.opts.Resolve, p.opts.Root, p.opts.Validator)
	if err != nil {
		return Result{}, err
	}
	if !validation.Valid {
		return Result{Validation: validation}, fmt.Errorf("validation failed")
	}
	journal := newJournal(candidates)
	if err := writeJournal(p.opts.StateDir, journal); err != nil {
		return Result{}, err
	}
	if err := backupCandidates(p.opts.StateDir, &journal, candidates, p.opts.BackupKeep); err != nil {
		_ = removeJournal(p.opts.StateDir, journal.ID)
		return Result{}, err
	}
	for _, candidate := range candidates {
		if err := atomicWrite(candidate.Path, candidate.After); err != nil {
			_ = rollback(journal)
			_ = removeJournal(p.opts.StateDir, journal.ID)
			return Result{}, fmt.Errorf("write configuration: %w", err)
		}
	}
	journal.Phase = "written"
	if err := writeJournal(p.opts.StateDir, journal); err != nil {
		_ = rollback(journal)
		_ = removeJournal(p.opts.StateDir, journal.ID)
		return Result{}, err
	}
	for _, candidate := range candidates {
		data, readErr := os.ReadFile(candidate.Path)
		if readErr != nil || hash(data) != hash(candidate.After) {
			_ = rollback(journal)
			_ = removeJournal(p.opts.StateDir, journal.ID)
			return Result{}, fmt.Errorf("%w: external writer changed %s", ErrConflict, candidate.File)
		}
	}
	result := Result{Saved: true, Stale: !change.Apply, Validation: validation}
	if change.Apply && p.opts.Daemon != nil {
		reload, reloadErr := p.opts.Daemon.Reload(ctx)
		if reloadErr != nil {
			if strings.Contains(reloadErr.Error(), "400") || strings.Contains(strings.ToLower(reloadErr.Error()), "reload rejected") {
				_ = rollback(journal)
				_ = removeJournal(p.opts.StateDir, journal.ID)
				return Result{Validation: validation}, fmt.Errorf("daemon reload rejected: %w", reloadErr)
			}
			result.Warnings = append(result.Warnings, "configuration saved but daemon reload could not be reached")
		} else if reload.OK {
			result.Applied = true
			result.Stale = false
		} else {
			_ = rollback(journal)
			_ = removeJournal(p.opts.StateDir, journal.ID)
			return Result{Validation: validation}, fmt.Errorf("daemon reload rejected: %s", reload.Message)
		}
	} else if change.Apply {
		result.Warnings = append(result.Warnings, "configuration saved but daemon reload is unavailable")
	}
	journal.Phase = "applied"
	_ = writeJournal(p.opts.StateDir, journal)
	_ = removeJournal(p.opts.StateDir, journal.ID)
	if change.IdempotencyKey != "" {
		key := p.cacheKey(change.IdempotencyKey)
		p.cache.mu.Lock()
		if len(p.cache.results) >= idempotencyCacheLimit {
			var oldest string
			var at time.Time
			for k, item := range p.cache.results {
				if oldest == "" || item.at.Before(at) {
					oldest, at = k, item.at
				}
			}
			delete(p.cache.results, oldest)
		}
		p.cache.results[key] = cachedResult{result: result, at: time.Now()}
		p.cache.mu.Unlock()
	}
	return result, nil
}

func currentFiles(change Change, resolve Resolver) (map[FileID][]byte, error) {
	files := make(map[FileID][]byte, len(change.Base))
	for id := range change.Base {
		path, err := resolve(id)
		if err != nil {
			return nil, err
		}
		data, err := readFile(path)
		if err != nil {
			return nil, err
		}
		files[id] = data
	}
	return files, nil
}

func (p *Pipeline) authorize(ctx context.Context, change Change) error {
	if !p.opts.EditEnabled {
		return ErrEditDisabled
	}
	if len(change.Raw) > 0 && p.opts.Actor.Role != "admin" && !change.Rollback {
		return ErrUnauthorized
	}
	if p.opts.Authorize != nil {
		_, err := p.opts.Authorize(ctx, change)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
	}
	return nil
}

func (p *Pipeline) emitAudit(ctx context.Context, change Change, result Result, err error) {
	if p.opts.Audit == nil {
		return
	}
	event := p.opts.Actor
	event.Action = "config.save"
	event.Success = err == nil && result.Saved
	event.Reason = change.Reason
	p.opts.Audit.Emit(ctx, event)
}
