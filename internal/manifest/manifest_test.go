package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_UnknownField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.yaml")
	data := `meta:
  project: "x"
  unknown_field: true
assets: []
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected unknown-field parse error")
	}
}

func TestLoadFile_AppliesBuiltInRenderDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.yaml")
	data := `meta:
  project: "x"
assets:
  - id: "a"
    source: "raw/in.svg"
    outputs:
      - path: "out/a.png"
        width: 1
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
`
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw", "in.svg"), []byte("<svg/>") , 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if len(m.Meta.Render.Tools) == 0 {
		t.Fatal("expected built-in render tools to be present")
	}
	if len(m.Meta.Render.Defaults.Tools) == 0 {
		t.Fatal("expected built-in default tool order to be present")
	}
	if _, ok := m.Meta.Render.Tools["resvg"]; !ok {
		t.Fatal("expected built-in resvg tool")
	}
}

func TestLoadFile_UserCanAmendBuiltInTool(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.yaml")
	data := `meta:
  project: "x"
  render:
    tools:
      resvg:
        command: "custom-resvg {input} {output}"
assets:
  - id: "a"
    source: "raw/in.svg"
    outputs:
      - path: "out/a.png"
        width: 1
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
          tools: "resvg"
`
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw", "in.svg"), []byte("<svg/>") , 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	step, ok := m.Meta.Render.Tools["resvg"]
	if !ok {
		t.Fatal("expected resvg tool after merge")
	}
	if got := step.Command; got != "custom-resvg {input} {output}" {
		t.Fatalf("unexpected command: %q", got)
	}
	if len(step.Accepts) == 0 || len(step.Produces) == 0 {
		t.Fatalf("expected accepts/produces inherited from built-in defaults, got %+v", step)
	}
}

func TestValidate_StrictVsLooseLegalFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sourceRel := filepath.Join("raw", "in.txt")
	sourceAbs := filepath.Join(dir, sourceRel)
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	m := &Manifest{
		Meta: Meta{Project: "test"},
		Assets: []Asset{{
			ID:     "a",
			Source: sourceRel,
			Outputs: []Output{{
				Path:   "out/a.txt",
				Width:  1,
				Height: 1,
				Options: Options{
					ScaleMode:  "fit",
					Background: "transparent",
				},
			}},
		}},
	}

	if errs := m.Validate(ValidationConfig{Strict: false, BaseDir: dir}); len(errs) != 0 {
		t.Fatalf("expected no loose-mode legal-field errors, got: %v", errs)
	}

	errs := m.Validate(ValidationConfig{Strict: true, BaseDir: dir})
	if len(errs) < 3 {
		t.Fatalf("expected strict legal-field errors, got: %v", errs)
	}
	joined := joinErrs(errs)
	for _, field := range []string{"owner is required in strict mode", "copyright is required in strict mode", "license is required in strict mode"} {
		if !strings.Contains(joined, field) {
			t.Fatalf("expected error containing %q, got %s", field, joined)
		}
	}
}

func TestValidate_RenderPipelineAndOutputControls(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.yaml")
	data := `meta:
  project: "test"
  render:
    defaults:
      profile: "legacy"
    profiles:
      legacy:
        pipeline:
          - tool: "cp"
            command: "cp {input} {output}"
assets:
  - id: "a"
    source: "raw/in.txt"
    outputs:
      - path: "out/a.txt"
        width: 1
        height: 1
        options:
          scale_mode: "fit"
          background: "transparent"
          pipeline_append:
            - tool: "cp"
              command: "cp {input} {output}"
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected decode error for legacy render keys")
	}
	if !strings.Contains(err.Error(), "field profile not found") && !strings.Contains(err.Error(), "field profiles not found") {
		t.Fatalf("expected unknown legacy-field error, got %v", err)
	}
}

func TestValidate_OptimizeByFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sourceRel := filepath.Join("raw", "in.txt")
	sourceAbs := filepath.Join(dir, sourceRel)
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	m := &Manifest{
		Meta: Meta{
			Project: "test",
			Render: RenderConfig{
				Defaults: RenderDefaults{Tools: []string{"oxipng"}},
				Tools: map[string]PipelineStep{
					"oxipng": {
						Tool:     "oxipng",
						Command:  "oxipng -o 3 --strip safe {output}",
						Accepts:  []string{".png"},
						Produces: []string{".png"},
					},
				},
				OptimizeByFormat: map[string]string{
					"png":  "oxipng",
					".gif": "missing",
				},
			},
		},
		Assets: []Asset{{
			ID:     "a",
			Source: sourceRel,
			Outputs: []Output{{
				Path:   "out/a.png",
				Width:  1,
				Height: 1,
				Options: Options{
					ScaleMode:  "fit",
					Background: "transparent",
				},
			}},
		}},
	}

	errStr := joinErrs(m.Validate(ValidationConfig{Strict: false, BaseDir: dir}))
	for _, want := range []string{
		"meta.render.optimize_by_format extension \"png\" must start with '.'",
		"meta.render.optimize_by_format[\".gif\"] references unknown optimize tool \"missing\"",
	} {
		if !strings.Contains(errStr, want) {
			t.Fatalf("expected error containing %q, got: %s", want, errStr)
		}
	}
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	if got := assetRef(Asset{ID: "x"}, 0); got != `asset["x"]` {
		t.Fatalf("unexpected assetRef with id: %q", got)
	}
	if got := assetRef(Asset{}, 7); got != "asset[7]" {
		t.Fatalf("unexpected assetRef without id: %q", got)
	}

	for _, tc := range []struct {
		v    string
		want bool
	}{
		{v: "fit", want: true},
		{v: "fill", want: true},
		{v: "stretch", want: true},
		{v: "crop", want: true},
		{v: "bogus", want: false},
	} {
		if got := validScaleMode(tc.v); got != tc.want {
			t.Fatalf("validScaleMode(%q)=%v want %v", tc.v, got, tc.want)
		}
	}

	for _, tc := range []struct {
		v    string
		want bool
	}{
		{v: "transparent", want: true},
		{v: "#A1B2C3", want: true},
		{v: "#abc123", want: true},
		{v: "#xyzxyz", want: false},
		{v: "white", want: false},
	} {
		if got := validBackground(tc.v); got != tc.want {
			t.Fatalf("validBackground(%q)=%v want %v", tc.v, got, tc.want)
		}
	}
}

func joinErrs(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n")
}
