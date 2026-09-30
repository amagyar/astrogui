package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/astrogui/astrogui/internal/config"
)

func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	return dir
}

func TestRememberCollectionPreservesOtherOverrides(t *testing.T) {
	isolateConfig(t)
	const root = "/project"
	file := config.File{}
	file.Set(root, config.Project{ContentDir: "content/posts", IdeasDir: "notes/ideas"})
	if err := rememberCollection(file, root, "blog"); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Projects[root]
	if got.Collection != "blog" || got.ContentDir != "content/posts" || got.IdeasDir != "notes/ideas" {
		t.Fatalf("saved project config = %+v", got)
	}
}

func TestRememberSelectionPersistsOnlyNewOrRepairedChoices(t *testing.T) {
	isolateConfig(t)
	const root = "/project"
	if err := rememberSelection(config.File{}, root, "", false, "blog"); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Projects[root].Collection; got != "blog" {
		t.Fatalf("new selection saved as %q", got)
	}

	otherConfig := t.TempDir()
	t.Setenv("HOME", otherConfig)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(otherConfig, ".config"))
	t.Setenv("AppData", filepath.Join(otherConfig, "AppData")) // os.UserConfigDir reads %AppData% on Windows
	if err := rememberSelection(config.File{}, root, "blog", true, "blog"); err != nil {
		t.Fatal(err)
	}
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("honored preference unexpectedly rewrote config: %v", err)
	}
}

func TestRememberCollectionReportsSaveFailure(t *testing.T) {
	isolateConfig(t)
	configPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err = rememberCollection(config.File{}, "/project", "blog")
	if err == nil {
		t.Fatal("rememberCollection succeeded despite an invalid config destination")
	}
}

func TestBrowserLaunchFailurePrintsManualURLWithoutPanic(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-opener")
	var stderr bytes.Buffer
	startBrowserCommand(exec.Command(missing), "http://127.0.0.1:1234/#token=x", &stderr)
	if !strings.Contains(stderr.String(), "http://127.0.0.1:1234/#token=x") {
		t.Fatalf("manual URL missing from fallback: %q", stderr.String())
	}
}
