package cfgsave

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Journal struct {
	ID        string        `json:"id"`
	Phase     string        `json:"phase"`
	CreatedAt time.Time     `json:"createdAt"`
	Files     []JournalFile `json:"files"`
}
type JournalFile struct {
	File       FileID `json:"file"`
	Path       string `json:"path"`
	BeforeHash string `json:"beforeHash"`
	AfterHash  string `json:"afterHash"`
	BackupPath string `json:"backupPath"`
}

func newJournal(candidates []Candidate) Journal {
	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		idBytes = []byte(strconv.FormatInt(time.Now().UnixNano(), 10))
	}
	j := Journal{ID: hex.EncodeToString(idBytes), Phase: "prepared", CreatedAt: time.Now().UTC()}
	for _, c := range candidates {
		j.Files = append(j.Files, JournalFile{File: c.File, Path: c.Path, BeforeHash: hash(c.Before), AfterHash: hash(c.After)})
	}
	return j
}

func writeJournal(stateDir string, journal Journal) error {
	if err := os.MkdirAll(filepath.Join(stateDir, "journal"), 0o700); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode journal: %w", err)
	}
	path := filepath.Join(stateDir, "journal", journal.ID+".json")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".journal-*")
	if err != nil {
		return fmt.Errorf("create journal temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write journal: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync journal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install journal: %w", err)
	}
	return syncDir(filepath.Dir(path))
}

func backupCandidates(stateDir string, journal *Journal, candidates []Candidate, keep int) error {
	if keep <= 0 {
		keep = 20
	}
	for i, c := range candidates {
		dir := filepath.Join(stateDir, "backups", string(c.File))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create backup directory: %w", err)
		}
		name := fmt.Sprintf("%s-%s.toml", time.Now().UTC().Format("20060102T150405.000000000Z"), hash(c.Before)[:8])
		path := filepath.Join(dir, name)
		if err := copyFile(path, c.Before, 0o600); err != nil {
			return fmt.Errorf("backup %s: %w", c.File, err)
		}
		journal.Files[i].BackupPath = path
		if err := pruneBackups(dir, keep); err != nil {
			return err
		}
	}
	return writeJournal(stateDir, *journal)
}

func copyFile(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func pruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for len(files) > keep {
		if err := os.Remove(filepath.Join(dir, files[0])); err != nil {
			return fmt.Errorf("prune backup: %w", err)
		}
		files = files[1:]
	}
	return nil
}

func atomicWrite(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat target: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rwm-save-*")
	if err != nil {
		return fmt.Errorf("create write temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := preserveOwnership(tmpName, info); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

func rollback(journal Journal) error {
	var errs []error
	for _, file := range journal.Files {
		data, err := os.ReadFile(file.BackupPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("read backup %s: %w", file.File, err))
			continue
		}
		if err := atomicWrite(file.Path, data); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", file.File, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("rollback failed: %v", errs)
	}
	return nil
}

func removeJournal(stateDir, id string) error {
	return os.Remove(filepath.Join(stateDir, "journal", id+".json"))
}

func readJournals(stateDir string) ([]Journal, error) {
	entries, err := os.ReadDir(filepath.Join(stateDir, "journal"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Journal
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(stateDir, "journal", entry.Name()))
		if err != nil {
			return nil, err
		}
		var j Journal
		if err := json.Unmarshal(data, &j); err != nil {
			return nil, fmt.Errorf("decode journal %s: %w", entry.Name(), err)
		}
		out = append(out, j)
	}
	return out, nil
}

type Backup struct {
	Path string    `json:"path"`
	Size int64     `json:"size"`
	Time time.Time `json:"time"`
}

func ListBackups(stateDir string, file FileID) ([]Backup, error) {
	entries, err := os.ReadDir(filepath.Join(stateDir, "backups", string(file)))
	if os.IsNotExist(err) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, Backup{Path: entry.Name(), Size: info.Size(), Time: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}
