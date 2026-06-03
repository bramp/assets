//nolint:testpackage // Render tests exercise unexported helpers.
package render

import (
	"testing"
)

func TestCollectProvenance(t *testing.T) {
	t.Parallel()

	steps := []ResolvedStep{
		{Tool: "go", Command: "go version"},
		{Tool: "go", Command: "go version"}, // duplicate tool should only be queried once
	}

	p := CollectProvenance(steps, []string{"go version", "go version"}, nil)
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

func TestCollectProvenance_UnknownToolVersion(t *testing.T) {
	t.Parallel()

	steps := []ResolvedStep{{
		Tool:    "definitely-not-a-real-tool-xyz",
		Command: "definitely-not-a-real-tool-xyz {input} {output}",
	}}

	p := CollectProvenance(steps, []string{"definitely-not-a-real-tool-xyz {input} {output}"}, nil)
	if p == nil {
		t.Fatal("expected provenance")
	}
	if got := p.Tools["definitely-not-a-real-tool-xyz"]; got != "" {
		t.Fatalf("expected empty version for unknown tool, got %q", got)
	}
}

func TestCollectProvenance_VersionArgsOverride(t *testing.T) {
	t.Parallel()

	steps := []ResolvedStep{{
		Tool:        "go",
		Command:     "go version",
		VersionArgs: []string{"version"},
	}}

	p := CollectProvenance(steps, []string{"go version"}, nil)
	if p == nil {
		t.Fatal("expected provenance")
	}
	if got := p.Tools["go"]; got == "" {
		t.Fatal("expected go version when version_args override is set")
	}
}

func TestCollectProvenance_UsesExpandedCommands(t *testing.T) {
	t.Parallel()

	steps := []ResolvedStep{{
		Tool:         "cp",
		Command:      "cp {input} {output}",
		InputFormat:  ".txt",
		OutputFormat: ".txt",
	}}

	p := CollectProvenance(steps, PlannedCommands(steps, BuildContext{
		InputPath:  "/tmp/raw/in.txt",
		OutputPath: "/tmp/out/out.txt",
	}), nil)
	if p == nil {
		t.Fatal("expected provenance")
	}
	if len(p.CommandChain) != 1 {
		t.Fatalf("unexpected command chain: %+v", p.CommandChain)
	}
	if got := p.CommandChain[0]; got != "cp '/tmp/raw/in.txt' '/tmp/out/out.txt'" {
		t.Fatalf("expected expanded command chain entry, got %q", got)
	}
}

func TestCollectProvenance_UsesCapturedExecutionCommands(t *testing.T) {
	t.Parallel()

	steps := []ResolvedStep{{
		Tool:    "cp",
		Command: "cp {input} {output}",
	}}
	chain := []string{"cp '/tmp/raw/in.txt' '/tmp/out/out.txt'"}

	p := CollectProvenance(steps, chain, nil)
	if p == nil {
		t.Fatal("expected provenance")
	}
	if len(p.CommandChain) != 1 {
		t.Fatalf("unexpected command chain: %+v", p.CommandChain)
	}
	if got := p.CommandChain[0]; got != chain[0] {
		t.Fatalf("expected captured command chain entry, got %q", got)
	}
}
