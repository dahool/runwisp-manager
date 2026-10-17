package cfgsave

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/runwisp/runwisp-manager/internal/audit"
)

type auditRecorder struct{ events []audit.Event }

func (r *auditRecorder) Emit(_ context.Context, event audit.Event) {
	r.events = append(r.events, event)
}

func TestPipelineSaveTransactionalContracts(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		conflict bool
		wantErr  error
	}{
		{name: "disabled", enabled: false, wantErr: ErrEditDisabled},
		{name: "conflict", enabled: true, conflict: true, wantErr: ErrConflict},
		{name: "successful save", enabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			before := []byte("name = \"old\"\n")
			if err := os.WriteFile(path, before, 0o640); err != nil {
				t.Fatal(err)
			}
			recorder := &auditRecorder{}
			pipeline := New(Options{Root: dir, StateDir: filepath.Join(dir, "state"), EditEnabled: tt.enabled, Resolve: func(id FileID) (string, error) { return path, nil }, Audit: recorder, Actor: audit.Event{Role: "admin"}})
			base := hash(before)
			if tt.conflict {
				base = hash([]byte("different"))
			}
			result, err := pipeline.Save(context.Background(), Change{IdempotencyKey: "key", Base: map[FileID]string{"config": base}, Raw: []FileContent{{File: "config", Content: []byte("name = \"new\"\n")}}})
			if !errors.Is(err, tt.wantErr) {
				if tt.wantErr != nil || err != nil {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
			}
			if tt.wantErr == nil {
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(data) != "name = \"new\"\n" || !result.Saved {
					t.Fatalf("save result = %#v, data = %q", result, data)
				}
			} else if data, readErr := os.ReadFile(path); readErr != nil || string(data) != string(before) {
				t.Fatalf("failed save changed file: %q, %v", data, readErr)
			}
			if len(recorder.events) != 1 {
				t.Fatalf("audit events = %d, want 1", len(recorder.events))
			}
		})
	}
}

func TestPipelineIdempotencyReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	before := []byte("name = \"old\"\n")
	if err := os.WriteFile(path, before, 0o640); err != nil {
		t.Fatal(err)
	}
	pipeline := New(Options{StateDir: filepath.Join(dir, "state"), EditEnabled: true, Resolve: func(FileID) (string, error) { return path, nil }, Actor: audit.Event{Role: "admin"}})
	change := Change{IdempotencyKey: "same", Base: map[FileID]string{"config": hash(before)}, Raw: []FileContent{{File: "config", Content: []byte("name = \"new\"\n")}}}
	first, err := pipeline.Save(context.Background(), change)
	if err != nil {
		t.Fatal(err)
	}
	second, err := pipeline.Save(context.Background(), change)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replay = %#v, first = %#v", second, first)
	}
}

func TestRecoverRestoresPartialJournal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	old := []byte("name = \"old\"\n")
	newValue := []byte("name = \"new\"\n")
	if err := os.WriteFile(path, newValue, 0o640); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup.toml")
	if err := os.WriteFile(backup, old, 0o600); err != nil {
		t.Fatal(err)
	}
	j := Journal{ID: "crash", Phase: "written", Files: []JournalFile{{File: "config", Path: path, BeforeHash: hash(old), AfterHash: hash(newValue), BackupPath: backup}}}
	if err := writeJournal(dir, j); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(newValue) {
		t.Fatalf("fully written journal was unexpectedly rolled back: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "journal", "crash.json")); !os.IsNotExist(err) {
		t.Fatalf("journal remains: %v", err)
	}
	// A partial target is restored from every backup.
	if err := os.WriteFile(path, []byte("partial"), 0o640); err != nil {
		t.Fatal(err)
	}
	j.ID = "partial"
	if err := writeJournal(dir, j); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(dir); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(old) {
		t.Fatalf("partial journal was not restored: %q", data)
	}
}
