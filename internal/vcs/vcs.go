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

// FileChange is one path and its two-column porcelain status.
type FileChange struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// WorkingTree is a snapshot of paths that git add -A would stage.
type WorkingTree struct {
	Changes []FileChange `json:"changes"`
	Clean   bool         `json:"clean"`
}

// Status reads a NUL-delimited porcelain status snapshot without staging or
// otherwise changing the repository.
func Status(repoDir string) (*WorkingTree, error) {
	if err := available(repoDir); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = repoDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vcs: git status failed: %s", strings.TrimSpace(out.String()))
	}
	changes, err := parsePorcelainZ(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("vcs: parsing git status: %w", err)
	}
	return &WorkingTree{Changes: changes, Clean: len(changes) == 0}, nil
}

func parsePorcelainZ(data []byte) ([]FileChange, error) {
	changes := make([]FileChange, 0)
	for offset := 0; offset < len(data); {
		end := bytes.IndexByte(data[offset:], 0)
		if end < 0 {
			end = len(data) - offset
		}
		record := data[offset : offset+end]
		offset += end + 1
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("invalid porcelain record %q", record)
		}
		status := string(record[:2])
		path := string(record[3:])
		if strings.ContainsAny(status, "RC") {
			if offset >= len(data) {
				return nil, fmt.Errorf("rename/copy record has no source path")
			}
			nextEnd := bytes.IndexByte(data[offset:], 0)
			if nextEnd < 0 {
				nextEnd = len(data) - offset
			}
			source := string(data[offset : offset+nextEnd])
			offset += nextEnd + 1
			path = source + " -> " + path
		}
		changes = append(changes, FileChange{Status: status, Path: path})
	}
	return changes, nil
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
	if r, err := run(repoDir, "add", "-A"); err != nil {
		return r, fmt.Errorf("vcs: staging the working tree: %w", err)
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
	committed, err := Commit(repoDir, message)
	if err != nil {
		return committed, err
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
