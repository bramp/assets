package commands

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/bramp/assets/internal/hash"
	"github.com/bramp/assets/internal/lockfile"
	"github.com/bramp/assets/internal/manifest"
	"github.com/bramp/assets/internal/render"
)

func RunVerifyLock(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)

	manifestPath := fs.String("manifest", "assets.yaml", "Path to assets manifest")
	lockPath := fs.String("lock", "assets.lock", "Path to lockfile")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "verify: unexpected positional arguments: %v\n", fs.Args())
		return 1
	}

	m, err := manifest.LoadFile(*manifestPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify: failed to load manifest %q: %v\n", *manifestPath, err)
		return 1
	}

	baseDir := filepath.Dir(*manifestPath)
	lf, err := lockfile.Load(filepath.Join(baseDir, *lockPath))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify: failed to load lockfile %q: %v\n", *lockPath, err)
		return 1
	}

	var errs []string
	for _, a := range m.Assets {
		sourcePath := filepath.Join(baseDir, a.Source)
		sourceHash, sourceSize, hashErr := hash.FileSHA256AndSize(sourcePath)
		if hashErr != nil {
			errs = append(errs, fmt.Sprintf("asset %q source hash failed: %v", a.ID, hashErr))
			continue
		}

		for _, out := range a.Outputs {
			lOut, ok := lf.Files[out.Path]
			if !ok {
				errs = append(errs, fmt.Sprintf("asset %q output %q missing from lockfile", a.ID, out.Path))
				continue
			}
			if !hasSourceRef(lOut.Sources, a.Source, sourceHash, sourceSize) {
				errs = append(errs, fmt.Sprintf("asset %q output %q source metadata mismatch", a.ID, out.Path))
			}

			steps, resolveErr := render.ResolvePipeline(m, a.Source, out)
			if resolveErr != nil {
				errs = append(errs, fmt.Sprintf("asset %q output %q pipeline resolve failed: %v", a.ID, out.Path, resolveErr))
				continue
			}

			currentProv := render.CollectProvenance(steps)
			if !reflect.DeepEqual(lOut.Provenance, currentProv) {
				errs = append(errs, fmt.Sprintf("asset %q output %q provenance mismatch", a.ID, out.Path))
			}

			outPath := filepath.Join(baseDir, out.Path)
			outputHash, outputSize, hashErr := hash.FileSHA256AndSize(outPath)
			if hashErr != nil {
				errs = append(errs, fmt.Sprintf("asset %q output %q missing on disk", a.ID, out.Path))
				continue
			}
			if outputSize != lOut.SizeBytes {
				errs = append(errs, fmt.Sprintf("asset %q output %q size mismatch", a.ID, out.Path))
			}
			if outputHash != lOut.SHA256 {
				errs = append(errs, fmt.Sprintf("asset %q output %q output hash mismatch", a.ID, out.Path))
			}
		}
	}

	if len(errs) == 0 {
		return 0
	}
	sort.Strings(errs)
	for _, msg := range errs {
		_, _ = fmt.Fprintf(stderr, "verify: %s\n", msg)
	}
	return 1
}

func hasSourceRef(sources map[string]lockfile.SourceRef, path string, sha256 string, sizeBytes int64) bool {
	src, ok := sources[path]
	if !ok {
		return false
	}
	return src.SHA256 == sha256 && src.SizeBytes == sizeBytes
}
