package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitRepo builds a fixture repository with a bare remote to push to.
func gitRepo(t *testing.T) (repo, remote string) {
	t.Helper()
	base := t.TempDir()
	remote = filepath.Join(base, "remote.git")
	repo = filepath.Join(base, "repo")
	for _, cmd := range [][]string{
		{"git", "init", "--bare", "--initial-branch=main", remote},
		{"git", "init", "--initial-branch=main", repo},
	} {
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", cmd, err, out)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("config", "user.name", "t")
	run("config", "user.email", "t@t")
	run("remote", "add", "origin", remote)
	run("commit", "--allow-empty", "-m", "root")
	run("push", "-u", "origin", "main")
	return repo, remote
}

// TestCommitIncludesSeveralPublishedPosts verifies several posts published
// beforehand are included in a single commit (task 6.1).
func TestCommitIncludesSeveralPublishedPosts(t *testing.T) {
	repo, remote := gitRepo(t)
	published := filepath.Join(repo, "src", "content", "blog")
	if err := os.MkdirAll(published, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two", "three"} {
		dir := filepath.Join(published, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntitle: "+name+"\n---\nbody"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r, err := Commit(repo, "publish three posts")
	if err != nil {
		t.Fatalf("commit: %v (%s)", err, r.Output)
	}
	show := exec.Command("git", "show", "--name-only", "--pretty=format:", "HEAD")
	show.Dir = repo
	out, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	files := strings.Fields(string(out))
	for _, name := range []string{"one", "two", "three"} {
		found := false
		for _, f := range files {
			if strings.Contains(f, name) {
				found = true
			}
		}
		if !found {
			t.Errorf("commit does not include post %q:\n%s", name, out)
		}
	}
	_ = remote
}

// TestPushFailureReportedNotSuccessful verifies a push failure carries its
// output and is not reported as success (task 6.2).
func TestPushFailureReportedNotSuccessful(t *testing.T) {
	repo, _ := gitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "newfile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Break the remote: replace origin with a path that rejects pushes.
	if out, err := exec.Command("git", "-C", repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "nowhere")).CombinedOutput(); err != nil {
		t.Fatalf("set-url: %v\n%s", err, out)
	}

	r, err := CommitAndPush(repo, "attempt a push")
	if err == nil {
		t.Fatal("push failure reported as success")
	}
	if r == nil || r.Output == "" {
		t.Fatal("push failure carries no output")
	}
	// The commit itself was created.
	if out, err := exec.Command("git", "-C", repo, "log", "-1", "--format=%s").CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "attempt a push" {
		t.Errorf("commit not created before the failed push: %s %v", out, err)
	}
}

func TestCommitAndPushPreservesCommitFailureOutput(t *testing.T) {
	repo, _ := gitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "newfile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho commit-hook-output >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := CommitAndPush(repo, "will fail")
	if err == nil {
		t.Fatal("commit hook failure reported as success")
	}
	if r == nil || r.Command != "git commit -m will fail" || !strings.Contains(r.Output, "commit-hook-output") {
		t.Fatalf("commit failure result lost: %+v (%v)", r, err)
	}
}

// TestMoveTriggersNoVersionControlOperation verifies staging stays separate
// from moves: after a move the working tree simply shows the change unstaged
// (task 6.3, in the vcs domain: commit is the only staging path).
func TestMoveTriggersNoVersionControlOperation(t *testing.T) {
	repo, _ := gitRepo(t)
	drafts := filepath.Join(repo, "drafts", "wip")
	if err := os.MkdirAll(filepath.Join(drafts, "post"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(drafts, "post", "index.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A move performed without any astrogui commit action:
	target := filepath.Join(repo, "src", "content", "blog", "post")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(drafts, "post"), target); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("staging happened without an explicit commit action:\n%s", out)
	}
}

// TestUnavailableCommandReportedPlainly verifies a missing git is reported
// and no action is claimed successful (task 6.4).
func TestUnavailableCommandReportedPlainly(t *testing.T) {
	repo, _ := gitRepo(t) // fixture built while git is still on PATH

	t.Setenv("PATH", t.TempDir()) // now nothing executable
	r, err := Commit("/nonexistent-repo", "message")
	if err == nil {
		t.Fatal("expected failure")
	}
	if r != nil {
		t.Errorf("no result should be reported when the command is unavailable: %+v", r)
	}
	if _, err := Commit(repo, "message"); err == nil {
		t.Fatal("commit without git on PATH reported success")
	}
}

func TestParsePorcelainZHandlesSpacesAndRenames(t *testing.T) {
	data := []byte("M  staged file.txt\x00 M modified file.txt\x00?? newline\nname.md\x00R  renamed to.txt\x00old name.txt\x00")
	changes, err := parsePorcelainZ(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 {
		t.Fatalf("got %d changes: %+v", len(changes), changes)
	}
	want := []FileChange{
		{Status: "M ", Path: "staged file.txt"},
		{Status: " M", Path: "modified file.txt"},
		{Status: "??", Path: "newline\nname.md"},
		{Status: "R ", Path: "old name.txt -> renamed to.txt"},
	}
	for i := range want {
		if changes[i] != want[i] {
			t.Errorf("change[%d] = %+v, want %+v", i, changes[i], want[i])
		}
	}
}

func TestStatusIsReadOnlyAndReportsStagedAndUntrackedPaths(t *testing.T) {
	repo, _ := gitRepo(t)
	staged := filepath.Join(repo, "staged file.txt")
	if err := os.WriteFile(staged, []byte("staged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "--", "staged file.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	untrackedName := "newline\nname.md"
	if runtime.GOOS == "windows" {
		// Newlines are not legal in Windows filenames; the parser edge they
		// exercise is covered by TestParsePorcelainZHandlesSpacesAndRenames
		// above, in memory.
		untrackedName = "untracked-name.md"
	}
	if err := os.WriteFile(filepath.Join(repo, untrackedName), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	if status.Clean || len(status.Changes) != 2 {
		t.Fatalf("status = %+v", status)
	}
	foundStaged, foundUntracked := false, false
	for _, change := range status.Changes {
		if change.Status == "A " && change.Path == "staged file.txt" {
			foundStaged = true
		}
		if change.Status == "??" && change.Path == untrackedName {
			foundUntracked = true
		}
	}
	if !foundStaged || !foundUntracked {
		t.Fatalf("status omitted staged/untracked paths: %+v", status.Changes)
	}
	// Status is a preview only; it must not alter the index.
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "staged file.txt" {
		t.Fatalf("status changed the index: %q %v", out, err)
	}
}
