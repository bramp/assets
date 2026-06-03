package render

import (
	"context"
	"os/exec"
	"sort"
	"strings"

	"github.com/bramp/assets/internal/lockfile"
)

// CollectProvenance records tool fingerprints using the exact command chain
// captured during pipeline execution.
func CollectProvenance(
	steps []ResolvedStep,
	commandChain []string,
	toolRepo ToolRepository,
) *lockfile.Provenance {
	return collectProvenance(steps, commandChain, toolRepo)
}

func collectProvenance(steps []ResolvedStep, commandChain []string, toolRepo ToolRepository) *lockfile.Provenance {
	chain := append([]string(nil), commandChain...)
	tools := map[string]string{}
	if toolRepo == nil {
		toolRepo = NewToolRepository()
	}

	if out, err := exec.CommandContext(context.Background(), "uname", "-a").CombinedOutput(); err == nil {
		tools["host_uname"] = strings.TrimSpace(string(out))
	}

	seen := map[string]bool{}
	for _, s := range steps {
		if s.Tool == "" || seen[s.Tool] {
			continue
		}
		seen[s.Tool] = true
		if v := toolRepo.Version(s); v != "" {
			tools[s.Tool] = v
		}
	}

	// Keep map output stable by ensuring deterministic insertion order for likely readers.
	if len(tools) > 0 {
		keys := make([]string, 0, len(tools))
		for k := range tools {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		stable := make(map[string]string, len(tools))
		for _, k := range keys {
			stable[k] = tools[k]
		}
		tools = stable
	}

	return &lockfile.Provenance{CommandChain: chain, Tools: tools}
}
