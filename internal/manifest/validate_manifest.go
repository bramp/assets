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

	errs = append(errs, validateProjectAndRender(m)...)

	if len(m.Assets) == 0 {
		errs = append(errs, errors.New("assets must contain at least one asset"))
		return errs
	}

	seenOutputs := make(map[string]string)
	for i, asset := range m.Assets {
		errs = append(errs, validateAsset(asset, i, cfg, seenOutputs, m.Meta.Render.Tools)...)
	}

	sort.Slice(errs, func(i, j int) bool {
		return errs[i].Error() < errs[j].Error()
	})

	return errs
}

func validateProjectAndRender(m *Manifest) []error {
	var errs []error
	if strings.TrimSpace(m.Meta.Project) == "" {
		errs = append(errs, errors.New("meta.project is required"))
	}
	errs = append(errs, validateRenderConfig(m.Meta.Render)...)
	return errs
}

func validateAsset(
	asset Asset,
	idx int,
	cfg ValidationConfig,
	seenOutputs map[string]string,
	tools map[string]PipelineStep,
) []error {
	var errs []error
	ref := assetRef(asset, idx)

	if strings.TrimSpace(asset.ID) == "" {
		errs = append(errs, fmt.Errorf("%s: id is required", ref))
	}
	if strings.TrimSpace(asset.Source) == "" {
		errs = append(errs, fmt.Errorf("%s: source is required", ref))
	}
	if len(asset.Outputs) == 0 {
		errs = append(errs, fmt.Errorf("%s: outputs must contain at least one output", ref))
	}

	if cfg.Strict {
		errs = append(errs, validateStrictLegal(ref, asset)...)
	}

	sourcePath := filepath.Join(cfg.BaseDir, asset.Source)
	if _, err := os.Stat(sourcePath); err != nil {
		errs = append(errs, fmt.Errorf("%s: source file does not exist: %s", ref, asset.Source))
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

	if strings.TrimSpace(out.Path) == "" {
		errs = append(errs, fmt.Errorf("%s: path is required", outputRef))
	}

	if out.Path != "" {
		if first, ok := seenOutputs[out.Path]; ok {
			errs = append(
				errs,
				fmt.Errorf("%s: duplicate output path %q (already used by %s)", outputRef, out.Path, first),
			)
		} else {
			seenOutputs[out.Path] = outputRef
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
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Sprintf("asset[%d]", idx)
	}
	return fmt.Sprintf("asset[%q]", a.ID)
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
