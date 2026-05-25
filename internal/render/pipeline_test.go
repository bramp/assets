package render

import (
	"testing"

	"github.com/bramp/assets/internal/manifest"
)

func TestResolvePipeline_Override(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Profile: "default"},
				Profiles: map[string]manifest.RenderProfile{
					"default": {
						Pipeline: []manifest.PipelineStep{{Tool: "cp", Command: "cp {input} {output}"}},
					},
				},
			},
		},
	}

	o := manifest.Output{Options: manifest.Options{PipelineOverride: []manifest.PipelineStep{{Tool: "echo", Command: "echo hi > {output}"}}}}
	steps, err := ResolvePipeline(m, o)
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 1 || steps[0].Tool != "echo" {
		t.Fatalf("unexpected override steps: %+v", steps)
	}
}

func TestResolvePipeline_Append(t *testing.T) {
	t.Parallel()

	m := &manifest.Manifest{
		Meta: manifest.Meta{
			Render: manifest.RenderConfig{
				Defaults: manifest.RenderDefaults{Profile: "default"},
				Profiles: map[string]manifest.RenderProfile{
					"default": {
						Pipeline: []manifest.PipelineStep{{Tool: "cp", Command: "cp {input} {tmp}"}},
					},
				},
			},
		},
	}

	o := manifest.Output{Options: manifest.Options{PipelineAppend: []manifest.PipelineStep{{Tool: "mv", Command: "mv {tmp} {output}"}}}}
	steps, err := ResolvePipeline(m, o)
	if err != nil {
		t.Fatalf("resolve pipeline: %v", err)
	}
	if len(steps) != 2 || steps[1].Tool != "mv" {
		t.Fatalf("unexpected appended steps: %+v", steps)
	}
}
