// Command astrogui is a locally-run management tool for an Astro blog project.
//
// It is not a dependency of the blog: it is a standalone binary run from inside
// the project directory. See the README for the operating boundary.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is stamped at build time via -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "astrogui: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("astrogui", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `astrogui %s — a local cockpit for an Astro blog

Usage:

  astrogui            start the board for the enclosing Astro project
  astrogui version    print the version and exit

Flags:

`, version)
		fs.PrintDefaults()
	}
	printVersion := fs.Bool("version", false, "print the version and exit")
	noOpen := fs.Bool("no-open", false, "start the server without opening the browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// The tool takes no arguments other than the "version" alias.
	rest := fs.Args()
	if len(rest) == 1 && rest[0] == "version" {
		*printVersion = true
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("astrogui takes no arguments (unexpected %q); run `astrogui --help`", rest)
	}
	if *printVersion {
		fmt.Println(version)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, !*noOpen)
}
