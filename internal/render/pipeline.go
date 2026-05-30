package render

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

type TargetSpec struct {
	Asset  manifest.Asset
	Output manifest.Output
}

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

type ResolveOptions struct {
	// CheckAvailability controls whether unavailable tools are filtered out.
	// Defaults to true in ResolvePipeline.
	CheckAvailability bool
}

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

func ResolvePipeline(m *manifest.Manifest, sourcePath string, o manifest.Output) ([]manifest.PipelineStep, error) {
	return ResolvePipelineWithOptions(m, sourcePath, o, ResolveOptions{CheckAvailability: true})
}

func ResolvePipelineWithOptions(m *manifest.Manifest, sourcePath string, o manifest.Output, opts ResolveOptions) ([]manifest.PipelineStep, error) {
	sourceExt := strings.ToLower(strings.TrimSpace(filepath.Ext(sourcePath)))
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(o.Path)))
	order := buildGraphPreferenceOrder(m, o)
	steps, err := resolveGraphPath(m.Meta.Render.Tools, order, sourceExt, outputExt, o.Options.ScaleMode, opts)
	if err != nil {
		return nil, err
	}
	// TODO(bramp): Model final optimization as an explicit graph node/state so
	// terminal optimization is selected during path resolution instead of appended
	// after graph traversal.
	steps, err = appendTerminalOptimizer(m.Meta.Render, steps, outputExt, opts)
	if err != nil {
		return nil, err
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no pipeline steps resolved for target %q", o.Path)
	}

	return steps, nil
}

func appendTerminalOptimizer(cfg manifest.RenderConfig, steps []manifest.PipelineStep, outputExt string, opts ResolveOptions) ([]manifest.PipelineStep, error) {
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
		return nil, fmt.Errorf("optimizer %q configured for %q not found in render tools", normOptimizeStepName, normExt)
	}
	if !matchesFormatList(optimizeStep.Accepts, normExt) || !matchesFormatList(optimizeStep.Produces, normExt) {
		return nil, fmt.Errorf("optimizer %q configured for %q must accept and produce %q", normOptimizeStepName, normExt, normExt)
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

func samePipelineStep(a manifest.PipelineStep, b manifest.PipelineStep) bool {
	return strings.TrimSpace(a.Tool) == strings.TrimSpace(b.Tool) && strings.TrimSpace(a.Command) == strings.TrimSpace(b.Command)
}

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

func buildGraphPreferenceOrder(m *manifest.Manifest, o manifest.Output) []string {
	return buildPreferenceOrder(o.Options.Tools, m.Meta.Render.Defaults.Tools)
}

func resolveGraphPath(tools map[string]manifest.PipelineStep, order []string, sourceExt string, outputExt string, scaleMode string, opts ResolveOptions) ([]manifest.PipelineStep, error) {
	if sourceExt == "" || outputExt == "" {
		return nil, fmt.Errorf("unable to resolve conversion path for empty source/output format")
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

func buildAvailabilityChecker(opts ResolveOptions) func(string) bool {
	if !opts.CheckAvailability {
		return func(string) bool { return true }
	}
	return binaryAvailable
}

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

func supportsFormat(supports []string, formatExt string) bool {
	if len(supports) == 0 {
		return true
	}
	if formatExt == "" {
		return false
	}

	normFormat := strings.ToLower(strings.TrimSpace(formatExt))
	for _, s := range supports {
		norm := strings.ToLower(strings.TrimSpace(s))
		if norm == "*" || norm == normFormat {
			return true
		}
	}
	return false
}

func binaryAvailable(toolName string) bool {
	binary := firstCommandToken(toolName)
	if binary == "" {
		return false
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

func ExecutePipeline(steps []manifest.PipelineStep, ctx BuildContext) error {
	return ExecutePipelineWithHook(steps, ctx, nil)
}

func ExecutePipelineWithHook(steps []manifest.PipelineStep, ctx BuildContext, onCommand func(string)) error {
	if err := os.MkdirAll(filepath.Dir(ctx.OutputPath), 0o755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "assets-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	stageExt := strings.ToLower(strings.TrimSpace(filepath.Ext(ctx.OutputPath)))
	if stageExt == "" {
		stageExt = ".tmp"
	}
	ctx.TmpPath = filepath.Join(tmpDir, "stage1"+stageExt)
	ctx.Tmp2Path = filepath.Join(tmpDir, "stage2"+stageExt)
	currentInput := ctx.InputPath

	for i, step := range steps {
		stepCtx := ctx
		stepCtx.InputPath = currentInput
		if i == len(steps)-1 {
			stepCtx.OutputPath = ctx.OutputPath
		} else {
			if i%2 == 0 {
				stepCtx.OutputPath = ctx.TmpPath
			} else {
				stepCtx.OutputPath = ctx.Tmp2Path
			}
		}

		cmdText := expandStepCommand(step, stepCtx)
		if onCommand != nil {
			onCommand(cmdText)
		}
		cmd := exec.Command("sh", "-c", cmdText)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf("pipeline step %q failed: %w (output: %s)", step.Tool, runErr, strings.TrimSpace(string(out)))
		}
		currentInput = stepCtx.OutputPath
	}

	if _, err := os.Stat(ctx.OutputPath); err != nil {
		return fmt.Errorf("pipeline did not produce output %q: %w", ctx.OutputPath, err)
	}

	return nil
}

// PlannedCommands expands pipeline steps into the exact command strings that
// would be executed, using the same input/output/tmp chaining behavior as
// ExecutePipeline but without running any commands.
func PlannedCommands(steps []manifest.PipelineStep, ctx BuildContext) []string {
	commands := make([]string, 0, len(steps))
	currentInput := ctx.InputPath

	for i, step := range steps {
		stepCtx := ctx
		stepCtx.InputPath = currentInput
		if i == len(steps)-1 {
			stepCtx.OutputPath = ctx.OutputPath
		} else if i%2 == 0 {
			stepCtx.OutputPath = ctx.TmpPath
		} else {
			stepCtx.OutputPath = ctx.Tmp2Path
		}

		commands = append(commands, expandStepCommand(step, stepCtx))
		currentInput = stepCtx.OutputPath
	}

	return commands
}

func expand(s string, ctx BuildContext) string {
	return expandWithSetSize(s, ctx, "")
}

func expandStepCommand(step manifest.PipelineStep, ctx BuildContext) string {
	setSize := ""
	if strings.TrimSpace(step.SetsSize) != "" {
		setSize = expandWithSetSize(step.SetsSize, ctx, "")
	}
	return expandStep(step.Command, ctx, setSize)
}

func expandWithSetSize(s string, ctx BuildContext, setSize string) string {
	return expandStep(s, ctx, setSize)
}

func expandStep(s string, ctx BuildContext, setSize string) string {
	replacer := strings.NewReplacer(
		"{input}", shellQuote(ctx.InputPath),
		"{output}", shellQuote(ctx.OutputPath),
		"{tmp}", shellQuote(ctx.TmpPath),
		"{tmp2}", shellQuote(ctx.Tmp2Path),
		"{width}", fmt.Sprintf("%d", ctx.Width),
		"{height}", fmt.Sprintf("%d", ctx.Height),
		"{WIDTH}", fmt.Sprintf("%d", ctx.Width),
		"{HEIGHT}", fmt.Sprintf("%d", ctx.Height),
		"{scale_mode}", shellQuote(ctx.ScaleMode),
		"{background}", shellQuote(ctx.Background),
		"{sets_size}", setSize,
		"{resize_args}", resizeArgs(ctx),
		"{scale}", "1",
	)
	return replacer.Replace(s)
}

func resizeArgs(ctx BuildContext) string {
	width := fmt.Sprintf("%d", ctx.Width)
	height := fmt.Sprintf("%d", ctx.Height)
	bg := ctx.Background
	if strings.TrimSpace(bg) == "" {
		bg = "transparent"
	}

	switch strings.ToLower(strings.TrimSpace(ctx.ScaleMode)) {
	case "fill", "crop":
		return fmt.Sprintf("-resize %sx%s^ -gravity center -extent %sx%s", width, height, width, height)
	case "stretch":
		return fmt.Sprintf("-resize %sx%s!", width, height)
	case "fit", "":
		fallthrough
	default:
		return fmt.Sprintf("-resize %sx%s -background %s -gravity center -extent %sx%s", width, height, shellQuote(bg), width, height)
	}
}

func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
