package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bramp/assets/internal/manifest"
)

func expand(s string, ctx BuildContext) string {
	return expandTemplate(s, ctx)
}

// expandStepCommand expands a pipeline step using the same placeholder rules
// as execution, including any step-specific {sets_size} expansion.
func expandStepCommand(step manifest.PipelineStep, ctx BuildContext) string {
	command := step.Command
	if strings.TrimSpace(step.SetsSize) != "" {
		command = strings.ReplaceAll(command, "{sets_size}", expandTemplate(step.SetsSize, ctx))
	}
	return expandTemplate(command, ctx)
}

// expandTemplate replaces supported placeholders in a command template.
func expandTemplate(s string, ctx BuildContext) string {
	width, height := sizeStrings(ctx)
	replacer := strings.NewReplacer(
		"{input}", shellQuote(ctx.InputPath),
		"{output}", shellQuote(ctx.OutputPath),
		"{tmp}", shellQuote(ctx.TmpPath),
		"{tmp2}", shellQuote(ctx.Tmp2Path),
		"{width}", width,
		"{height}", height,
		"{scale_mode}", shellQuote(ctx.ScaleMode),
		"{background}", shellQuote(ctx.Background),
		"{resize_args}", resizeArgs(ctx, width, height),
		"{scale}", "1",
	)
	return replacer.Replace(s)
}

func sizeStrings(ctx BuildContext) (string, string) {
	return strconv.Itoa(ctx.Width), strconv.Itoa(ctx.Height)
}

func resizeArgs(ctx BuildContext, width string, height string) string {
	bg := ctx.Background
	if strings.TrimSpace(bg) == "" {
		bg = defaultBackgroundColor
	}

	switch strings.ToLower(strings.TrimSpace(ctx.ScaleMode)) {
	case "fill", "crop":
		return fmt.Sprintf("-resize %sx%s^ -gravity center -extent %sx%s", width, height, width, height)
	case "stretch":
		return fmt.Sprintf("-resize %sx%s!", width, height)
	case scaleModeFit, "":
		fallthrough
	default:
		return fmt.Sprintf(
			"-resize %sx%s -background %s -gravity center -extent %sx%s",
			width,
			height,
			shellQuote(bg),
			width,
			height,
		)
	}
}

// shellQuote wraps a string for safe shell embedding in the generated command
// text.
func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
