package commands

import (
	"flag"
	"fmt"
	"io"

	"github.com/bramp/assets/internal/manifest"
)

var defaultsYAML = "# Suggested render defaults for assets.yaml\n" +
	"# Copy this under meta.render in your manifest.\n" +
	"# Graph tool preferences are order-based tie-breakers for shortest compatible paths.\n" +
	manifest.BuiltinRenderDefaultsYAML()

func RunDefaults(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("defaults", flag.ContinueOnError)
	fs.SetOutput(stderr)

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "defaults: unexpected positional arguments: %v\n", fs.Args())
		return 1
	}

	_, _ = io.WriteString(stdout, defaultsYAML)

	return 0
}
