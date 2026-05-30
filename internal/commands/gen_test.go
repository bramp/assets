package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGen_GoldenOutput(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("testdata", "manifest_gen.yaml")
	goldenPath := filepath.Join("testdata", "gen.golden.mk")

	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunGen([]string{"--manifest", manifestPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	if got, want := stdout.String(), string(goldenBytes); got != want {
		t.Fatalf("generated output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRunGen_LoadError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunGen([]string{"--manifest", "testdata/does-not-exist.yaml"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr output")
	}
}

func TestRunGen_ArgErrors(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exit := RunGen([]string{"--unknown"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected parse error, got %d", exit)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := RunGen([]string{"extra"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected positional argument error, got %d", exit)
	}
}

func TestRunGen_CommentCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "assets.yaml")
	manifestContent := `meta:
  project: "test"
  render:
    defaults:
      tools: ["raster", "opt"]
    tools:
      raster:
        tool: "sh"
        accepts: [".svg"]
        produces: [".png"]
        command: "cp {input} {output}"
      opt:
        tool: "sh"
        accepts: [".png"]
        produces: [".png"]
        command: "cp {input} {output}"
assets:
  - id: "a"
    source: "raw/in.svg"
    outputs:
      - path: "out/a.png"
        width: 100
        height: 100
        options:
          scale_mode: "fit"
          background: "transparent"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunGen([]string{"--manifest", manifestPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"out/a.png: raw/in.svg",
		"  # cp 'raw/in.svg' '__tmp1__'",
		"  # oxipng -o 3 --strip safe --out 'out/a.png' '__tmp1__'",
		"assets.lock: $(GENERATED_ASSET_FILES)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected generated output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRunGen_CommentCommandsSplitChains(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "assets.yaml")
	manifestContent := `meta:
  project: "test"
  render:
    defaults:
      tools: ["raster"]
    tools:
      raster:
        tool: "sh"
        accepts: [".svg"]
        produces: [".png"]
        command: "cp {input} {output} && : -resize {width}x{height}"
assets:
  - id: "a"
    source: "raw/in.svg"
    outputs:
      - path: "out/a.png"
        width: 100
        height: 100
        options:
          scale_mode: "fit"
          background: "transparent"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunGen([]string{"--manifest", manifestPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"  # cp 'raw/in.svg' '__tmp1__'",
		"  # && : -resize 100x100",
		"  # oxipng -o 3 --strip safe --out 'out/a.png' '__tmp1__'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected generated output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRunGen_CommentCommandsSplitPipesAndAnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "assets.yaml")
	manifestContent := `meta:
  project: "test"
  render:
    defaults:
      tools: ["pipe"]
    tools:
      pipe:
        tool: "sh"
        accepts: [".svg"]
        produces: [".png"]
        command: "cat {input} | cat > {output} && : done"
assets:
  - id: "a"
    source: "raw/in.svg"
    outputs:
      - path: "out/a.png"
        width: 100
        height: 100
        options:
          scale_mode: "fit"
          background: "transparent"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunGen([]string{"--manifest", manifestPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"  # cat 'raw/in.svg'",
		"  # | cat > '__tmp1__'",
		"  # && : done",
		"  # oxipng -o 3 --strip safe --out 'out/a.png' '__tmp1__'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected generated output to contain %q, got:\n%s", want, got)
		}
	}
}
