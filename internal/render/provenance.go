package render

import (
	"context"
	"os/exec"
	"sort"
	"strings"

	"github.com/bramp/assets/internal/lockfile"
	"github.com/bramp/assets/internal/manifest"
)

// CollectProvenance records command chain and tool fingerprints for resolved steps.
func CollectProvenance(steps []manifest.PipelineStep) *lockfile.Provenance {
	chain := make([]string, 0, len(steps))
	tools := map[string]string{}

	if out, err := exec.CommandContext(context.Background(), "uname", "-a").CombinedOutput(); err == nil {
		tools["host_uname"] = strings.TrimSpace(string(out))
	}

	seen := map[string]bool{}
	for _, s := range steps {
		chain = append(chain, s.Command)
		if s.Tool == "" || seen[s.Tool] {
			continue
		}
		seen[s.Tool] = true
		if v := commandVersion(s.Tool); v != "" {
			tools[s.Tool] = v
		}
	}

	// Keep command chain deterministic for hashing/storage comparisons.
	chainCopy := append([]string(nil), chain...)

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

	return &lockfile.Provenance{CommandChain: chainCopy, Tools: tools}
}

func commandVersion(cmdName string) string {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		out, err := exec.CommandContext(context.Background(), cmdName, args...).CombinedOutput()
		if err != nil {
			continue
		}
		line := strings.TrimSpace(string(out))
		if line == "" {
			continue
		}
		if idx := strings.IndexByte(line, '\n'); idx >= 0 {
			line = line[:idx]
		}
		return line
	}
	return ""
}
