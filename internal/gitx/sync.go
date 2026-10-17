// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

// FetchResult is the in-memory outcome of the last fetch attempt.
type FetchResult struct {
	Remote    string    `json:"remote"`
	FetchedAt time.Time `json:"lastFetch,omitempty"`
	Error     string    `json:"lastFetchError,omitempty"`
}

// Event is emitted after fetch attempts and can be forwarded by an SSE
// adapter. The channel is deliberately bounded so a slow browser cannot stop
// Git polling.
type Event struct {
	Kind   string      `json:"kind"`
	Status FetchResult `json:"status"`
}

// Sync owns background fetch state for one repository.
type Sync struct {
	git     *Git
	remote  string
	mu      sync.RWMutex
	result  FetchResult
	events  chan Event
	stop    chan struct{}
	done    chan struct{}
	started bool
}

func NewSync(g *Git, remote string) *Sync {
	if remote == "" {
		remote = "origin"
	}
	return &Sync{git: g, remote: remote, events: make(chan Event, 16), stop: make(chan struct{}), done: make(chan struct{})}
}

func (s *Sync) Events() <-chan Event { return s.events }

func (s *Sync) Result() FetchResult { s.mu.RLock(); defer s.mu.RUnlock(); return s.result }

// Fetch updates remote refs without touching the working tree.
func (s *Sync) Fetch(ctx context.Context) FetchResult {
	result := FetchResult{Remote: s.remote, FetchedAt: time.Now()}
	if s.git == nil {
		result.Error = "git runner is nil"
	} else if _, err := s.git.mutate(ctx, "fetch", "--prune", s.remote); err != nil {
		result.Error = err.Error()
	}
	s.mu.Lock()
	s.result = result
	s.mu.Unlock()
	s.emit(Event{Kind: "git", Status: result})
	return result
}

// Start launches polling. Poll intervals are jittered by ±10%; failures use
// exponential backoff up to one hour, while cancellation always terminates
// the goroutine and closes Events.
func (s *Sync) Start(ctx context.Context, interval time.Duration) {
	s.mu.Lock()
	if s.started || interval <= 0 {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()
	go func() {
		defer close(s.done)
		defer close(s.events)
		backoff := interval
		for {
			wait := jitter(backoff)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-s.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			result := s.Fetch(ctx)
			if result.Error == "" {
				backoff = interval
			} else if backoff < time.Hour {
				backoff *= 2
				if backoff > time.Hour {
					backoff = time.Hour
				}
			}
		}
	}()
}

func (s *Sync) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *Sync) emit(event Event) {
	select {
	case s.events <- event:
	default:
		select {
		case <-s.events:
		default:
		}
		select {
		case s.events <- event:
		default:
		}
	}
}

func jitter(value time.Duration) time.Duration {
	if value <= 1 {
		return value
	}
	percent := rand.Float64()*0.2 - 0.1
	return time.Duration(float64(value) * (1 + percent))
}
