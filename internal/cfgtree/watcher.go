package cfgtree

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Event struct {
	TreeVersion  string   `json:"tree_version"`
	ChangedFiles []string `json:"changed_files"`
	Stale        bool     `json:"stale"`
}

type Watcher struct {
	options Options
	poll    bool
	events  chan Event
	mu      sync.RWMutex
	tree    Tree
	stop    chan struct{}
	done    chan struct{}
	close   sync.Once
}

func NewWatcher(options Options, poll bool) (*Watcher, error) {
	tree, err := Discover(options)
	if err != nil {
		return nil, err
	}
	tree = Index(tree)
	w := &Watcher{options: options, poll: poll, events: make(chan Event, 16), tree: tree, stop: make(chan struct{}), done: make(chan struct{})}
	return w, nil
}

func (w *Watcher) Tree() Tree           { w.mu.RLock(); defer w.mu.RUnlock(); return cloneTree(w.tree) }
func (w *Watcher) Events() <-chan Event { return w.events }

func (w *Watcher) Start(ctx context.Context) {
	go func() {
		defer close(w.done)
		if w.poll {
			w.runPoll(ctx)
			return
		}
		w.runFSNotify(ctx)
	}()
}

func (w *Watcher) Close() { w.close.Do(func() { close(w.stop); <-w.done; close(w.events) }) }

func (w *Watcher) runPoll(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.reload()
		}
	}
}

func (w *Watcher) runFSNotify(ctx context.Context) {
	notifier, err := fsnotify.NewWatcher()
	if err != nil {
		w.reload()
		return
	}
	defer notifier.Close()
	w.addDirs(notifier)
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-w.stop:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-notifier.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(200 * time.Millisecond)
			} else {
				timer.Stop()
				timer.Reset(200 * time.Millisecond)
			}
			timerC = timer.C
		case <-timerC:
			timerC = nil
			w.reload()
			w.addDirs(notifier)
		}
	}
}

func (w *Watcher) addDirs(notifier *fsnotify.Watcher) {
	seen := map[string]bool{}
	for _, file := range w.Tree().Files {
		dir := filepath.Dir(file.Path)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		_ = notifier.Add(dir)
	}
}

func (w *Watcher) reload() {
	next, err := Discover(w.options)
	if err != nil {
		return
	}
	next = Index(next)
	w.mu.Lock()
	previous := w.tree
	w.tree = next
	w.mu.Unlock()
	changed := changedFiles(previous, next)
	if len(changed) == 0 && previous.TreeVersion == next.TreeVersion {
		return
	}
	w.emit(Event{TreeVersion: next.TreeVersion, ChangedFiles: changed, Stale: true})
}

func (w *Watcher) emit(event Event) {
	select {
	case w.events <- event:
	default:
	}
}

func changedFiles(before, after Tree) []string {
	old := make(map[string]string, len(before.Files))
	for _, f := range before.Files {
		old[f.Path] = f.Version
	}
	var out []string
	for _, f := range after.Files {
		if old[f.Path] != f.Version {
			out = append(out, f.Path)
		}
	}
	for path := range old {
		found := false
		for _, f := range after.Files {
			if f.Path == path {
				found = true
				break
			}
		}
		if !found {
			out = append(out, path)
		}
	}
	return out
}

func cloneTree(tree Tree) Tree {
	tree.Files = append([]File(nil), tree.Files...)
	tree.Warnings = append([]string(nil), tree.Warnings...)
	tree.Units = append([]Unit(nil), tree.Units...)
	for i := range tree.Units {
		tree.Units[i].Fields = cloneMap(tree.Units[i].Fields)
	}
	return tree
}

func PollEnabled() bool { return os.Getenv("RWM_FS_POLL") == "true" }
