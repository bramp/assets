//nolint:testpackage // Render tests intentionally exercise unexported helpers.
package render

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

var unresolvedPlaceholderRE = regexp.MustCompile(`\{[a-z_]+\}`)

//nolint:gocognit,tparallel // Explicit matrix assertions keep coverage intent clear; test uses t.Setenv.
func TestBuiltinDefaultTools_ExecuteEveryConfiguration(t *testing.T) {
	testManifestYAML := `meta:
  project: "default-tool-matrix"
assets: []
`

	m, err := manifest.Load(strings.NewReader(testManifestYAML))
	if err != nil {
		t.Fatalf("load manifest with built-in defaults: %v", err)
	}

	if len(m.Meta.Render.Tools) == 0 {
		t.Fatal("expected built-in render tools to be present")
	}

	stubBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("mkdir stub bin: %v", err)
	}

	toolNames := sortedToolNames(m.Meta.Render.Tools)
	for _, name := range toolNames {
		tool := m.Meta.Render.Tools[name]
		if err := writeToolStub(stubBin, tool.Tool); err != nil {
			t.Fatalf("write stub for %q: %v", tool.Tool, err)
		}
	}

	pathSep := string(os.PathListSeparator)
	t.Setenv("PATH", stubBin+pathSep+os.Getenv("PATH"))

	workDir := t.TempDir()
	for _, toolName := range toolNames {
		step := m.Meta.Render.Tools[toolName]
		for _, mode := range modesForStep(step) {
			t.Run(fmt.Sprintf("%s/%s", toolName, mode), func(t *testing.T) {
				t.Parallel()

				inputExt := firstConcreteExt(step.Accepts)
				outputExt := firstConcreteExt(step.Produces)
				if outputExt == "" {
					outputExt = inputExt
				}

				inputPath := filepath.Join(workDir, fmt.Sprintf("in-%s-%s%s", toolName, mode, inputExt))
				outputPath := filepath.Join(workDir, fmt.Sprintf("out-%s-%s%s", toolName, mode, outputExt))
				if err := os.WriteFile(inputPath, []byte("not-empty\n"), 0o644); err != nil {
					t.Fatalf("write input: %v", err)
				}

				ctx := BuildContext{
					InputPath:  inputPath,
					OutputPath: outputPath,
					Width:      128,
					Height:     96,
					ScaleMode:  mode,
					Background: "transparent",
				}

				command := expandStepCommand(step, ctx)
				if unresolvedPlaceholderRE.MatchString(command) {
					t.Fatalf("unresolved placeholder in command %q", command)
				}

				if err := ExecutePipeline([]manifest.PipelineStep{step}, ctx); err != nil {
					t.Fatalf("execute step failed (tool=%s mode=%s): %v", toolName, mode, err)
				}
			})
		}
	}
}

func sortedToolNames(tools map[string]manifest.PipelineStep) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func modesForStep(step manifest.PipelineStep) []string {
	modes := make(map[string]struct{})
	for _, mode := range step.ScaleModes {
		norm := strings.ToLower(strings.TrimSpace(mode))
		switch norm {
		case "":
			continue
		case "*":
			for _, m := range []string{"fit", "fill", "stretch", "crop"} {
				modes[m] = struct{}{}
			}
		default:
			modes[norm] = struct{}{}
		}
	}
	for mode := range step.SizeByMode {
		norm := strings.ToLower(strings.TrimSpace(mode))
		if norm == "" || norm == "*" {
			continue
		}
		modes[norm] = struct{}{}
	}
	if len(modes) == 0 {
		modes["fit"] = struct{}{}
	}

	out := make([]string, 0, len(modes))
	for mode := range modes {
		out = append(out, mode)
	}
	sort.Strings(out)
	return out
}

func firstConcreteExt(exts []string) string {
	for _, ext := range exts {
		norm := strings.ToLower(strings.TrimSpace(ext))
		if norm == "" || norm == "*" {
			continue
		}
		if strings.HasPrefix(norm, ".") {
			return norm
		}
	}
	return ".tmp"
}

func writeToolStub(binDir string, name string) error {
	stubPath := filepath.Join(binDir, name)
	stub := `#!/bin/sh
set -eu

pick_output() {
  out=""
  prev=""
  for arg in "$@"; do
    case "$prev" in
      -o|--out|--output|--export-filename)
        out="$arg"
        prev=""
        continue
        ;;
    esac
    case "$arg" in
      -o|--out|--output|--export-filename)
        prev="$arg"
        continue
        ;;
      --export-filename=*)
        out="${arg#*=}"
        ;;
    esac
    prev=""
  done

  if [ -z "$out" ] && [ "${1:-}" = "resize" ]; then
    out="${3:-}"
  fi
  if [ -z "$out" ] && [ "${1:-}" = "copy" ]; then
    out="${3:-}"
  fi
  if [ -z "$out" ]; then
    for arg in "$@"; do out="$arg"; done
  fi

  printf '%s' "$out"
}

out="$(pick_output "$@")"
if [ -z "$out" ]; then
  exit 1
fi

mkdir -p "$(dirname "$out")"
printf 'stub-output\n' > "$out"
`

	if err := os.WriteFile(stubPath, []byte(stub), 0o755); err != nil {
		return err
	}
	return nil
}
