package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

func TestResolvePipeline_Override(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Profile: "default"},
				Profiles: map[string]manifest.RenderProfile{
					"default": {
						Pipeline: []manifest.PipelineStep{{Tool: "cp", Command: "cp {input} {output}"}},
					},
				},
			},
		},
	}

	o := manifest.Output{Options: manifest.Options{PipelineOverride: []manifest.PipelineStep{{Tool: "echo", Command: "echo hi > {output}"}}}}
	steps, err := ResolvePipeline(m, o)
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 1 || steps[0].Tool != "echo" {
		t.Fatalf("unexpected override steps: %+v", steps)
	}
}

func TestResolvePipeline_Append(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Profile: "default"},
				Profiles: map[string]manifest.RenderProfile{
					"default": {
						Pipeline: []manifest.PipelineStep{{Tool: "cp", Command: "cp {input} {tmp}"}},
					},
				},
			},
		},
	}

	o := manifest.Output{Options: manifest.Options{PipelineAppend: []manifest.PipelineStep{{Tool: "mv", Command: "mv {tmp} {output}"}}}}
	steps, err := ResolvePipeline(m, o)
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 2 || steps[1].Tool != "mv" {
		t.Fatalf("unexpected appended steps: %+v", steps)
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
	for _, want := range []string{"'/tmp/in '\\''quote'\\''.txt'", "'/tmp/out.txt'", "10", "20", "'fit'", "'transparent'", "1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected expanded string to contain %q, got %q", want, got)
		}
	}

	if sq := shellQuote(""); sq != "''" {
		t.Fatalf("unexpected empty quote: %q", sq)
	}
}
