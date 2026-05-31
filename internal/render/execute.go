package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

// ExecutePipeline runs all resolved pipeline steps with default command hook behavior.
func ExecutePipeline(steps []manifest.PipelineStep, ctx BuildContext) error {
	return ExecutePipelineWithHook(steps, ctx, nil)
}

// ExecutePipelineWithHook runs all resolved steps and optionally reports command text.
func ExecutePipelineWithHook(steps []manifest.PipelineStep, ctx BuildContext, onCommand func(string)) error {
	if err := os.MkdirAll(filepath.Dir(ctx.OutputPath), outputDirPerm); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "assets-build-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx.TmpPath = filepath.Join(tmpDir, "stage1")
	ctx.Tmp2Path = filepath.Join(tmpDir, "stage2")
	currentInput := ctx.InputPath
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(ctx.OutputPath)))

	for i, step := range steps {
		stepCtx := plannedStepContext(ctx, steps, i, currentInput, outputExt)
		if err := ensureFileExistsAndNonEmpty(stepCtx.InputPath); err != nil {
			return fmt.Errorf("pipeline step %q input %q invalid: %w", step.Tool, stepCtx.InputPath, err)
		}

		cmdText := expandStepCommand(step, stepCtx)
		if onCommand != nil {
			onCommand(cmdText)
		}
		cmd := exec.CommandContext(context.Background(), "sh", "-c", cmdText)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf(
				"pipeline step %q failed: %w (output: %s)",
				step.Tool,
				runErr,
				strings.TrimSpace(string(out)),
			)
		}
		if err := ensureFileExistsAndNonEmpty(stepCtx.OutputPath); err != nil {
			return fmt.Errorf("pipeline step %q did not produce output %q: %w", step.Tool, stepCtx.OutputPath, err)
		}
		currentInput = stepCtx.OutputPath
	}

	if err := ensureFileExistsAndNonEmpty(ctx.OutputPath); err != nil {
		return fmt.Errorf("pipeline did not produce output %q: %w", ctx.OutputPath, err)
	}

	return nil
}

func ensureFileExistsAndNonEmpty(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Size() <= 0 {
		return errors.New("size must be > 0 bytes")
	}
	// TODO(bramp): Validate file content against expected media format after each step.
	return nil
}

// PlannedCommands expands pipeline steps into the exact command strings that
// would be executed, using the same input/output/tmp chaining behavior as
// ExecutePipeline but without running any commands.
func PlannedCommands(steps []manifest.PipelineStep, ctx BuildContext) []string {
	commands := make([]string, 0, len(steps))
	currentInput := ctx.InputPath
	outputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(ctx.OutputPath)))

	for i, step := range steps {
		stepCtx := plannedStepContext(ctx, steps, i, currentInput, outputExt)
		commands = append(commands, expandStepCommand(step, stepCtx))
		currentInput = stepCtx.OutputPath
	}

	return commands
}

// plannedStepContext computes the per-step input/output pair used by both
// execution and dry-run planning so the two code paths stay identical.
func plannedStepContext(
	ctx BuildContext,
	steps []manifest.PipelineStep,
	index int,
	currentInput string,
	outputExt string,
) BuildContext {
	stepCtx := ctx
	stepCtx.InputPath = currentInput

	step := steps[index]
	var nextStep *manifest.PipelineStep
	if index+1 < len(steps) {
		nextStep = &steps[index+1]
	}

	switch {
	case index == len(steps)-1:
		stepCtx.OutputPath = ctx.OutputPath
	case index%2 == 0:
		stepCtx.OutputPath = stepTempPath(ctx.TmpPath, step, currentInput, outputExt, nextStep)
	default:
		stepCtx.OutputPath = stepTempPath(ctx.Tmp2Path, step, currentInput, outputExt, nextStep)
	}

	return stepCtx
}

// stepTempPath chooses the next temporary file path for a pipeline stage,
// preserving the intended extension when the step changes formats.
func stepTempPath(
	basePath string,
	step manifest.PipelineStep,
	currentInput string,
	finalOutputExt string,
	nextStep *manifest.PipelineStep,
) string {
	ext := stepOutputExt(step, currentInput, finalOutputExt, nextStep)
	if ext == "" {
		return basePath
	}
	return strings.TrimSuffix(basePath, filepath.Ext(basePath)) + ext
}

// stepOutputExt picks the most appropriate extension for the next temporary
// file based on the current step, the current input, and the final target.
func stepOutputExt(
	step manifest.PipelineStep,
	currentInput string,
	finalOutputExt string,
	nextStep *manifest.PipelineStep,
) string {
	produced := producedFormats(step.Produces, finalOutputExt)
	if len(produced) == 0 {
		return strings.ToLower(strings.TrimSpace(filepath.Ext(currentInput)))
	}
	if len(produced) == 1 {
		return produced[0]
	}

	if nextStep != nil {
		for _, ext := range produced {
			if ext == finalOutputExt && matchesFormatList(nextStep.Accepts, ext) {
				return ext
			}
		}
		for _, ext := range produced {
			if matchesFormatList(nextStep.Accepts, ext) {
				return ext
			}
		}
	}

	inputExt := strings.ToLower(strings.TrimSpace(filepath.Ext(currentInput)))
	for _, ext := range produced {
		if ext == inputExt {
			return ext
		}
	}
	for _, ext := range produced {
		if ext == finalOutputExt {
			return ext
		}
	}
	return produced[0]
}
