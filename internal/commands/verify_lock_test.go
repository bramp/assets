package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunVerifyLock_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}
	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 0 {
		t.Fatalf("verify-lock failed: %s", stderr.String())
	}
}

func TestRunVerifyLock_SourceMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}

	if err := os.WriteFile(filepath.Join(dir, "raw", "in.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}

	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 1 {
		t.Fatalf("expected verify-lock failure, got %d", exit)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected verify-lock stderr")
	}
}

func writePipelineFixture(t *testing.T, dir string) string {
	t.Helper()
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
	return manifestPath
}
