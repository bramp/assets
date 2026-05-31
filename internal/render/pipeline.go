package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

const (
	defaultBackgroundColor = "transparent"
	scaleModeFit           = "fit"
	outputDirPerm          = 0o750
)

// TargetSpec identifies the manifest asset/output pair matching a target path.
type TargetSpec struct {
	Asset  manifest.Asset
	Output manifest.Output
}

// BuildContext provides placeholder values used to render pipeline commands.
type BuildContext struct {
	InputPath  string
	OutputPath string
	Width      int
	Height     int
	ScaleMode  string
	Background string
	TmpPath    string
	Tmp2Path   string
}

// ResolveOptions controls behavior of pipeline path resolution.
type ResolveOptions struct {
	// CheckAvailability controls whether unavailable tools are filtered out.
	// Defaults to true in ResolvePipeline.
	CheckAvailability bool
}

// FindTarget resolves a manifest output path to its source asset and output spec.
func FindTarget(m *manifest.Manifest, targetPath string) (*TargetSpec, error) {
	for _, a := range m.Assets {
		for _, o := range a.Outputs {
			if o.Path == targetPath {
				return &TargetSpec{Asset: a, Output: o}, nil
			}
		}
	}
	return nil, fmt.Errorf("target not found in manifest: %s", targetPath)
}

// ResolvePipeline chooses a compatible tool chain from source to output format.
func ResolvePipeline(m *manifest.Manifest, sourcePath string, o manifest.Output) ([]manifest.PipelineStep, error) {
	return ResolvePipelineWithOptions(m, sourcePath, o, ResolveOptions{CheckAvailability: true})
}

// ResolvePipelineWithOptions resolves a compatible pipeline with caller-provided options.
func ResolvePipelineWithOptions(
	m *manifest.Manifest,
	sourcePath string,
	o manifest.Output,
	opts ResolveOptions,
) ([]manifest.PipelineStep, error) {
	sourceExt := strings.ToLower(strings.TrimSpace(filepath.Ext(sourcePath)))
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(o.Path)))

	order := buildPreferenceOrder(o.Options.Tools, m.Meta.Render.Defaults.Tools)

	steps, err := resolveGraphPath(m.Meta.Render.Tools, order, sourceExt, outputExt, o.Options.ScaleMode, opts)
	if err != nil {
		return nil, err
	}
	// TODO(bramp): Model final optimization as an explicit graph node/state so
	// terminal optimization is selected during path resolution instead of appended
	// after graph traversal.
	if steps, err = appendTerminalOptimizer(m.Meta.Render, steps, outputExt, opts); err != nil {
		return nil, err
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no pipeline steps resolved for target %q", o.Path)
	}

	return steps, nil
}
