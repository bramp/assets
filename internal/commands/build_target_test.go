package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunBuildTarget_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "raw", "in.txt")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	manifest := `meta:
  project: "test"
  render:
    defaults:
      profile: "basic"
    profiles:
      basic:
        pipeline:
          - stage: "copy"
            tool: "cp"
            command: "cp {input} {output}"
assets:
  - id: "a"
    source: "raw/in.txt"
    outputs:
      - path: "out/out.txt"
        width: 1
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
`
	manifestPath := filepath.Join(dir, "assets.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stderr bytes.Buffer
	exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr)
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exit, stderr.String())
	}

	outPath := filepath.Join(dir, "out", "out.txt")
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "hello\n" {
		t.Fatalf("unexpected output: %q", string(got))
	}
}

func TestRunBuildTarget_TargetNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "raw", "in.txt")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	manifest := `meta:
  project: "test"
assets:
  - id: "a"
    source: "raw/in.txt"
    outputs:
      - path: "out/out.txt"
        width: 1
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
`
	manifestPath := filepath.Join(dir, "assets.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stderr bytes.Buffer
	exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/missing.txt"}, &stderr)
	if exit != 1 {
		t.Fatalf("expected exit 1, got %d", exit)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr output")
	}
}
