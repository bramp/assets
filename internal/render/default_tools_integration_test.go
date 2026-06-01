//go:build integration

package render

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

func TestBuiltinDefaultTools_ExecuteEveryConfiguration_RealBinaries(t *testing.T) {
	testManifestYAML := `meta:
  project: "default-tool-matrix-real"
assets: []
`

	m, err := manifest.Load(strings.NewReader(testManifestYAML))
	if err != nil {
		t.Fatalf("load manifest with built-in defaults: %v", err)
	}

	missing := missingBinaries(m.Meta.Render.Tools)
	requireAll := strings.EqualFold(strings.TrimSpace(os.Getenv("ASSETS_INTEGRATION_REQUIRE_ALL")), "1")
	if len(missing) > 0 && requireAll {
		t.Fatalf(
			"missing required binaries for integration run: %s",
			strings.Join(missing, ", "),
		)
	}
	if len(missing) > 0 {
		t.Logf("skipping tools with missing binaries: %s", strings.Join(missing, ", "))
	}

	workDir := t.TempDir()
	toolNames := sortedAvailableToolNames(m.Meta.Render.Tools)
	if len(toolNames) == 0 {
		t.Skip("no default tools available on PATH")
	}

	for _, toolName := range toolNames {
		toolName := toolName
		step := m.Meta.Render.Tools[toolName]

		for _, mode := range modesForStep(step) {
			mode := mode
			t.Run(fmt.Sprintf("%s/%s", toolName, mode), func(t *testing.T) {
				inputExt := firstConcreteExt(step.Accepts)
				outputExt := firstConcreteExt(step.Produces)
				if outputExt == "" {
					outputExt = inputExt
				}

				inputPath := filepath.Join(workDir, "inputs", fmt.Sprintf("%s-%s%s", toolName, mode, inputExt))
				outputPath := filepath.Join(workDir, "outputs", fmt.Sprintf("%s-%s%s", toolName, mode, outputExt))

				if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
					t.Fatalf("mkdir input dir: %v", err)
				}
				if err := writeFixtureForExt(inputPath, inputExt); err != nil {
					t.Fatalf("write fixture %q: %v", inputExt, err)
				}

				ctx := BuildContext{
					InputPath:  inputPath,
					OutputPath: outputPath,
					Width:      128,
					Height:     96,
					ScaleMode:  mode,
					Background: "transparent",
				}
				resolved := resolveStepForTest(toolName, step, inputExt, outputExt)

				command := expandStepCommand(resolved, ctx)
				if unresolvedPlaceholderRE.MatchString(command) {
					t.Fatalf("unresolved placeholder in command %q", command)
				}

				if err := ExecutePipeline([]ResolvedStep{resolved}, ctx); err != nil {
					t.Fatalf("execute step failed (tool=%s mode=%s): %v", toolName, mode, err)
				}
			})
		}
	}
}

func TestBuiltinDefaultTools_VersionProbeEveryAvailableTool(t *testing.T) {
	testManifestYAML := `meta:
  project: "default-tool-version-probe-real"
assets: []
`

	m, err := manifest.Load(strings.NewReader(testManifestYAML))
	if err != nil {
		t.Fatalf("load manifest with built-in defaults: %v", err)
	}

	missing := missingBinaries(m.Meta.Render.Tools)
	if len(missing) > 0 {
		t.Logf("skipping tools with missing binaries: %s", strings.Join(missing, ", "))
	}

	toolNames := sortedAvailableToolNames(m.Meta.Render.Tools)
	if len(toolNames) == 0 {
		t.Skip("no default tools available on PATH")
	}

	repo := NewToolRepository()
	for _, toolName := range toolNames {
		toolName := toolName
		step := m.Meta.Render.Tools[toolName]

		t.Run(toolName, func(t *testing.T) {
			resolved := ResolvedStep{Tool: step.Tool, VersionArgs: step.VersionArgs}
			if got := strings.TrimSpace(repo.Version(resolved)); got == "" {
				t.Fatalf("expected non-empty version probe result for default tool %q (binary=%q)", toolName, step.Tool)
			}
		})
	}
}

func missingBinaries(tools map[string]manifest.ToolSpec) []string {
	uniq := map[string]struct{}{}
	for _, step := range tools {
		name := firstCommandToken(step.Tool)
		if name == "" {
			continue
		}
		uniq[name] = struct{}{}
	}

	missing := make([]string, 0, len(uniq))
	for bin := range uniq {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedAvailableToolNames(tools map[string]manifest.ToolSpec) []string {
	out := make([]string, 0, len(tools))
	for name, step := range tools {
		bin := firstCommandToken(step.Tool)
		if bin == "" {
			continue
		}
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func writeFixtureForExt(path string, ext string) error {
	normExt := strings.ToLower(strings.TrimSpace(ext))
	switch normExt {
	case ".svg":
		return os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="4" height="4"><rect width="4" height="4" fill="#ff0000"/></svg>`), 0o644)
	case ".png", ".jpg", ".jpeg", ".gif":
		return writeRasterFixture(path, normExt)
	case ".webp":
		b, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoIAAIAAkA4JaQAA3AA/vuUAAA=")
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0o644)
	default:
		return os.WriteFile(path, []byte("fixture\n"), 0o644)
	}
}

func writeRasterFixture(path string, ext string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(60 * x), G: uint8(60 * y), B: 180, A: 255})
		}
	}

	switch ext {
	case ".png":
		return png.Encode(f, img)
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	case ".gif":
		return gif.Encode(f, img, nil)
	default:
		return fmt.Errorf("unsupported raster fixture extension %q", ext)
	}
}
