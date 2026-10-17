package cfgsave

import (
	"context"
	"fmt"
	"sort"

	"github.com/runwisp/runwisp-manager/internal/tomledit"
	"github.com/runwisp/runwisp-manager/internal/validate"
)

// Validator is supplied by the application so cfgsave can run either the
// quick schema check or the full Phase 07 shadow-tree validation.
type Validator func(context.Context, []validate.Candidate) (validate.Result, error)

// Candidate is the computed replacement for one file.
type Candidate struct {
	File   FileID `json:"file"`
	Path   string `json:"path"`
	Before []byte `json:"-"`
	After  []byte `json:"-"`
}

type Preview struct {
	Candidates []PreviewFile   `json:"files"`
	Validation validate.Result `json:"validation"`
	Conflicts  []Conflict      `json:"conflicts,omitempty"`
}

type PreviewFile struct {
	File    FileID `json:"file"`
	Diff    string `json:"diff"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// Compute applies all requested edits in memory and performs validation. No
// mutation of the filesystem occurs here.
func Compute(ctx context.Context, change Change, resolver Resolver, root string, validator Validator) ([]Candidate, validate.Result, error) {
	if len(change.Ops) == 0 && len(change.Raw) == 0 {
		return nil, validate.Result{Valid: true}, fmt.Errorf("change has no file edits")
	}
	seen := map[FileID]bool{}
	var candidates []Candidate
	for _, group := range change.Ops {
		path, err := resolver(group.File)
		if err != nil {
			return nil, validate.Result{}, fmt.Errorf("resolve %s: %w", group.File, err)
		}
		if seen[group.File] {
			return nil, validate.Result{}, fmt.Errorf("duplicate file edit %s", group.File)
		}
		seen[group.File] = true
		before, err := readFile(path)
		if err != nil {
			return nil, validate.Result{}, err
		}
		doc, err := tomledit.Parse(before)
		if err != nil {
			return nil, validate.Result{}, fmt.Errorf("parse %s: %w", group.File, err)
		}
		after, err := doc.Apply(group.Ops)
		if err != nil {
			return nil, validate.Result{}, fmt.Errorf("apply %s: %w", group.File, err)
		}
		candidates = append(candidates, Candidate{File: group.File, Path: path, Before: before, After: after})
	}
	for _, raw := range change.Raw {
		path, err := resolver(raw.File)
		if err != nil {
			return nil, validate.Result{}, fmt.Errorf("resolve %s: %w", raw.File, err)
		}
		if seen[raw.File] {
			return nil, validate.Result{}, fmt.Errorf("duplicate file edit %s", raw.File)
		}
		seen[raw.File] = true
		before, err := readFile(path)
		if err != nil {
			return nil, validate.Result{}, err
		}
		candidates = append(candidates, Candidate{File: raw.File, Path: path, Before: before, After: append([]byte(nil), raw.Content...)})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	if validator == nil {
		return candidates, validate.Result{Valid: true}, nil
	}
	inputs := make([]validate.Candidate, len(candidates))
	for i, candidate := range candidates {
		inputs[i] = validate.Candidate{Path: candidate.Path, Content: candidate.After}
	}
	result, err := validator(ctx, inputs)
	if err != nil {
		return nil, validate.Result{}, fmt.Errorf("validate candidate: %w", err)
	}
	return candidates, result, nil
}

func makePreview(candidates []Candidate, result validate.Result) Preview {
	out := Preview{Validation: result}
	for _, candidate := range candidates {
		diff := UnifiedDiff(candidate.Before, candidate.After, string(candidate.File))
		added, removed := diffStats(diff)
		out.Candidates = append(out.Candidates, PreviewFile{File: candidate.File, Diff: diff, Added: added, Removed: removed})
	}
	return out
}

func diffStats(diff string) (added, removed int) {
	for _, line := range splitLines(diff) {
		if len(line) == 0 || line[0] == '@' || line[0] == '-' && len(line) > 1 && line[1] == '-' || line[0] == '+' && len(line) > 1 && line[1] == '+' {
			continue
		}
		if line[0] == '+' {
			added++
		}
		if line[0] == '-' {
			removed++
		}
	}
	return
}

func splitLines(s string) []string {
	var out []string
	for len(s) > 0 {
		i := 0
		for i < len(s) && s[i] != '\n' {
			i++
		}
		out = append(out, s[:i])
		if i == len(s) {
			break
		}
		s = s[i+1:]
	}
	return out
}
