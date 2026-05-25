package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	golden, err := os.ReadFile(filepath.Join("testdata", "verify_lock_source_mismatch.golden.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if diff := cmp.Diff(strings.TrimSpace(string(golden)), strings.TrimSpace(stderr.String())); diff != "" {
		t.Fatalf("verify-lock mismatch output (-want +got):\n%s", diff)
	}
}

func TestEndToEnd_CheckGenBuildVerify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	// Add strict-mode legal fields for this end-to-end validation.
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	updated := strings.ReplaceAll(string(manifestBytes), "source: \"raw/in.txt\"", "source: \"raw/in.txt\"\n    owner: \"A\"\n    copyright: \"C\"\n    license: \"L\"")
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer

	if exit := RunCheck([]string{"--manifest", manifestPath, "--strict"}, &stderr); exit != 0 {
		t.Fatalf("check failed: %s", stderr.String())
	}
	stderr.Reset()

	if exit := RunGen([]string{"--manifest", manifestPath}, &out, &stderr); exit != 0 {
		t.Fatalf("gen failed: %s", stderr.String())
	}
	if !strings.Contains(out.String(), "out/out.txt: raw/in.txt") {
		t.Fatalf("unexpected gen output: %s", out.String())
	}
	out.Reset()
	stderr.Reset()

	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}
	stderr.Reset()

	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 0 {
		t.Fatalf("verify-lock failed: %s", stderr.String())
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
