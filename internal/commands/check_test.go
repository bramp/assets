package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCheck_ArgumentAndLoadErrors(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if exit := RunCheck([]string{"--unknown"}, &stderr); exit != 1 {
		t.Fatalf("expected parse failure exit 1, got %d", exit)
	}

	stderr.Reset()
	if exit := RunCheck([]string{"--manifest", "missing.yaml"}, &stderr); exit != 1 {
		t.Fatalf("expected load failure exit 1, got %d", exit)
	}

	stderr.Reset()
	if exit := RunCheck([]string{"extra-positional"}, &stderr); exit != 1 {
		t.Fatalf("expected positional arg failure exit 1, got %d", exit)
	}
}

func TestRunCheck_ValidationFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `meta:
  project: ""
assets:
  - id: "a"
    source: "raw/missing.txt"
    outputs:
      - path: "out/out.txt"
        width: 0
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
`
	path := filepath.Join(dir, "assets.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stderr bytes.Buffer
	if exit := RunCheck([]string{"--manifest", path}, &stderr); exit != 1 {
		t.Fatalf("expected validation failure, got %d", exit)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected validation errors on stderr")
	}
}
