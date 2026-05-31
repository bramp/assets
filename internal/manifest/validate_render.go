package manifest

import (
	"errors"
	"fmt"
	"strings"
)

func validateRenderConfig(cfg RenderConfig) []error {
	var errs []error

	errs = append(errs, validateToolRegistry("meta.render.tools", cfg.Tools)...)
	errs = append(errs, validateStageOrder("meta.render.defaults.tools", cfg.Defaults.Tools, cfg.Tools)...)

	for ext, tool := range cfg.OptimizeByFormat {
		normExt := strings.TrimSpace(ext)
		if normExt == "" {
			errs = append(errs, errors.New("meta.render.optimize_by_format contains an empty extension key"))
			continue
		}
		if !strings.HasPrefix(normExt, ".") {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format extension %q must start with '.'", ext))
		}
		normTool := strings.TrimSpace(tool)
		if normTool == "" {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format[%q] must name an optimize tool", ext))
			continue
		}
		if _, ok := cfg.Tools[normTool]; !ok {
			errs = append(
				errs,
				fmt.Errorf("meta.render.optimize_by_format[%q] references unknown optimize tool %q", ext, normTool),
			)
		}
	}

	return errs
}

func validateStageOrder(prefix string, order ToolPreference, registry map[string]PipelineStep) []error {
	return validateStagePreference(prefix, order, registry, true, true)
}

func validateStagePreference(
	prefix string,
	pref ToolPreference,
	registry map[string]PipelineStep,
	allowAuto bool,
	allowDisable bool,
) []error {
	var errs []error
	for i, name := range pref {
		norm := strings.TrimSpace(name)
		if norm == "" {
			errs = append(errs, fmt.Errorf("%s[%d] must not be empty", prefix, i))
			continue
		}
		if allowAuto && strings.EqualFold(norm, "auto") {
			continue
		}
		if allowDisable && (strings.EqualFold(norm, "none") || strings.EqualFold(norm, "off")) {
			continue
		}
		if _, ok := registry[norm]; !ok {
			errs = append(errs, fmt.Errorf("%s[%d] %q does not exist in stage registry", prefix, i, name))
		}
	}
	return errs
}

func validSupportsFormat(v string) bool {
	if v == "*" {
		return true
	}
	return strings.HasPrefix(v, ".")
}

func validScaleModeValue(v string) bool {
	if v == "*" {
		return true
	}
	return validScaleMode(v)
}

func validatePipelineStepSupports(prefix string, step PipelineStep) []error {
	var errs []error
	for i, f := range step.Accepts {
		norm := strings.TrimSpace(f)
		if !validSupportsFormat(norm) {
			errs = append(errs, fmt.Errorf("%s.accepts[%d] %q must be '*' or extension like .png", prefix, i, f))
		}
	}
	for i, f := range step.Produces {
		norm := strings.TrimSpace(f)
		if !validSupportsFormat(norm) {
			errs = append(errs, fmt.Errorf("%s.produces[%d] %q must be '*' or extension like .png", prefix, i, f))
		}
	}
	return errs
}

func validatePipelineStepScaleModes(prefix string, step PipelineStep) []error {
	var errs []error
	for i, mode := range step.ScaleModes {
		norm := strings.TrimSpace(mode)
		if !validScaleModeValue(norm) {
			errs = append(
				errs,
				fmt.Errorf("%s.scale_modes[%d] %q must be '*' or one of fit, fill, stretch, crop", prefix, i, mode),
			)
		}
	}
	return errs
}

func validateStageRegistry(prefix string, registry map[string]PipelineStep) []error {
	var errs []error
	for name, step := range registry {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Errorf("%s contains an empty tool name", prefix))
			continue
		}
		if strings.TrimSpace(step.Tool) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: tool is required", prefix, name))
		}
		if strings.TrimSpace(step.Command) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: command is required", prefix, name))
		}
		errs = append(errs, validatePipelineStepSize(prefix, name, step)...)
		errs = append(errs, validatePipelineStepSupports(fmt.Sprintf("%s[%q]", prefix, name), step)...)
		errs = append(errs, validatePipelineStepScaleModes(fmt.Sprintf("%s[%q]", prefix, name), step)...)
	}
	return errs
}

func validateToolRegistry(prefix string, registry map[string]PipelineStep) []error {
	var errs []error
	for name, step := range registry {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Errorf("%s contains an empty tool name", prefix))
			continue
		}
		if len(step.Accepts) == 0 || len(step.Produces) == 0 {
			errs = append(errs, fmt.Errorf("%s[%q]: tools must define both accepts and produces", prefix, name))
		}
		errs = append(errs, validateStageRegistry(prefix, map[string]PipelineStep{name: step})...)
	}
	return errs
}

func validatePipelineStepSize(prefix, name string, step PipelineStep) []error {
	var errs []error
	hasSizeConfig := strings.TrimSpace(step.SizeTemplate) != "" || len(step.SizeByMode) > 0
	hasSizePlaceholder := strings.Contains(step.Command, "{size}")

	if hasSizeConfig && !hasSizePlaceholder {
		errs = append(
			errs,
			fmt.Errorf(
				"%s[%q]: size_template or size_by_mode is configured but command does not use {size}",
				prefix,
				name,
			),
		)
	}
	if hasSizePlaceholder && !hasSizeConfig {
		errs = append(
			errs,
			fmt.Errorf(
				"%s[%q]: command uses {size} but no size_template or size_by_mode is configured",
				prefix,
				name,
			),
		)
	}

	for mode, tmpl := range step.SizeByMode {
		normMode := strings.TrimSpace(mode)
		if normMode == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: size_by_mode contains an empty scale mode key", prefix, name))
			continue
		}
		if normMode != "*" && !validScaleModeValue(normMode) {
			errs = append(
				errs,
				fmt.Errorf(
					"%s[%q]: size_by_mode[%q] must be '*' or one of fit, fill, stretch, crop",
					prefix,
					name,
					mode,
				),
			)
		}
		if strings.TrimSpace(tmpl) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: size_by_mode[%q] must not be empty", prefix, name, mode))
		}
	}

	for i, arg := range step.VersionArgs {
		if strings.TrimSpace(arg) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: version_args[%d] must not be empty", prefix, name, i))
		}
	}

	return errs
}
