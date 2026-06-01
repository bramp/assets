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

// ResolvedStep is a concrete runtime step selected from the tool capability
// graph for a specific source/target path.
type ResolvedStep struct {
	Name         string
	Tool         string
	Command      string
	SizeTemplate string
	SizeByMode   map[string]string
	VersionArgs  []string
	InputFormat  string
	OutputFormat string
}

// ResolveOptions controls behavior of pipeline path resolution.
type ResolveOptions struct {
	// CheckAvailability controls whether unavailable tools are filtered out.
	// Defaults to true in ResolvePipeline.
	CheckAvailability bool
	// ToolRepo supplies cached tool metadata/probes for resolve and provenance.
	// If nil, ResolvePipelineWithOptions creates a default repository.
	ToolRepo ToolRepository
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
func ResolvePipeline(m *manifest.Manifest, sourcePath string, o manifest.Output) ([]ResolvedStep, error) {
	return ResolvePipelineWithOptions(m, sourcePath, o, ResolveOptions{CheckAvailability: true})
}

// ResolvePipelineWithOptions resolves a compatible pipeline with caller-provided options.
func ResolvePipelineWithOptions(
	m *manifest.Manifest,
	sourcePath string,
	o manifest.Output,
	opts ResolveOptions,
) ([]ResolvedStep, error) {
	sourceExt := strings.ToLower(strings.TrimSpace(filepath.Ext(sourcePath)))
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(o.Path)))
	toolRepo := opts.ToolRepo
	if toolRepo == nil {
		toolRepo = NewToolRepository()
	}

	preferenceRank := buildPreferenceRank(o.Options.Tools, m.Meta.Render.Defaults.Tools)

	steps, err := resolveGraphPath(
		m.Meta.Render.Tools,
		m.Meta.Render.OptimizeByFormat,
		preferenceRank,
		sourceExt,
		outputExt,
		o.Options.ScaleMode,
		requireSizedGoal(sourceExt, outputExt, o.Width, o.Height) && !hasExplicitToolPreference(o.Options.Tools),
		opts,
		toolRepo,
	)
	if err != nil {
		return nil, err
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no pipeline steps resolved for target %q", o.Path)
	}

	return steps, nil
}

// ResolveGraphDOT renders the planned resolver graph as a DOT diagram for
// debugging path search and tie-breaking decisions.
func ResolveGraphDOT(
	m *manifest.Manifest,
	sourcePath string,
	o manifest.Output,
	opts ResolveOptions,
) (string, error) {
	sourceExt := strings.ToLower(strings.TrimSpace(filepath.Ext(sourcePath)))
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(o.Path)))
	toolRepo := opts.ToolRepo
	if toolRepo == nil {
		toolRepo = NewToolRepository()
	}

	preferenceRank := buildPreferenceRank(o.Options.Tools, m.Meta.Render.Defaults.Tools)
	resolver := newGraphResolver(
		m.Meta.Render.Tools,
		m.Meta.Render.OptimizeByFormat,
		preferenceRank,
		sourceExt,
		outputExt,
		o.Options.ScaleMode,
		requireSizedGoal(sourceExt, outputExt, o.Width, o.Height) && !hasExplicitToolPreference(o.Options.Tools),
		opts.CheckAvailability,
		toolRepo,
	)

	return resolver.graphDOT(sourcePath + " -> " + o.Path)
}
