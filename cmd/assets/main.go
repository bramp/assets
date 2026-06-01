package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bramp/assets/internal/commands"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 1
	}

	var exitCode int
	switch args[0] {
	case "gen":
		exitCode = commands.RunGen(args[1:], stdout, stderr)
	case "defaults":
		exitCode = commands.RunDefaults(args[1:], stdout, stderr)
	case "doctor":
		exitCode = commands.RunDoctor(args[1:], stdout, stderr)
	case "build":
		exitCode = commands.RunBuildTarget(args[1:], stderr)
	case "verify":
		exitCode = commands.RunVerifyLock(args[1:], stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		exitCode = 1
	}

	return exitCode
}

func printUsage(stderr io.Writer) {
	_, _ = fmt.Fprintln(stderr, "Usage: assets <command> [flags]")
	_, _ = fmt.Fprintln(stderr, "")
	_, _ = fmt.Fprintln(stderr, "Commands:")
	_, _ = fmt.Fprintln(stderr, "  gen         Generate deterministic Makefile fragment")
	_, _ = fmt.Fprintln(stderr, "  defaults    Print a recommended render pipeline config snippet")
	_, _ = fmt.Fprintln(stderr, "  doctor      Diagnose tool availability and version drift")
	_, _ = fmt.Fprintln(stderr, "  build       Build a single target output")
	_, _ = fmt.Fprintln(stderr, "  verify      Verify manifest validity and output/lockfile freshness")
	_, _ = fmt.Fprintln(stderr, "")
	_, _ = fmt.Fprintln(stderr, "Use 'assets <command> -h' for command help.")
}
