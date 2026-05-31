package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_NoCommand_PrintsUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage: assets <command> [flags]") {
		t.Fatalf("expected usage in stderr, got: %q", stderr.String())
	}
}

func TestRun_UnknownCommand_PrintsUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"nope"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command: nope") {
		t.Fatalf("expected unknown command error, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: assets <command> [flags]") {
		t.Fatalf("expected usage in stderr, got: %q", stderr.String())
	}
}

func TestRun_DefaultsRoute(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"defaults"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected defaults output on stdout")
	}
}
