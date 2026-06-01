package manifest

import (
	_ "embed"
	"errors"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

//go:embed defaults.yaml
var builtinDefaultsYAML string

const (
	scaleModeFit          = "fit"
	backgroundTransparent = "transparent"
	toolKindTransform     = "transform"
	toolKindOptimize      = "optimize"
)

//nolint:gochecknoglobals // Exposed for external validation tooling and docs.
var StrictLegalFields = []string{"owner", "copyright", "license"}

// Manifest declares project metadata and source-to-output asset rules.
type Manifest struct {
	Meta   Meta    `yaml:"meta"`
	Assets []Asset `yaml:"assets"`
}

// Meta contains global project and render configuration.
type Meta struct {
	Project string       `yaml:"project"`
	Render  RenderConfig `yaml:"render"`
}

// RenderConfig contains default and named pipeline step definitions.
type RenderConfig struct {
	Defaults RenderDefaults      `yaml:"defaults"`
	Tools    map[string]ToolSpec `yaml:"tools"`
}

// RenderDefaults configures global render behavior.
type RenderDefaults struct {
	Tools                  ToolPreference `yaml:"tools"`
	StrictRendererVersions bool           `yaml:"strict_renderer_versions"`
}

// Asset describes one source file and all generated outputs derived from it.
type Asset struct {
	Source    string   `yaml:"source"`
	Owner     string   `yaml:"owner"`
	Copyright string   `yaml:"copyright"`
	License   string   `yaml:"license"`
	Outputs   []Output `yaml:"outputs"`
}

// Output defines one generated file and its target dimensions/options.
type Output struct {
	Path    string  `yaml:"path"`
	Width   int     `yaml:"width"`
	Height  int     `yaml:"height"`
	Options Options `yaml:"options"`
}

// Options contains per-output render controls.
// TODO: Not all options are enforced uniformly across every tool path yet.
// Validation accepts schema-level fields, but execution support is tool- and
// pipeline-dependent; keep this aligned as additional tool integrations land.
type Options struct {
	ScaleMode     string         `yaml:"scale_mode"`
	Background    string         `yaml:"background"`
	Tools         ToolPreference `yaml:"tools"`
	FormatOptions map[string]any `yaml:"format_options"`
}

// ToolPreference accepts either a single scalar tool name or a YAML list.
// Scalars are normalized to a one-item list to keep resolution logic consistent.
type ToolPreference []string

// UnmarshalYAML decodes a string or list YAML value into a normalized tool list.
func (p *ToolPreference) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.DocumentNode, yaml.MappingNode, yaml.AliasNode:
		return errors.New("tool preference must be a string or list")
	case yaml.ScalarNode:
		norm := strings.TrimSpace(value.Value)
		if norm == "" {
			*p = nil
			return nil
		}
		*p = ToolPreference{norm}
		return nil
	case yaml.SequenceNode:
		items := make([]string, 0, len(value.Content))
		for _, node := range value.Content {
			if node.Kind != yaml.ScalarNode {
				return errors.New("tool preference entries must be strings")
			}
			items = append(items, strings.TrimSpace(node.Value))
		}
		*p = ToolPreference(items)
		return nil
	default:
		return errors.New("tool preference must be a string or list")
	}
}

// ToolSpec describes one render tool capability used by graph resolution.
// Accepts/Produces are capability sets; concrete input/output formats are
// selected when resolving runtime steps.
type ToolSpec struct {
	Kind       string   `yaml:"kind"`
	Tool       string   `yaml:"tool"`
	Command    string   `yaml:"command"`
	Accepts    []string `yaml:"accepts"`
	Produces   []string `yaml:"produces"`
	ScaleModes []string `yaml:"scale_modes"`
	// SizeTemplate is the default fragment expanded into {size}.
	SizeTemplate string `yaml:"size_template"`
	// SizeByMode overrides SizeTemplate by requested scale mode.
	SizeByMode map[string]string `yaml:"size_by_mode"`
	// VersionArgs overrides version probing args for provenance collection.
	VersionArgs []string `yaml:"version_args"`
}

// KindOrDefault normalizes tool kind, defaulting to transform when omitted.
func (t ToolSpec) KindOrDefault() string {
	norm := strings.ToLower(strings.TrimSpace(t.Kind))
	if norm == "" {
		return toolKindTransform
	}
	return norm
}

// SetsTargetSize reports whether this tool can apply requested output size.
//
// A tool is considered size-capable if it declares a size template, has
// mode-specific size templates, or references size placeholders directly in its
// command.
func (t ToolSpec) SetsTargetSize() bool {
	if strings.TrimSpace(t.SizeTemplate) != "" || len(t.SizeByMode) > 0 {
		return true
	}
	return strings.Contains(t.Command, "{size}") ||
		strings.Contains(t.Command, "{width}") ||
		strings.Contains(t.Command, "{height}")
}

// ValidationConfig controls strictness and filesystem context for validation.
type ValidationConfig struct {
	Strict  bool
	BaseDir string
}

// BuiltinRenderDefaultsYAML returns the embedded default render config snippet.
func BuiltinRenderDefaultsYAML() string {
	return builtinDefaultsYAML
}
