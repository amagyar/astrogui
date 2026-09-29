package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/astrogui/astrogui/internal/cache"
	"github.com/astrogui/astrogui/internal/config"
	"github.com/astrogui/astrogui/internal/lifecycle"
	"github.com/astrogui/astrogui/internal/posts"
	"github.com/astrogui/astrogui/internal/project"
	"github.com/astrogui/astrogui/internal/safe"
	"github.com/astrogui/astrogui/internal/server"
	"github.com/astrogui/astrogui/internal/watch"
)

// preferredPort is the port astrogui tries first; a conflict is reported and
// a free port takes its place.
const preferredPort = 41990

// serve resolves the enclosing project and hosts the board interface until
// the context is cancelled.
func serve(ctx context.Context, open bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	proj, err := project.Find(cwd)
	if err != nil {
		return err // names the directory searched
	}

	cfgFile, err := config.Load()
	if err != nil {
		return err
	}
	cfg := cfgFile.For(proj.Root)

	cols, err := project.DetectCollections(proj.Root)
	if err != nil {
		return err
	}
	chosen, err := project.ChooseCollection(cols, os.Stdin, os.Stderr)
	if err != nil {
		return err // no collection is selected without an explicit answer
	}
	collection := project.Managed(proj.Root, cfg.ContentPath(proj.Root), chosen)

	// Persist a non-default choice so subsequent runs skip the prompt.
	if cfg.ContentDir != "" && cfg.Collection == "" {
		cfg.Collection = collection.Name
	}

	ideas := cfg.IdeasPath(proj.Root)
	wip := cfg.WipPath(proj.Root)
	content := collection.Dir

	guard, err := safe.New(ideas, wip, content)
	if err != nil {
		return err
	}
	derived, err := cache.Open()
	if err != nil {
		return err
	}

	app := &server.App{
		Project:    proj.Root,
		Collection: collection,
		Config:     cfg,
		Version:    version,
		Guard:      guard,
		Cache:      derived,
		Manager:    lifecycle.New(ideas, wip, content, guard, derived, proj.Root),
	}

	// Record first-seen for posts that predate this run (derived data; the
	// filesystem timestamps remain the fallback of record).
	replayFirstSeen(app)
	_ = derived.Save()

	srv, err := server.New(app, server.UI())
	if err != nil {
		return err
	}

	ln, conflict, err := server.Listen(preferredPort)
	if err != nil {
		return err
	}
	defer ln.Close()
	if conflict != "" {
		fmt.Fprintln(os.Stderr, "astrogui:", conflict)
	}
	url := srv.Announce(ln.Addr().String())
	fmt.Fprintln(os.Stderr, "astrogui: serving", url)

	if open {
		openBrowser(url)
	}

	// The watcher keeps the board live; it is advisory, and its failure is
	// never fatal (the board also polls).
	if watcher, werr := watch.New(ideas, wip, content); werr == nil {
		defer watcher.Close()
	} else {
		fmt.Fprintln(os.Stderr, "astrogui: watching unavailable, board will poll:", werr)
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// openBrowser opens the announced URL in the user's browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "astrogui: open the interface at", url)
	}
	_ = cmd.Process.Release()
}

// replayFirstSeen walks the managed directories once at startup so derived
// ages exist for posts created before this run (still derived data; the
// filesystem timestamps are the fallback of record).
func replayFirstSeen(app *server.App) {
	now := time.Now()
	l, err := posts.List(app.Manager.Ideas, app.Manager.WIP, app.Manager.Content)
	if err != nil {
		return
	}
	for _, p := range l.All() {
		app.Cache.FirstSeen(app.Project, p.Name, now)
	}
}
