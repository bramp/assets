package render

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

// appendTerminalOptimizer appends a format-specific optimizer as the final
// pipeline step when configured.
//
// This keeps optimization policy in the manifest (not hardcoded in planning)
// and validates that the chosen optimizer is format-safe and available.
func appendTerminalOptimizer(
	cfg manifest.RenderConfig,
	steps []manifest.PipelineStep,
	outputExt string,
	opts ResolveOptions,
) ([]manifest.PipelineStep, error) {
	normExt := strings.ToLower(strings.TrimSpace(outputExt))
	if normExt == "" || len(cfg.OptimizeByFormat) == 0 {
		return steps, nil
	}

	optimizeStepName, ok := cfg.OptimizeByFormat[normExt]
	if !ok {
		return steps, nil
	}
	normOptimizeStepName := strings.TrimSpace(optimizeStepName)
	if normOptimizeStepName == "" {
		return steps, nil
	}

	optimizeStep, ok := cfg.Tools[normOptimizeStepName]
	if !ok {
		return nil, fmt.Errorf(
			"optimizer %q configured for %q not found in render tools",
			normOptimizeStepName,
			normExt,
		)
	}
	if !matchesFormatList(optimizeStep.Accepts, normExt) || !matchesFormatList(optimizeStep.Produces, normExt) {
		return nil, fmt.Errorf(
			"optimizer %q configured for %q must accept and produce %q",
			normOptimizeStepName,
			normExt,
			normExt,
		)
	}

	if len(steps) > 0 && samePipelineStep(steps[len(steps)-1], optimizeStep) {
		return steps, nil
	}

	toolAvailable := buildAvailabilityChecker(opts)
	if !toolAvailable(optimizeStep.Tool) {
		return nil, fmt.Errorf("optimizer tool %q for %q is not available", optimizeStep.Tool, normExt)
	}

	return append(steps, optimizeStep), nil
}

// samePipelineStep compares only executable identity (tool + command) to avoid
// appending duplicate terminal optimizer steps.
func samePipelineStep(a manifest.PipelineStep, b manifest.PipelineStep) bool {
	return strings.TrimSpace(a.Tool) == strings.TrimSpace(b.Tool) &&
		strings.TrimSpace(a.Command) == strings.TrimSpace(b.Command)
}

// supportsScaleMode reports whether a step can satisfy the requested mode.
// Empty mode or empty capability list is treated as permissive for backward
// compatibility with tools that do not model mode constraints.
func supportsScaleMode(supported []string, mode string) bool {
	normMode := strings.ToLower(strings.TrimSpace(mode))
	if normMode == "" || len(supported) == 0 {
		return true
	}
	for _, m := range supported {
		norm := strings.ToLower(strings.TrimSpace(m))
		if norm == "*" || norm == normMode {
			return true
		}
	}
	return false
}

// resolveGraphPath finds a compatible conversion path from sourceExt to
// outputExt using a bounded breadth-first search.
//
// Why: shortest-path planning gives deterministic, easy-to-reason-about
// pipelines while still allowing preference-based tie-breaking.
//
//nolint:funlen,gocognit // Path search weighs availability, format compatibility, and preferences.
func resolveGraphPath(
	tools map[string]manifest.PipelineStep,
	order []string,
	sourceExt string,
	outputExt string,
	scaleMode string,
	opts ResolveOptions,
) ([]manifest.PipelineStep, error) {
	if sourceExt == "" || outputExt == "" {
		return nil, errors.New("unable to resolve conversion path for empty source/output format")
	}
	toolAvailable := buildAvailabilityChecker(opts)
	maxDepth := 4
	preferenceRank := make(map[string]int, len(order))
	for i, n := range order {
		norm := strings.ToLower(strings.TrimSpace(n))
		if norm == "" {
			continue
		}
		if _, exists := preferenceRank[norm]; !exists {
			preferenceRank[norm] = i
		}
	}

	type pathState struct {
		format string
		tools  []string
	}
	queue := []pathState{{format: sourceExt, tools: nil}}
	best := make(map[string]int)
	best[sourceExt] = 0
	var solutions [][]string

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		if len(state.tools) >= maxDepth {
			continue
		}

		for name, step := range tools {
			normName := strings.ToLower(strings.TrimSpace(name))
			if normName == "" || !supportsScaleMode(step.ScaleModes, scaleMode) {
				continue
			}
			if !toolAvailable(step.Tool) {
				continue
			}
			if normName == "none" || normName == "off" {
				continue
			}
			if !matchesFormatList(step.Accepts, state.format) {
				continue
			}

			for _, produced := range producedFormats(step.Produces, outputExt) {
				if produced == "" {
					continue
				}
				nextTools := append(append([]string(nil), state.tools...), normName)
				if produced == outputExt {
					solutions = append(solutions, nextTools)
					continue
				}
				depth := len(nextTools)
				if prev, ok := best[produced]; ok && depth >= prev {
					continue
				}
				best[produced] = depth
				queue = append(queue, pathState{format: produced, tools: nextTools})
			}
		}
	}

	if len(solutions) == 0 {
		return nil, fmt.Errorf("no compatible conversion path from %q to %q", sourceExt, outputExt)
	}

	bestPath := solutions[0]
	bestScore := graphPathScore(bestPath, preferenceRank)
	for _, p := range solutions[1:] {
		s := graphPathScore(p, preferenceRank)
		if s < bestScore {
			bestPath = p
			bestScore = s
		}
	}

	resolved := make([]manifest.PipelineStep, 0, len(bestPath))
	for _, name := range bestPath {
		step, ok := tools[name]
		if !ok {
			continue
		}
		resolved = append(resolved, step)
	}
	return resolved, nil
}

// buildAvailabilityChecker returns a tool-availability predicate honoring
// ResolveOptions.CheckAvailability.
//
// Why: generation and tests can be host-agnostic, while execution paths can
// enforce that required binaries exist.
func buildAvailabilityChecker(opts ResolveOptions) func(string) bool {
	if !opts.CheckAvailability {
		return func(string) bool { return true }
	}
	return binaryAvailable
}

// firstCommandToken extracts the executable token from a tool declaration.
// Tool entries may include arguments, but availability checks need only the
// binary name.
func firstCommandToken(toolName string) string {
	binary := strings.TrimSpace(toolName)
	if binary == "" {
		return ""
	}
	parts := strings.Fields(binary)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// producedFormats normalizes a tool's produces list into concrete extensions.
// A wildcard produce entry resolves to the requested output extension so path
// search can reason about exact intermediate formats.
func producedFormats(produces []string, outputExt string) []string {
	if len(produces) == 0 {
		return nil
	}
	result := make([]string, 0, len(produces))
	for _, p := range produces {
		norm := strings.ToLower(strings.TrimSpace(p))
		if norm == "" {
			continue
		}
		if norm == "*" {
			if outputExt != "" {
				result = append(result, outputExt)
			}
			continue
		}
		result = append(result, norm)
	}
	return result
}

// matchesFormatList reports whether format is accepted by list, honoring
// wildcard entries and normalized extension matching.
func matchesFormatList(list []string, format string) bool {
	if len(list) == 0 || format == "" {
		return false
	}
	normFormat := strings.ToLower(strings.TrimSpace(format))
	for _, v := range list {
		norm := strings.ToLower(strings.TrimSpace(v))
		if norm == "*" || norm == normFormat {
			return true
		}
	}
	return false
}

// graphPathScore ranks candidate paths by favoring shorter chains first and
// then applying preference order as a deterministic tie-breaker.
func graphPathScore(path []string, pref map[string]int) int {
	score := len(path) * 1000
	for i, n := range path {
		rank, ok := pref[n]
		if !ok {
			rank = 999
		}
		score += rank * (10 + i)
	}
	return score
}

// buildPreferenceOrder computes the effective per-output preference order.
//
// Why: output-level preferences can embed "auto" to splice defaults at a
// specific position rather than replacing defaults entirely.
func buildPreferenceOrder(outputPref manifest.ToolPreference, defaultPref manifest.ToolPreference) []string {
	if len(outputPref) == 0 {
		return append([]string(nil), defaultPref...)
	}

	order := make([]string, 0, len(outputPref)+len(defaultPref))
	for _, item := range outputPref {
		norm := strings.TrimSpace(item)
		if strings.EqualFold(norm, "auto") {
			order = append(order, defaultPref...)
			continue
		}
		order = append(order, item)
	}

	return order
}

// binaryAvailable reports whether a tool executable can be resolved on PATH.
// It checks only the command token so tool declarations can remain flexible.
func binaryAvailable(toolName string) bool {
	binary := firstCommandToken(toolName)
	if binary == "" {
		return false
	}
	_, err := exec.LookPath(binary)
	return err == nil
}
