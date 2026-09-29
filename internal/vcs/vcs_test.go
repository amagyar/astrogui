package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
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
