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
				Defaults: RenderDefaults{Profile: "missing"},
				Profiles: map[string]RenderProfile{
					"bad": {
						Pipeline: []PipelineStep{{Stage: "transform", Command: "echo hi"}},
					},
				},
			},
		},
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
					Profile:    "missing-profile",
					PipelineAppend: []PipelineStep{{
						Tool: "cp",
					}},
				},
			}},
		}},
	}

	errs := m.Validate(ValidationConfig{Strict: false, BaseDir: dir})
	joined := joinErrs(errs)
	for _, want := range []string{
		"meta.render.profiles[\"bad\"].pipeline[0]: tool is required",
		"meta.render.defaults.profile \"missing\" does not exist",
		"options.profile \"missing-profile\" does not exist",
		"options.pipeline_append[0]: command is required",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected error containing %q, got: %s", want, joined)
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
