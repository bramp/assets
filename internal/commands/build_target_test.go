package commands

import (
	"bytes"
	"encoding/json"
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

	// TODO(bramp): Should we create a temp directory for the output and lockfile instead of writing to the manifest directory?

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

	lockPath := filepath.Join(dir, "assets.lock")
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	var lockData map[string]interface{}
	if err := json.Unmarshal(lockBytes, &lockData); err != nil {
		t.Fatalf("unmarshal lockfile: %v", err)
	}
	assets, ok := lockData["assets"].(map[string]interface{})
	if !ok || len(assets) == 0 {
		t.Fatalf("expected assets entries in lockfile: %s", string(lockBytes))
	}
	aData, ok := assets["a"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected asset entry 'a' in lockfile: %s", string(lockBytes))
	}
	outputs, ok := aData["outputs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected outputs in lockfile asset entry: %s", string(lockBytes))
	}
	oData, ok := outputs["out/out.txt"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected output entry in lockfile: %s", string(lockBytes))
	}
	if _, hasConfigHash := oData["config_hash"]; hasConfigHash {
		t.Fatalf("did not expect config_hash in lockfile output: %s", string(lockBytes))
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
