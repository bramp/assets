package commands

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/bramp/assets/internal/manifest"
)

func RunCheck(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	manifestPath := fs.String("manifest", "assets.yaml", "Path to assets manifest")
	strict := fs.Bool("strict", false, "Require legal metadata fields (owner, copyright, license)")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "check: unexpected positional arguments: %v\n", fs.Args())
		return 1
	}

	m, err := manifest.LoadFile(*manifestPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "check: failed to load manifest %q: %v\n", *manifestPath, err)
		return 1
	}

	errs := m.Validate(manifest.ValidationConfig{
		Strict: *strict,
		BaseDir: filepath.Dir(*manifestPath),
	})
	if len(errs) == 0 {
		return 0
	}

	for _, vErr := range errs {
		_, _ = fmt.Fprintf(stderr, "check: %v\n", vErr)
	}
	return 1
}
