package manifest

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

//go:embed defaults.yaml
var builtinDefaultsYAML string

// StrictLegalFields are validated only when strict mode is enabled.
var StrictLegalFields = []string{"owner", "copyright", "license"}

type Manifest struct {
	Meta   Meta    `yaml:"meta"`
	Assets []Asset `yaml:"assets"`
}

type Meta struct {
	Project string       `yaml:"project"`
	Render  RenderConfig `yaml:"render"`
}

type RenderConfig struct {
	Defaults         RenderDefaults          `yaml:"defaults"`
	Tools            map[string]PipelineStep `yaml:"tools"`
	OptimizeByFormat map[string]string       `yaml:"optimize_by_format"`
}

type RenderDefaults struct {
	Tools                  ToolPreference `yaml:"tools"`
	StrictRendererVersions bool           `yaml:"strict_renderer_versions"`
}

type Asset struct {
	ID        string   `yaml:"id"`
	Source    string   `yaml:"source"`
	Owner     string   `yaml:"owner"`
	Copyright string   `yaml:"copyright"`
	License   string   `yaml:"license"`
	Outputs   []Output `yaml:"outputs"`
}

type Output struct {
	Path    string  `yaml:"path"`
	Width   int     `yaml:"width"`
	Height  int     `yaml:"height"`
	Options Options `yaml:"options"`
}

type Options struct {
	ScaleMode     string                 `yaml:"scale_mode"`
	Background    string                 `yaml:"background"`
	Tools         ToolPreference         `yaml:"tools"`
	FormatOptions map[string]interface{} `yaml:"format_options"`
}

// ToolPreference accepts either a single scalar tool name or a YAML list.
// Scalars are normalized to a one-item list to keep resolution logic consistent.
type ToolPreference []string

func (p *ToolPreference) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
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
				return fmt.Errorf("tool preference entries must be strings")
			}
			items = append(items, strings.TrimSpace(node.Value))
		}
		*p = ToolPreference(items)
		return nil
	default:
		return fmt.Errorf("tool preference must be a string or list")
	}
}

type PipelineStep struct {
	Tool       string   `yaml:"tool"`
	Command    string   `yaml:"command"`
	Accepts    []string `yaml:"accepts"`
	Produces   []string `yaml:"produces"`
	ScaleModes []string `yaml:"scale_modes"`
	SetsSize   string   `yaml:"sets_size"`
}

type ValidationConfig struct {
	Strict  bool
	BaseDir string
}

func BuiltinRenderDefaultsYAML() string {
	return builtinDefaultsYAML
}

func LoadFile(path string) (*Manifest, error) {
	baseDoc, err := defaultManifestDoc()
	if err != nil {
		return nil, fmt.Errorf("load built-in defaults: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	userDoc, err := decodeYAMLDocument(f)
	if err != nil {
		return nil, err
	}
	mergedDoc := mergeYAMLNodes(baseDoc, userDoc)

	var m Manifest
	if err := decodeYAMLNodeKnownFields(mergedDoc, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func defaultManifestDoc() (*yaml.Node, error) {
	defaultsDoc, err := decodeYAMLDocument(strings.NewReader(builtinDefaultsYAML))
	if err != nil {
		return nil, err
	}
	renderNode, ok := mappingValue(defaultsDoc.Content[0], "render")
	if !ok {
		return nil, fmt.Errorf("defaults.yaml missing render block")
	}

	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "meta"},
				{
					Kind: yaml.MappingNode,
					Tag:  "!!map",
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "render"},
						cloneYAMLNode(renderNode),
					},
				},
			},
		}},
	}, nil
}

func decodeYAMLDocument(r io.Reader) (*yaml.Node, error) {
	dec := yaml.NewDecoder(r)
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("expected a single YAML document")
	}
	return &doc, nil
}

func decodeYAMLNodeKnownFields(doc *yaml.Node, out interface{}) error {
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func mergeYAMLNodes(base *yaml.Node, override *yaml.Node) *yaml.Node {
	if base == nil {
		return cloneYAMLNode(override)
	}
	if override == nil {
		return cloneYAMLNode(base)
	}

	if base.Kind == yaml.DocumentNode && override.Kind == yaml.DocumentNode {
		mergedRoot := mergeYAMLNodes(base.Content[0], override.Content[0])
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mergedRoot}}
	}

	if base.Kind != yaml.MappingNode || override.Kind != yaml.MappingNode {
		return cloneYAMLNode(override)
	}

	merged := cloneYAMLNode(base)
	for i := 0; i+1 < len(override.Content); i += 2 {
		key := override.Content[i]
		value := override.Content[i+1]
		idx := mappingKeyIndex(merged, key.Value)
		if idx < 0 {
			merged.Content = append(merged.Content, cloneYAMLNode(key), cloneYAMLNode(value))
			continue
		}
		merged.Content[idx+1] = mergeYAMLNodes(merged.Content[idx+1], value)
	}
	return merged
}

func mappingKeyIndex(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func mappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	idx := mappingKeyIndex(node, key)
	if idx < 0 {
		return nil, false
	}
	return node.Content[idx+1], true
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := *n
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			clone.Content[i] = cloneYAMLNode(c)
		}
	}
	if n.Alias != nil {
		clone.Alias = cloneYAMLNode(n.Alias)
	}
	return &clone
}

func (m *Manifest) Validate(cfg ValidationConfig) []error {
	var errs []error

	if strings.TrimSpace(m.Meta.Project) == "" {
		errs = append(errs, fmt.Errorf("meta.project is required"))
	}

	renderErrs := validateRenderConfig(m.Meta.Render)
	if len(renderErrs) > 0 {
		errs = append(errs, renderErrs...)
	}

	if len(m.Assets) == 0 {
		errs = append(errs, fmt.Errorf("assets must contain at least one asset"))
		return errs
	}

	seenOutputs := make(map[string]string)
	for i, asset := range m.Assets {
		assetRef := assetRef(asset, i)

		if strings.TrimSpace(asset.ID) == "" {
			errs = append(errs, fmt.Errorf("%s: id is required", assetRef))
		}
		if strings.TrimSpace(asset.Source) == "" {
			errs = append(errs, fmt.Errorf("%s: source is required", assetRef))
		}
		if len(asset.Outputs) == 0 {
			errs = append(errs, fmt.Errorf("%s: outputs must contain at least one output", assetRef))
		}

		if cfg.Strict {
			if strings.TrimSpace(asset.Owner) == "" {
				errs = append(errs, fmt.Errorf("%s: owner is required in strict mode", assetRef))
			}
			if strings.TrimSpace(asset.Copyright) == "" {
				errs = append(errs, fmt.Errorf("%s: copyright is required in strict mode", assetRef))
			}
			if strings.TrimSpace(asset.License) == "" {
				errs = append(errs, fmt.Errorf("%s: license is required in strict mode", assetRef))
			}
		}

		sourcePath := filepath.Join(cfg.BaseDir, asset.Source)
		if _, err := os.Stat(sourcePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: source file does not exist: %s", assetRef, asset.Source))
		}

		for j, out := range asset.Outputs {
			outputRef := fmt.Sprintf("%s output[%d]", assetRef, j)
			if strings.TrimSpace(out.Path) == "" {
				errs = append(errs, fmt.Errorf("%s: path is required", outputRef))
			}

			if out.Path != "" {
				if first, ok := seenOutputs[out.Path]; ok {
					errs = append(errs, fmt.Errorf("%s: duplicate output path %q (already used by %s)", outputRef, out.Path, first))
				} else {
					seenOutputs[out.Path] = outputRef
				}
			}

			if out.Width <= 0 {
				errs = append(errs, fmt.Errorf("%s: width must be > 0", outputRef))
			}
			if out.Height <= 0 {
				errs = append(errs, fmt.Errorf("%s: height must be > 0", outputRef))
			}

			if !validScaleMode(out.Options.ScaleMode) {
				errs = append(errs, fmt.Errorf("%s: options.scale_mode must be one of fit, fill, stretch, crop", outputRef))
			}
			if !validBackground(out.Options.Background) {
				errs = append(errs, fmt.Errorf("%s: options.background must be transparent or #RRGGBB", outputRef))
			}

			stageErrs := validateStagePreference(outputRef+" options.tools", out.Options.Tools, m.Meta.Render.Tools, true, true)
			errs = append(errs, stageErrs...)
		}
	}

	sort.Slice(errs, func(i, j int) bool {
		return errs[i].Error() < errs[j].Error()
	})

	return errs
}

func assetRef(a Asset, idx int) string {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Sprintf("asset[%d]", idx)
	}
	return fmt.Sprintf("asset[%q]", a.ID)
}

func validScaleMode(v string) bool {
	switch v {
	case "fit", "fill", "stretch", "crop":
		return true
	default:
		return false
	}
}

func validBackground(v string) bool {
	if v == "transparent" {
		return true
	}
	return hexColorRe.MatchString(v)
}

func validateRenderConfig(cfg RenderConfig) []error {
	var errs []error

	err := validateToolRegistry("meta.render.tools", cfg.Tools)
	errs = append(errs, err...)
	err = validateStageOrder("meta.render.defaults.tools", cfg.Defaults.Tools, cfg.Tools)
	errs = append(errs, err...)

	for ext, tool := range cfg.OptimizeByFormat {
		normExt := strings.TrimSpace(ext)
		if normExt == "" {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format contains an empty extension key"))
			continue
		}
		if !strings.HasPrefix(normExt, ".") {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format extension %q must start with '.'", ext))
		}
		normTool := strings.TrimSpace(tool)
		if normTool == "" {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format[%q] must name an optimize tool", ext))
			continue
		}
		if _, ok := cfg.Tools[normTool]; !ok {
			errs = append(errs, fmt.Errorf("meta.render.optimize_by_format[%q] references unknown optimize tool %q", ext, normTool))
		}
	}

	return errs
}

func validateStageOrder(prefix string, order ToolPreference, registry map[string]PipelineStep) []error {
	return validateStagePreference(prefix, order, registry, true, true)
}

func validateStagePreference(prefix string, pref ToolPreference, registry map[string]PipelineStep, allowAuto bool, allowDisable bool) []error {
	var errs []error
	for i, name := range pref {
		norm := strings.TrimSpace(name)
		if norm == "" {
			errs = append(errs, fmt.Errorf("%s[%d] must not be empty", prefix, i))
			continue
		}
		if allowAuto && strings.EqualFold(norm, "auto") {
			continue
		}
		if allowDisable && (strings.EqualFold(norm, "none") || strings.EqualFold(norm, "off")) {
			continue
		}
		if _, ok := registry[norm]; !ok {
			errs = append(errs, fmt.Errorf("%s[%d] %q does not exist in stage registry", prefix, i, name))
		}
	}
	return errs
}

func validSupportsFormat(v string) bool {
	if v == "*" {
		return true
	}
	return strings.HasPrefix(v, ".")
}

func validScaleModeValue(v string) bool {
	if v == "*" {
		return true
	}
	return validScaleMode(v)
}

func validatePipelineStepSupports(prefix string, step PipelineStep) []error {
	var errs []error
	for i, f := range step.Accepts {
		norm := strings.TrimSpace(f)
		if !validSupportsFormat(norm) {
			errs = append(errs, fmt.Errorf("%s.accepts[%d] %q must be '*' or extension like .png", prefix, i, f))
		}
	}
	for i, f := range step.Produces {
		norm := strings.TrimSpace(f)
		if !validSupportsFormat(norm) {
			errs = append(errs, fmt.Errorf("%s.produces[%d] %q must be '*' or extension like .png", prefix, i, f))
		}
	}
	return errs
}

func validatePipelineStepScaleModes(prefix string, step PipelineStep) []error {
	var errs []error
	for i, mode := range step.ScaleModes {
		norm := strings.TrimSpace(mode)
		if !validScaleModeValue(norm) {
			errs = append(errs, fmt.Errorf("%s.scale_modes[%d] %q must be '*' or one of fit, fill, stretch, crop", prefix, i, mode))
		}
	}
	return errs
}

func validateStageRegistry(prefix string, registry map[string]PipelineStep) []error {
	var errs []error
	for name, step := range registry {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Errorf("%s contains an empty tool name", prefix))
			continue
		}
		if strings.TrimSpace(step.Tool) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: tool is required", prefix, name))
		}
		if strings.TrimSpace(step.Command) == "" {
			errs = append(errs, fmt.Errorf("%s[%q]: command is required", prefix, name))
		}
		if strings.TrimSpace(step.SetsSize) != "" && !strings.Contains(step.Command, "{sets_size}") && !commandUsesTargetSizePlaceholders(step.Command) {
			errs = append(errs, fmt.Errorf("%s[%q]: sets_size is configured but command does not use {sets_size} or width/height placeholders", prefix, name))
		}
		errs = append(errs, validatePipelineStepSupports(fmt.Sprintf("%s[%q]", prefix, name), step)...)
		errs = append(errs, validatePipelineStepScaleModes(fmt.Sprintf("%s[%q]", prefix, name), step)...)
	}
	return errs
}

func validateToolRegistry(prefix string, registry map[string]PipelineStep) []error {
	var errs []error
	for name, step := range registry {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Errorf("%s contains an empty tool name", prefix))
			continue
		}
		if len(step.Accepts) == 0 || len(step.Produces) == 0 {
			errs = append(errs, fmt.Errorf("%s[%q]: tools must define both accepts and produces", prefix, name))
		}
		errs = append(errs, validateStageRegistry(prefix, map[string]PipelineStep{name: step})...)
	}
	return errs
}

func commandUsesTargetSizePlaceholders(cmd string) bool {
	return strings.Contains(cmd, "{width}") || strings.Contains(cmd, "{height}") || strings.Contains(cmd, "{WIDTH}") || strings.Contains(cmd, "{HEIGHT}")
}

func validatePipelineSteps(prefix string, steps []PipelineStep) []error {
	var errs []error
	if len(steps) == 0 {
		errs = append(errs, fmt.Errorf("%s must contain at least one step", prefix))
		return errs
	}

	for i, step := range steps {
		if strings.TrimSpace(step.Tool) == "" {
			errs = append(errs, fmt.Errorf("%s[%d]: tool is required", prefix, i))
		}
		if strings.TrimSpace(step.Command) == "" {
			errs = append(errs, fmt.Errorf("%s[%d]: command is required", prefix, i))
		}
		errs = append(errs, validatePipelineStepSupports(fmt.Sprintf("%s[%d]", prefix, i), step)...)
	}

	return errs
}
