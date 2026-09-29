// Package vcs implements the explicit version control actions: commit and
// commit-and-push. Lifecycle moves trigger no version control operation;
// staging is a separate, explicit act the user performs.
//
// The commands run are exactly the ones the user already runs themselves
// (git add, git commit, git push in their own repository), which is what the
// consent-based boundary allows.
package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrUnavailable reports that a required command is not present in the
// environment. The action is not reported as successful.
var ErrUnavailable = errors.New("git is not available in this environment")

// Result reports a command's outcome verbatim, so failures surface with
// their own output.
type Result struct {
	Command string
	Output  string
}

// Commit stages the working tree and creates one commit with the message.
// Several posts published beforehand are included in the single commit
// because staging is the whole tree.
func Commit(repoDir, message string) (*Result, error) {
	if err := available(repoDir); err != nil {
		return nil, err
	}
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("vcs: commit message is empty; the user confirms the message before the commit is created")
	}
	if _, err := run(repoDir, "add", "-A"); err != nil {
		return nil, fmt.Errorf("vcs: staging the working tree: %w", err)
	}
	// Nothing staged: report plainly instead of failing the commit.
	if _, err := run(repoDir, "diff", "--cached", "--quiet"); err == nil {
		return &Result{Command: "git diff --cached --quiet", Output: "nothing to commit: the working tree is clean"}, nil
	}
	return run(repoDir, "commit", "-m", message)
}

// CommitAndPush commits as Commit does and then pushes to the tracked
// remote. A push failure is returned with its output and is never reported
// as success.
func CommitAndPush(repoDir, message string) (*Result, error) {
	if _, err := Commit(repoDir, message); err != nil {
		return nil, err
	}
	r, err := run(repoDir, "push")
	if err != nil {
		return r, fmt.Errorf("vcs: push failed (the commit was created):\n%s", r.Output)
	}
	return r, nil
}

// available verifies git exists before any action is attempted.
func available(dir string) error {
	_, err := exec.LookPath("git")
	if err != nil {
		return ErrUnavailable
	}
	if _, err := run(dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("vcs: %s is not a git repository", dir)
	}
	return nil
}

func run(dir string, args ...string) (*Result, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	r := &Result{
		Command: "git " + strings.Join(args, " "),
		Output:  strings.TrimSpace(out.String()),
	}
	if err != nil {
		if r.Output == "" {
			r.Output = err.Error()
		}
		return r, fmt.Errorf("%s failed: %w", r.Command, err)
	}
	return r, nil
}
