package render

import (
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

func TestCollectProvenance(t *testing.T) {
	t.Parallel()

	steps := []manifest.PipelineStep{
		{Tool: "go", Command: "go version"},
		{Tool: "go", Command: "go version"}, // duplicate tool should only be queried once
	}

	p := CollectProvenance(steps)
	if p == nil {
		t.Fatal("expected provenance")
	}
	if len(p.CommandChain) != 2 {
		t.Fatalf("unexpected command chain: %+v", p.CommandChain)
	}
	if p.Tools["host_uname"] == "" {
		t.Fatal("expected host_uname in tools")
	}
	if p.Tools["go"] == "" {
		t.Fatal("expected go version in tools")
	}
}

func TestCommandVersion_UnknownTool(t *testing.T) {
	t.Parallel()

	if got := commandVersion("definitely-not-a-real-tool-xyz"); got != "" {
		t.Fatalf("expected empty version for unknown tool, got %q", got)
	}
}
