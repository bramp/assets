package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bramp/assets/internal/commands"
)

func TestRunDefaults_GoldenOutput(t *testing.T) {
	t.Parallel()

	goldenPath := filepath.Join("testdata", "defaults.golden.yaml")
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := commands.RunDefaults(nil, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if got, want := stdout.String(), string(goldenBytes); got != want {
		t.Fatalf("defaults output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRunDefaults_ArgErrors(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exit := commands.RunDefaults([]string{"extra"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected positional argument error, got %d", exit)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := commands.RunDefaults([]string{"--unknown"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected parse error, got %d", exit)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := commands.RunDefaults([]string{"--transform", "nope"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected flag parse error, got %d", exit)
	}
}
