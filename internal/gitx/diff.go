// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	secretAssignment = regexp.MustCompile(`(?i)([A-Za-z0-9_.-]*(?:pass(?:word)?|secret|token|api[_-]?key|credential|private[_-]?key)[A-Za-z0-9_.-]*\s*=\s*)(["']?)([^"'\s#]+)`) //nolint:gochecknoglobals
	privateBlock     = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]+-----.*?-----END [A-Z0-9 ]+-----`)
	tokenValue       = regexp.MustCompile(`(?i)\b(?:ghp|glpat|xox[baprs]|sk)-[A-Za-z0-9_-]{12,}\b`)
)

func (g *Git) Diff(ctx context.Context, path string, staged bool) (string, error) {
	literal, err := LiteralPath(path)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--no-color", "--no-ext-diff", "-U3"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", literal)
	out, err := g.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("read git diff: %w", err)
	}
	if len(out) == 0 && !staged {
		data, readErr := os.ReadFile(g.repo + "/" + path)
		if readErr == nil {
			out = []byte("diff --git a/" + path + " b/" + path + "\nnew file mode 100644\n--- /dev/null\n+++ b/" + path + "\n@@ -0,0 +1," + strconv.Itoa(max(1, strings.Count(string(data), "\n"))) + " @@\n" + addLines(string(data)))
		}
	}
	return MaskDiff(string(out)), nil
}

func addLines(data string) string {
	lines := strings.SplitAfter(data, "\n")
	var b strings.Builder
	for _, line := range lines {
		if line != "" {
			b.WriteByte('+')
			b.WriteString(line)
		}
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MaskDiff(diff string) string {
	diff = privateBlock.ReplaceAllString(diff, "-----BEGIN [REDACTED]-----\n[secret omitted]\n-----END [REDACTED]-----")
	diff = secretAssignment.ReplaceAllString(diff, `${1}${2}[REDACTED]`)
	return tokenValue.ReplaceAllString(diff, "[REDACTED]")
}

type LogEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

func (g *Git) Log(ctx context.Context, path string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	args := []string{"log", "-n", strconv.Itoa(limit), "--format=%H%x1f%an%x1f%aI%x1f%s%x1e", "--follow", "--"}
	if path != "" {
		literal, err := LiteralPath(path)
		if err != nil {
			return nil, err
		}
		args = append(args, literal)
	}
	out, err := g.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("read git log: %w", err)
	}
	var entries []LogEntry
	for _, record := range strings.Split(string(out), "\x1e") {
		fields := strings.Split(strings.TrimSpace(record), "\x1f")
		if len(fields) != 4 {
			continue
		}
		entries = append(entries, LogEntry{Hash: fields[0], Author: fields[1], Date: fields[2], Subject: fields[3]})
	}
	return entries, nil
}
