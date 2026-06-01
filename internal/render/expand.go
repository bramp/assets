package render

import (
	"strconv"
	"strings"
)

func expand(s string, ctx BuildContext) string {
	return expandTemplate(s, ctx)
}

// expandStepCommand expands a pipeline step using the same placeholder rules
// as execution, including tool-specific {size} expansion.
func expandStepCommand(step ResolvedStep, ctx BuildContext) string {
	size := expandTemplate(stepSizeTemplate(step, ctx.ScaleMode), ctx)
	command := strings.ReplaceAll(step.Command, "{size}", size)
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
		"{scale}", "1",
	)
	return replacer.Replace(s)
}

func sizeStrings(ctx BuildContext) (string, string) {
	return strconv.Itoa(ctx.Width), strconv.Itoa(ctx.Height)
}

func stepSizeTemplate(step ResolvedStep, scaleMode string) string {
	mode := strings.ToLower(strings.TrimSpace(scaleMode))
	if tmpl, ok := step.SizeByMode[mode]; ok && strings.TrimSpace(tmpl) != "" {
		return tmpl
	}
	return step.SizeTemplate
}

// shellQuote wraps a string for safe shell embedding in the generated command
// text.
func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
