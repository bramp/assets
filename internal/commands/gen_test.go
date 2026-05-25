package commands

import (
	"bytes"
	"os"
	"path/filepath"
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
