// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Actor struct {
	Username string
	Name     string
	Email    string
}

type CommitOptions struct {
	CommitterName   string
	CommitterEmail  string
	AuthorDomain    string
	ExpectedBranch  string
	SecretGuard     bool
	DenyGlobs       []string
	AllowAdminGuard bool
}

type CommitRequest struct {
	Paths          []string
	Message        string
	OverrideReason string
	Actor          Actor
}

type CommitResult struct {
	SHA     string `json:"sha"`
	Summary string `json:"summary"`
	Author  string `json:"author"`
}

var emailPattern = regexp.MustCompile(`^[^\s<>@]+@[^\s<>@]+\.[^\s<>@]+$`)

func Author(actor Actor, domain string) (string, error) {
	name := strings.TrimSpace(actor.Name)
	if name == "" {
		name = strings.TrimSpace(actor.Username)
	}
	if name == "" {
		return "", errors.New("git author name is required")
	}
	if strings.ContainsAny(name, "<>\r\n\x00") {
		return "", errors.New("git author name is invalid")
	}
	email := strings.TrimSpace(actor.Email)
	if email == "" {
		domain = strings.TrimSpace(domain)
		if domain == "" || strings.ContainsAny(domain, " /\\") {
			return "", errors.New("git author domain is invalid")
		}
		email = strings.TrimSpace(actor.Username) + "@" + domain
	}
	if !emailPattern.MatchString(email) {
		return "", errors.New("git author email is invalid")
	}
	return name + " <" + email + ">", nil
}

func (g *Git) Commit(ctx context.Context, request CommitRequest, options CommitOptions) (CommitResult, error) {
	if len(request.Paths) == 0 {
		return CommitResult{}, errors.New("nothing selected to commit")
	}
	if err := g.CheckGuards(ctx, Repository{Enabled: true, Branch: options.ExpectedBranch, InProgress: ""}, request.Paths, options, request.OverrideReason); err != nil {
		return CommitResult{}, err
	}
	author, err := Author(request.Actor, options.AuthorDomain)
	if err != nil {
		return CommitResult{}, err
	}
	committerName := options.CommitterName
	if committerName == "" {
		committerName = "runwisp-manager"
	}
	committerEmail := options.CommitterEmail
	if committerEmail == "" {
		committerEmail = "manager@localhost"
	}
	if !validEmail(committerEmail) {
		return CommitResult{}, errors.New("git committer email is invalid")
	}
	paths := make([]string, 0, len(request.Paths))
	seen := make(map[string]bool)
	for _, path := range request.Paths {
		literal, pathErr := LiteralPath(path)
		if pathErr != nil {
			return CommitResult{}, pathErr
		}
		if !seen[path] {
			seen[path] = true
			paths = append(paths, literal)
		}
	}
	if options.ExpectedBranch != "" {
		if err := ValidateRef(options.ExpectedBranch); err != nil {
			return CommitResult{}, err
		}
	}
	message := strings.TrimSpace(request.Message)
	if message == "" {
		return CommitResult{}, errors.New("commit message is required")
	}
	message += "\n\nManaged-By: runwisp-manager"
	if request.OverrideReason != "" {
		message += "\nReason: " + strings.TrimSpace(request.OverrideReason)
	}
	addArgs := append([]string{"add", "--"}, paths...)
	if _, err := g.mutate(ctx, addArgs...); err != nil {
		return CommitResult{}, fmt.Errorf("stage selected files: %w", err)
	}
	commitArgs := []string{"-c", "user.name=" + committerName, "-c", "user.email=" + committerEmail, "commit", "--only", "-F", "-", "--author=" + author, "--"}
	commitArgs = append(commitArgs, paths...)
	if _, err := g.mutateInput(ctx, []byte(message+"\n"), commitArgs...); err != nil {
		return CommitResult{}, fmt.Errorf("commit selected files: %w", err)
	}
	shaBytes, err := g.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return CommitResult{}, fmt.Errorf("read committed revision: %w", err)
	}
	return CommitResult{SHA: strings.TrimSpace(string(shaBytes)), Summary: strings.SplitN(message, "\n", 2)[0], Author: author}, nil
}

func validEmail(email string) bool {
	return email == "manager@localhost" || emailPattern.MatchString(email)
}
