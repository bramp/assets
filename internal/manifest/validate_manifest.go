package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Validate checks manifest structure, references, and configured policy constraints.
func (m *Manifest) Validate(cfg ValidationConfig) []error {
	var errs []error

	errs = append(errs, validateMetaConfig(&m.Meta)...)

	if len(m.Assets) == 0 {
		errs = append(errs, errors.New("assets must contain at least one asset"))
		return errs
	}

	seenSources := make(map[string]string)
	seenOutputs := make(map[string]string)
	for i, asset := range m.Assets {
		errs = append(errs, validateAsset(asset, i, cfg, seenSources, seenOutputs, m.Meta.Render.Tools)...)
	}

	// Sort errors alphabetically for stable output in tests and CLI feedback.
	sort.Slice(errs, func(i, j int) bool {
		return errs[i].Error() < errs[j].Error()
	})

	return errs
}

func validateMetaConfig(m *Meta) []error {
	var errs []error
	if strings.TrimSpace(m.Project) == "" {
		errs = append(errs, errors.New("meta.project is required"))
	}
	errs = append(errs, validateRenderConfig(m.Render)...)

	return errs
}

func validateAsset(
	asset Asset,
	idx int,
	cfg ValidationConfig,
	seenSources map[string]string,
	seenOutputs map[string]string,
	tools map[string]PipelineStep,
) []error {
	var errs []error
	ref := assetRef(asset, idx)
	source := canonicalPath(asset.Source)

	if source == "" {
		errs = append(errs, fmt.Errorf("%s: source is required", ref))
	} else {
		if first, ok := seenSources[source]; ok {
			errs = append(
				errs,
				fmt.Errorf("%s: duplicate source path %q (already used by %s)", ref, source, first),
			)
		} else {
			seenSources[source] = ref
		}
	}
	if len(asset.Outputs) == 0 {
		errs = append(errs, fmt.Errorf("%s: outputs must contain at least one output", ref))
	}

	if cfg.Strict {
		errs = append(errs, validateStrictLegal(ref, asset)...)
	}

	if source != "" {
		sourcePath := filepath.Join(cfg.BaseDir, source)
		if _, err := os.Stat(sourcePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: source file does not exist: %s", ref, source))
		}
	}

	for i, out := range asset.Outputs {
		errs = append(errs, validateOutput(ref, i, out, seenOutputs, tools)...)
	}

	return errs
}

func validateStrictLegal(assetRef string, asset Asset) []error {
	var errs []error
	if strings.TrimSpace(asset.Owner) == "" {
		errs = append(errs, fmt.Errorf("%s: owner is required in strict mode", assetRef))
	}
	if strings.TrimSpace(asset.Copyright) == "" {
		errs = append(errs, fmt.Errorf("%s: copyright is required in strict mode", assetRef))
	}
	if strings.TrimSpace(asset.License) == "" {
		errs = append(errs, fmt.Errorf("%s: license is required in strict mode", assetRef))
	}
	return errs
}

func validateOutput(
	assetRef string,
	idx int,
	out Output,
	seenOutputs map[string]string,
	tools map[string]PipelineStep,
) []error {
	var errs []error
	outputRef := fmt.Sprintf("%s output[%d]", assetRef, idx)
	outputPath := canonicalPath(out.Path)

	if outputPath == "" {
		errs = append(errs, fmt.Errorf("%s: path is required", outputRef))
	}

	if outputPath != "" {
		if first, ok := seenOutputs[outputPath]; ok {
			errs = append(
				errs,
				fmt.Errorf("%s: duplicate output path %q (already used by %s)", outputRef, outputPath, first),
			)
		} else {
			seenOutputs[outputPath] = outputRef
		}
	}

	if out.Width <= 0 {
		errs = append(errs, fmt.Errorf("%s: width must be > 0", outputRef))
	}
	if out.Height <= 0 {
		errs = append(errs, fmt.Errorf("%s: height must be > 0", outputRef))
	}

	if !validScaleMode(out.Options.ScaleMode) {
		errs = append(
			errs,
			fmt.Errorf("%s: options.scale_mode must be one of fit, fill, stretch, crop", outputRef),
		)
	}
	if !validBackground(out.Options.Background) {
		errs = append(
			errs,
			fmt.Errorf("%s: options.background must be transparent or #RRGGBB", outputRef),
		)
	}

	errs = append(
		errs,
		validateStagePreference(outputRef+" options.tools", out.Options.Tools, tools, true, true)...,
	)
	return errs
}

func assetRef(a Asset, idx int) string {
	if source := canonicalPath(a.Source); source != "" {
		return fmt.Sprintf("asset[%q]", source)
	}
	return fmt.Sprintf("asset[%d]", idx)
}

func validScaleMode(v string) bool {
	switch v {
	case scaleModeFit, "fill", "stretch", "crop":
		return true
	default:
		return false
	}
}

func validBackground(v string) bool {
	if v == backgroundTransparent {
		return true
	}
	return hexColorRe.MatchString(v)
}
