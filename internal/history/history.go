// SPDX-License-Identifier: GPL-3.0-or-later
// Package history merges recoverable configuration versions for one unit.
package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/cfgsave"
	"github.com/runwisp/runwisp-manager/internal/gitx"
	"github.com/runwisp/runwisp-manager/internal/tomledit"
)

type Version struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"ts"`
	Source   string    `json:"source"`
	Actor    string    `json:"actor,omitempty"`
	Summary  string    `json:"summary,omitempty"`
	UnitHash string    `json:"unitHash"`
	SHA      string    `json:"sha,omitempty"`
	Text     string    `json:"text,omitempty"`
}
type Service struct {
	Root, StateDir string
	Audit          *audit.Store
	Git            *gitx.Git
}

func (s *Service) List(ctx context.Context, file string, unitPath []string, includeText bool) ([]Version, error) {
	var versions []Version
	seen := map[string]bool{}
	add := func(ts time.Time, source, actor, summary, sha string, text []byte) {
		if len(text) == 0 {
			return
		}
		sum := sha256.Sum256(text)
		uh := hex.EncodeToString(sum[:])
		if seen[uh] {
			return
		}
		seen[uh] = true
		v := Version{ID: source + ":" + uh, Time: ts, Source: source, Actor: actor, Summary: summary, UnitHash: uh, SHA: sha}
		if includeText {
			v.Text = string(text)
		}
		versions = append(versions, v)
	}
	backups, err := cfgsave.ListBackups(s.StateDir, cfgsave.FileID(file))
	if err != nil {
		return nil, err
	}
	for _, b := range backups {
		data, readErr := os.ReadFile(filepath.Join(s.StateDir, "backups", file, b.Path))
		if readErr != nil {
			continue
		}
		doc, parseErr := tomledit.Parse(data)
		if parseErr == nil {
			text, _ := doc.ExtractUnit(unitPath)
			add(b.Time, "backup", "", "configuration backup", "", text)
		}
	}
	if s.Git != nil {
		logs, logErr := s.Git.Log(ctx, file, 200)
		if logErr == nil {
			for _, entry := range logs {
				data, showErr := s.Git.Show(ctx, entry.Hash, file)
				if showErr != nil {
					continue
				}
				doc, parseErr := tomledit.Parse(data)
				if parseErr == nil {
					unit, _ := doc.ExtractUnit(unitPath)
					ts, _ := time.Parse(time.RFC3339, entry.Date)
					add(ts, "git", entry.Author, entry.Subject, entry.Hash, unit)
				}
			}
		}
	}
	if s.Audit != nil {
		events, listErr := s.Audit.List(ctx, audit.Filter{Limit: 500, Target: file})
		if listErr == nil {
			for _, event := range events {
				if strings.HasPrefix(event.Action, "config.") {
					add(event.Time, "audit", event.Username, event.Action, event.GitSHA, nil)
				}
			}
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Time.After(versions[j].Time) })
	return versions, nil
}

func (s *Service) Text(ctx context.Context, file string, unitPath []string, id string, admin bool) (Version, error) {
	versions, err := s.List(ctx, file, unitPath, true)
	if err != nil {
		return Version{}, err
	}
	for _, v := range versions {
		if v.ID == id {
			if !admin {
				v.Text = mask(v.Text)
			}
			return v, nil
		}
	}
	return Version{}, fmt.Errorf("history version not found")
}
func mask(value string) string {
	doc, err := tomledit.Parse([]byte(value))
	if err != nil {
		return "[REDACTED]"
	}
	_ = doc
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
			if idx := strings.Index(line, "="); idx >= 0 {
				lines[i] = line[:idx+1] + " \"[REDACTED]\""
			}
		}
	}
	return strings.Join(lines, "\n")
}
