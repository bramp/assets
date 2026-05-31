package commands

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/bramp/assets/internal/hash"
	"github.com/bramp/assets/internal/lockfile"
	"github.com/bramp/assets/internal/manifest"
	"github.com/bramp/assets/internal/render"
)

// RunBuildTarget builds a single manifest output target and updates the lockfile.
func RunBuildTarget(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)

	manifestPath := fs.String("manifest", "assets.yaml", "Path to assets manifest")
	target := fs.String("target", "", "Target output path to build")
	lockPath := fs.String("lock", "assets.lock", "Path to lockfile")
	quiet := fs.Bool("quiet", false, "Suppress command output while building")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "build: unexpected positional arguments: %v\n", fs.Args())
		return 1
	}
	if *target == "" {
		_, _ = fmt.Fprintln(stderr, "build: --target is required")
		return 1
	}

	m, err := manifest.LoadFile(*manifestPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build: failed to load manifest %q: %v\n", *manifestPath, err)
		return 1
	}

	spec, err := render.FindTarget(m, *target)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	steps, err := render.ResolvePipeline(m, spec.Asset.Source, spec.Output)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	baseDir := filepath.Dir(*manifestPath)
	ctx := render.BuildContext{
		InputPath:  filepath.Join(baseDir, spec.Asset.Source),
		OutputPath: filepath.Join(baseDir, spec.Output.Path),
		Width:      spec.Output.Width,
		Height:     spec.Output.Height,
		ScaleMode:  spec.Output.Options.ScaleMode,
		Background: spec.Output.Options.Background,
	}

	onCommand := func(string) {}
	if !*quiet {
		onCommand = func(cmd string) {
			_, _ = fmt.Fprintln(stderr, cmd)
		}
	}

	if err := render.ExecutePipelineWithHook(steps, ctx, onCommand); err != nil {
		_, _ = fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	outputHash, outputSize, err := hash.FileSHA256AndSize(ctx.OutputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build: failed to hash output: %v\n", err)
		return 1
	}

	sourceHash, sourceSize, err := hash.FileSHA256AndSize(ctx.InputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build: failed to hash source: %v\n", err)
		return 1
	}

	lockAbsPath := filepath.Join(baseDir, *lockPath)
	provenance := render.CollectProvenance(steps)
	if err := saveBuildResultWithRetry(
		lockAbsPath,
		spec.Asset.Source,
		spec.Output.Path,
		sourceHash,
		outputHash,
		sourceSize,
		outputSize,
		provenance,
	); err != nil {
		_, _ = fmt.Fprintf(stderr, "build: failed to update lockfile: %v\n", err)
		return 1
	}

	return 0
}

func saveBuildResultWithRetry(
	lockAbsPath, sourcePath, outputPath, sourceHash, outputHash string,
	sourceSize, outputSize int64,
	provenance *lockfile.Provenance,
) error {
	ls, err := lockfile.Open(lockAbsPath)
	if err != nil {
		return err
	}
	defer func() { _ = ls.Close() }()

	ls.UpsertOutput(outputPath, lockfile.GeneratedRef{
		Sources:    map[string]lockfile.SourceRef{sourcePath: {SHA256: sourceHash, SizeBytes: sourceSize}},
		Provenance: provenance,
		SHA256:     outputHash,
		SizeBytes:  outputSize,
	})
	return ls.SaveWithRetry(6, 10*time.Millisecond)
}
