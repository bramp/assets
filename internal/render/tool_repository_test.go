//nolint:testpackage // Tests exercise repository internals for cache behavior.
package render

import (
	"errors"
	"testing"
)

func TestCommandToolRepository_AvailableCachesLookups(t *testing.T) {
	t.Parallel()

	calls := 0
	repo := &commandToolRepository{
		available: map[string]bool{},
		versions:  map[string]string{},
		lookPath: func(binary string) error {
			calls++
			if binary == "missing" {
				return errors.New("missing")
			}
			return nil
		},
	}

	if !repo.Available("sh") {
		t.Fatal("expected sh to be available")
	}
	if !repo.Available("sh") {
		t.Fatal("expected cached sh availability")
	}
	if calls != 1 {
		t.Fatalf("expected one lookPath call for cached binary, got %d", calls)
	}

	if repo.Available("missing") {
		t.Fatal("expected missing binary to be unavailable")
	}
	if repo.Available("missing") {
		t.Fatal("expected cached missing result")
	}
	if calls != 2 {
		t.Fatalf("expected second lookPath call for distinct binary, got %d", calls)
	}
}

func TestCommandToolRepository_VersionCachesPerTool(t *testing.T) {
	t.Parallel()

	calls := 0
	repo := &commandToolRepository{
		available: map[string]bool{},
		versions:  map[string]string{},
		lookPath: func(string) error {
			return nil
		},
		runProbe: func(binary string, args []string) string {
			calls++
			if binary != "tool" {
				return ""
			}
			if len(args) == 1 && args[0] == "--version" {
				return "tool v1"
			}
			if len(args) == 1 && args[0] == "version" {
				return "tool version"
			}
			if len(args) == 1 && args[0] == "-version" {
				return "tool dash-version"
			}
			return ""
		},
	}

	step := ResolvedStep{Tool: "tool", Command: "tool {input} {output}"}
	if got := repo.Version(step); got != "tool v1" {
		t.Fatalf("unexpected default version probe result: %q", got)
	}
	if got := repo.Version(step); got != "tool v1" {
		t.Fatalf("unexpected cached version probe result: %q", got)
	}
	if calls != 1 {
		t.Fatalf("expected one probe call after cache hit, got %d", calls)
	}

	override := ResolvedStep{Tool: "tool", VersionArgs: []string{"-version"}}
	if got := repo.Version(override); got != "tool v1" {
		t.Fatalf("expected same cached version regardless of args, got %q", got)
	}
	if calls != 1 {
		t.Fatalf("expected no additional probe call after tool is cached, got %d", calls)
	}
}

func TestCommandToolRepository_VersionUsesOverrideWhenUncached(t *testing.T) {
	t.Parallel()

	repo := &commandToolRepository{
		available: map[string]bool{},
		versions:  map[string]string{},
		lookPath: func(string) error {
			return nil
		},
		runProbe: func(binary string, args []string) string {
			if binary == "tool" && len(args) == 1 && args[0] == "-version" {
				return "tool dash-version"
			}
			return ""
		},
	}

	step := ResolvedStep{Tool: "tool", VersionArgs: []string{"-version"}}
	if got := repo.Version(step); got != "tool dash-version" {
		t.Fatalf("unexpected override version probe result: %q", got)
	}
}
