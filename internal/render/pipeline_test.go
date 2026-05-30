package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

func TestResolvePipeline_PreferenceTieBreak(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Tools: manifest.ToolPreference{"resvg"}},
				Tools: map[string]manifest.PipelineStep{
					"resvg": {
						Tool:     "resvg",
						Accepts:  []string{".svg"},
						Produces: []string{".png"},
						Command:  "echo resvg > {output}",
					},
					"inkscape": {
						Tool:     "inkscape",
						Accepts:  []string{".svg"},
						Produces: []string{".png"},
						Command:  "echo inkscape > {output}",
					},
				},
			},
		},
	}

	steps, err := ResolvePipelineWithOptions(m, "raw/logo.svg", manifest.Output{Path: "out/logo.png"}, ResolveOptions{CheckAvailability: false})
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected single-step path, got %+v", steps)
	}
	if !strings.Contains(steps[0].Command, "resvg") {
		t.Fatalf("expected defaults.tools tie-break to prefer resvg, got %q", steps[0].Command)
	}
}

func TestResolvePipeline_ToolAvailabilityMatrix(t *testing.T) {
	t.Parallel()

	newManifest := func(primaryTool string, fallbackTool string) *manifest.Manifest {
		return &manifest.Manifest{
			Meta: manifest.Meta{
				Render: manifest.RenderConfig{
					Defaults: manifest.RenderDefaults{Tools: manifest.ToolPreference{"resvg", "inkscape"}},
					Tools: map[string]manifest.PipelineStep{
						"resvg": {
							Tool:     primaryTool,
							Accepts:  []string{".svg"},
							Produces: []string{".png"},
							Command:  "echo resvg > {output}",
						},
						"inkscape": {
							Tool:     fallbackTool,
							Accepts:  []string{".svg"},
							Produces: []string{".png"},
							Command:  "echo inkscape > {output}",
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name              string
		manifest          *manifest.Manifest
		checkAvailability bool
		wantTool          string
		wantErrLike       string
	}{
		{
			name:              "preferred_available_picks_preferred",
			manifest:          newManifest("sh", "cp"),
			checkAvailability: true,
			wantTool:          "sh",
		},
		{
			name:              "preferred_unavailable_falls_back_to_available",
			manifest:          newManifest("definitely-missing-binary", "sh"),
			checkAvailability: true,
			wantTool:          "sh",
		},
		{
			name:              "all_unavailable_errors",
			manifest:          newManifest("definitely-missing-binary", "definitely-also-missing"),
			checkAvailability: true,
			wantErrLike:       "no compatible conversion path",
		},
		{
			name:              "availability_disabled_ignores_missing_and_keeps_preference",
			manifest:          newManifest("definitely-missing-binary", "sh"),
			checkAvailability: false,
			wantTool:          "definitely-missing-binary",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			steps, err := ResolvePipelineWithOptions(tc.manifest, "raw/logo.svg", manifest.Output{Path: "out/logo.png"}, ResolveOptions{CheckAvailability: tc.checkAvailability})
			if tc.wantErrLike != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrLike) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrLike, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve pipeline: %v", err)
			}
			if len(steps) != 1 {
				t.Fatalf("expected single-step path, got %+v", steps)
			}
			if steps[0].Tool != tc.wantTool {
				t.Fatalf("unexpected selected tool: got=%q want=%q", steps[0].Tool, tc.wantTool)
			}
		})
	}
}

func TestFindTarget(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Assets: []manifest.Asset{{
			ID:     "a",
			Source: "raw/in.txt",
			Outputs: []manifest.Output{{
				Path: "out/a.txt",
			}},
		}},
	}

	spec, err := FindTarget(m, "out/a.txt")
	if err != nil {
		t.Fatalf("find target: %v", err)
	}
	if spec.Asset.ID != "a" || spec.Output.Path != "out/a.txt" {
		t.Fatalf("unexpected target spec: %+v", spec)
	}

	if _, err := FindTarget(m, "out/missing.txt"); err == nil {
		t.Fatal("expected missing target error")
	}
}

func TestExecutePipeline_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := filepath.Join(dir, "in.txt")
	output := filepath.Join(dir, "nested", "out.txt")
	if err := os.WriteFile(input, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	ctx := BuildContext{InputPath: input, OutputPath: output}
	steps := []manifest.PipelineStep{{Tool: "cp", Command: "cp {input} {output}"}}
	if err := ExecutePipeline(steps, ctx); err != nil {
		t.Fatalf("execute pipeline: %v", err)
	}

	b, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(b) != "hello\n" {
		t.Fatalf("unexpected output: %q", string(b))
	}
}

func TestExecutePipeline_FailureAndMissingOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(input, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	t.Run("step fails", func(t *testing.T) {
		ctx := BuildContext{InputPath: input, OutputPath: filepath.Join(dir, "out-fail.txt")}
		steps := []manifest.PipelineStep{{Tool: "sh", Command: "false"}}
		err := ExecutePipeline(steps, ctx)
		if err == nil || !strings.Contains(err.Error(), "pipeline step") {
			t.Fatalf("expected pipeline step error, got %v", err)
		}
	})

	t.Run("no output produced", func(t *testing.T) {
		ctx := BuildContext{InputPath: input, OutputPath: filepath.Join(dir, "out-missing.txt")}
		steps := []manifest.PipelineStep{{Tool: "sh", Command: "echo hi >/dev/null"}}
		err := ExecutePipeline(steps, ctx)
		if err == nil || !strings.Contains(err.Error(), "did not produce output") {
			t.Fatalf("expected missing output error, got %v", err)
		}
	})
}

func TestExpandAndShellQuote(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		InputPath:  "/tmp/in 'quote'.txt",
		OutputPath: "/tmp/out.txt",
		TmpPath:    "/tmp/t1",
		Tmp2Path:   "/tmp/t2",
		Width:      10,
		Height:     20,
		ScaleMode:  "fit",
		Background: "transparent",
	}

	got := expand("cp {input} {output} {tmp} {tmp2} {width} {height} {scale_mode} {background} {scale}", ctx)
	for _, want := range []string{`'/tmp/in '\''quote'\''.txt'`, "'/tmp/out.txt'", "10", "20", "'fit'", "'transparent'", "1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected expanded string to contain %q, got %q", want, got)
		}
	}

	if sq := shellQuote(""); sq != "''" {
		t.Fatalf("unexpected empty quote: %q", sq)
	}

	step := manifest.PipelineStep{
		Command:  "resvg {sets_size} {input} {output}",
		SetsSize: "-w {WIDTH} -h {HEIGHT}",
	}
	gotStepCmd := expandStepCommand(step, ctx)
	for _, want := range []string{"-w 10", "-h 20", `'/tmp/in '\''quote'\''.txt'`, "'/tmp/out.txt'"} {
		if !strings.Contains(gotStepCmd, want) {
			t.Fatalf("expected expanded step command to contain %q, got %q", want, gotStepCmd)
		}
	}
}

func TestResolvePipeline_CommandChainTable(t *testing.T) {
	t.Parallel()

	newManifest := func(rasterTool string, optimizeTool string) *manifest.Manifest {
		return &manifest.Manifest{
			Meta: manifest.Meta{
				Render: manifest.RenderConfig{
					Defaults: manifest.RenderDefaults{Tools: manifest.ToolPreference{"rsvg", "optipng"}},
					Tools: map[string]manifest.PipelineStep{
						"rsvg": {
							Tool:     rasterTool,
							Accepts:  []string{".svg"},
							Produces: []string{".raster"},
							Command:  "rsvg {input} {output} -w {width} -h {height}",
						},
						"optipng": {
							Tool:     optimizeTool,
							Accepts:  []string{".raster", ".png"},
							Produces: []string{".png"},
							Command:  "optipng {input} -out {output}",
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name              string
		manifest          *manifest.Manifest
		checkAvailability bool
		wantCommands      []string
		wantErrLike       string
	}{
		{
			name:              "all_available_two_step_chain",
			manifest:          newManifest("sh", "cp"),
			checkAvailability: true,
			wantCommands: []string{
				"rsvg 'from.svg' '/tmp/stage1.tmp' -w 100 -h 100",
				"optipng '/tmp/stage1.tmp' -out 'to.png'",
			},
		},
		{
			name:              "raster_unavailable_with_checks_enabled_errors",
			manifest:          newManifest("definitely-missing-binary", "cp"),
			checkAvailability: true,
			wantErrLike:       "no compatible conversion path",
		},
		{
			name:              "raster_unavailable_with_checks_disabled_still_plans",
			manifest:          newManifest("definitely-missing-binary", "cp"),
			checkAvailability: false,
			wantCommands: []string{
				"rsvg 'from.svg' '/tmp/stage1.tmp' -w 100 -h 100",
				"optipng '/tmp/stage1.tmp' -out 'to.png'",
			},
		},
		{
			name:              "png_to_png_optimizer_only",
			manifest:          newManifest("sh", "cp"),
			checkAvailability: true,
			wantCommands: []string{
				"optipng 'from.png' -out 'to.png'",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := manifest.Output{Path: "to.png", Options: manifest.Options{ScaleMode: "fit"}}
			source := "from.svg"
			if strings.Contains(tc.name, "png_to_png") {
				source = "from.png"
			}

			steps, err := ResolvePipelineWithOptions(tc.manifest, source, output, ResolveOptions{CheckAvailability: tc.checkAvailability})
			if tc.wantErrLike != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrLike) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrLike, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve pipeline: %v", err)
			}

			gotCommands := expandedCommandsForTest(steps, BuildContext{
				InputPath:  source,
				OutputPath: "to.png",
				Width:      100,
				Height:     100,
				ScaleMode:  "fit",
				Background: "transparent",
				TmpPath:    "/tmp/stage1.tmp",
				Tmp2Path:   "/tmp/stage2.tmp",
			})

			if len(gotCommands) != len(tc.wantCommands) {
				t.Fatalf("unexpected command count: got=%d want=%d got=%v want=%v", len(gotCommands), len(tc.wantCommands), gotCommands, tc.wantCommands)
			}
			for i := range tc.wantCommands {
				if gotCommands[i] != tc.wantCommands[i] {
					t.Fatalf("unexpected command at %d:\n got: %q\nwant: %q", i, gotCommands[i], tc.wantCommands[i])
				}
			}
		})
	}
}

func TestResolvePipeline_AppendsConfiguredTerminalOptimizer(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Tools: manifest.ToolPreference{"resvg"}},
				OptimizeByFormat: map[string]string{
					".png": "oxipng",
				},
				Tools: map[string]manifest.PipelineStep{
					"resvg": {
						Tool:     "resvg",
						Accepts:  []string{".svg"},
						Produces: []string{".png"},
						Command:  "resvg {input} {output}",
					},
					"oxipng": {
						Tool:     "oxipng",
						Accepts:  []string{".png"},
						Produces: []string{".png"},
						Command:  "oxipng -o 3 --strip safe {output}",
					},
				},
			},
		},
	}

	steps, err := ResolvePipelineWithOptions(m, "raw/logo.svg", manifest.Output{Path: "out/logo.png"}, ResolveOptions{CheckAvailability: false})
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected raster + optimizer, got %+v", steps)
	}
	if steps[1].Tool != "oxipng" {
		t.Fatalf("expected terminal optimizer to be oxipng, got %q", steps[1].Tool)
	}
}

func TestResolvePipeline_DoesNotDuplicateTerminalOptimizer(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Tools: manifest.ToolPreference{"oxipng"}},
				OptimizeByFormat: map[string]string{
					".png": "oxipng",
				},
				Tools: map[string]manifest.PipelineStep{
					"oxipng": {
						Tool:     "oxipng",
						Accepts:  []string{".png"},
						Produces: []string{".png"},
						Command:  "oxipng -o 3 --strip safe {output}",
					},
				},
			},
		},
	}

	steps, err := ResolvePipelineWithOptions(m, "raw/logo.png", manifest.Output{Path: "out/logo.png"}, ResolveOptions{CheckAvailability: false})
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected single optimizer step, got %+v", steps)
	}
}

func expandedCommandsForTest(steps []manifest.PipelineStep, ctx BuildContext) []string {
	return PlannedCommands(steps, ctx)
}
