package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bramp/assets/internal/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	var exitCode int
	switch os.Args[1] {
	case "check":
		exitCode = commands.RunCheck(os.Args[2:], os.Stderr)
	case "gen":
		exitCode = commands.RunGen(os.Args[2:], os.Stdout, os.Stderr)
	case "build-target":
		exitCode = commands.RunBuildTarget(os.Args[2:], os.Stderr)
	case "verify-lock":
		exitCode = commands.RunVerifyLock(os.Args[2:], os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage(os.Stderr)
		exitCode = 1
	}

	os.Exit(exitCode)
}

func printUsage(stderr io.Writer) {
	_, _ = fmt.Fprintln(stderr, "Usage: assets <command> [flags]")
	_, _ = fmt.Fprintln(stderr, "")
	_, _ = fmt.Fprintln(stderr, "Commands:")
	_, _ = fmt.Fprintln(stderr, "  check       Validate assets manifest and source file presence")
	_, _ = fmt.Fprintln(stderr, "  gen         Generate deterministic Makefile fragment")
	_, _ = fmt.Fprintln(stderr, "  build-target Build a single target output")
	_, _ = fmt.Fprintln(stderr, "  verify-lock Verify manifest, outputs, and lockfile alignment")
	_, _ = fmt.Fprintln(stderr, "")
	_, _ = fmt.Fprintln(stderr, "Use 'assets <command> -h' for command help.")
}
