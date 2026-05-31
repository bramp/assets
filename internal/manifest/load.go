package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	manifestKeyMeta   = "meta"
	manifestKeyRender = "render"
)

// Load reads, merges defaults, and decodes a manifest from a reader.
func Load(r io.Reader) (*Manifest, error) {
	base, err := defaultManifestMap()
	if err != nil {
		return nil, fmt.Errorf("load built-in defaults: %w", err)
	}

	user, err := decodeYAMLMap(r)
	if err != nil {
		return nil, err
	}
	merged := mergeYAMLMaps(base, user)

	var m Manifest
	if err := decodeYAMLKnownFields(merged, &m); err != nil {
		return nil, err
	}
	normalizeManifest(&m)

	return &m, nil
}

// LoadFile reads, merges defaults, and decodes a manifest from disk.
func LoadFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Load(f)
}

// defaultManifestMap extracts only the built-in meta.render block and wraps it
// as a minimal manifest map used as the merge base.
//
// It intentionally does not return builtinDefaultsYAML directly because that
// file may contain keys outside the user manifest schema. By narrowing to
// meta.render, Load can merge render defaults deterministically without
// injecting unrelated defaults into the decoded Manifest.
func defaultManifestMap() (map[string]any, error) {
	var defaults map[string]any
	if err := yaml.Unmarshal([]byte(builtinDefaultsYAML), &defaults); err != nil {
		return nil, fmt.Errorf("decode defaults.yaml: %w", err)
	}

	render, ok := defaults[manifestKeyRender]
	if !ok {
		return nil, errors.New("defaults.yaml missing render block")
	}

	return map[string]any{
		manifestKeyMeta: map[string]any{
			manifestKeyRender: render,
		},
	}, nil
}

func decodeYAMLMap(r io.Reader) (map[string]any, error) {
	dec := yaml.NewDecoder(r)
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("expected a YAML mapping document")
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("expected a single YAML document")
		}
		return nil, err
	}

	return doc, nil
}

func decodeYAMLKnownFields(v any, out any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func mergeYAMLMaps(base map[string]any, override map[string]any) map[string]any {
	mergedAny := cloneYAMLValue(base)
	merged, ok := mergedAny.(map[string]any)
	if !ok {
		merged = make(map[string]any)
	}
	for key, overrideValue := range override {
		baseValue, ok := merged[key]
		if !ok {
			merged[key] = cloneYAMLValue(overrideValue)
			continue
		}
		merged[key] = mergeYAMLValues(baseValue, overrideValue)
	}
	return merged
}

func mergeYAMLValues(base any, override any) any {
	baseMap, baseIsMap := base.(map[string]any)
	overrideMap, overrideIsMap := override.(map[string]any)
	if baseIsMap && overrideIsMap {
		return mergeYAMLMaps(baseMap, overrideMap)
	}

	return cloneYAMLValue(override)
}

func cloneYAMLValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			out[k] = cloneYAMLValue(child)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = cloneYAMLValue(child)
		}
		return out
	default:
		return t
	}
}

func normalizeManifest(m *Manifest) {
	for i := range m.Assets {
		m.Assets[i].Source = canonicalSourcePath(m.Assets[i].Source)
	}
}

func canonicalSourcePath(raw string) string {
	norm := strings.TrimSpace(raw)
	if norm == "" {
		return ""
	}
	// Manifest paths are treated as lexical relative paths, not filesystem-resolved
	// paths, so normalize separators and dot segments deterministically.
	norm = strings.ReplaceAll(norm, "\\", "/")
	norm = path.Clean(norm)
	if norm == "." {
		return ""
	}
	return norm
}
