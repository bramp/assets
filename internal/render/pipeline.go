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

func ResolvePipeline(m *manifest.Manifest, o manifest.Output) ([]manifest.PipelineStep, error) {
	if len(o.Options.PipelineOverride) > 0 {
		return append([]manifest.PipelineStep(nil), o.Options.PipelineOverride...), nil
	}

	profileName := strings.TrimSpace(o.Options.Profile)
	if profileName == "" {
		profileName = strings.TrimSpace(m.Meta.Render.Defaults.Profile)
	}

	steps := make([]manifest.PipelineStep, 0)
	if profileName != "" {
		p, ok := m.Meta.Render.Profiles[profileName]
		if !ok {
			return nil, fmt.Errorf("render profile %q not found", profileName)
		}
		steps = append(steps, p.Pipeline...)
	}

	if len(o.Options.PipelineAppend) > 0 {
		steps = append(steps, o.Options.PipelineAppend...)
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no pipeline steps resolved for target %q", o.Path)
	}

	return steps, nil
}

func ExecutePipeline(steps []manifest.PipelineStep, ctx BuildContext) error {
	if err := os.MkdirAll(filepath.Dir(ctx.OutputPath), 0o755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "assets-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	ctx.TmpPath = filepath.Join(tmpDir, "stage1.tmp")
	ctx.Tmp2Path = filepath.Join(tmpDir, "stage2.tmp")

	for _, step := range steps {
		cmdText := expand(step.Command, ctx)
		cmd := exec.Command("sh", "-c", cmdText)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf("pipeline step %q failed: %w (output: %s)", step.Tool, runErr, strings.TrimSpace(string(out)))
		}
	}

	if _, err := os.Stat(ctx.OutputPath); err != nil {
		return fmt.Errorf("pipeline did not produce output %q: %w", ctx.OutputPath, err)
	}

	return nil
}

func expand(s string, ctx BuildContext) string {
	replacer := strings.NewReplacer(
		"{input}", shellQuote(ctx.InputPath),
		"{output}", shellQuote(ctx.OutputPath),
		"{tmp}", shellQuote(ctx.TmpPath),
		"{tmp2}", shellQuote(ctx.Tmp2Path),
		"{width}", fmt.Sprintf("%d", ctx.Width),
		"{height}", fmt.Sprintf("%d", ctx.Height),
		"{scale_mode}", shellQuote(ctx.ScaleMode),
		"{background}", shellQuote(ctx.Background),
		"{scale}", "1",
	)
	return replacer.Replace(s)
}

func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
