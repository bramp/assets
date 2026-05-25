package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

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
	Defaults RenderDefaults           `yaml:"defaults"`
	Profiles map[string]RenderProfile `yaml:"profiles"`
}

type RenderDefaults struct {
	Profile                string `yaml:"profile"`
	StrictRendererVersions bool   `yaml:"strict_renderer_versions"`
}

type RenderProfile struct {
	Pipeline      []PipelineStep         `yaml:"pipeline"`
	FormatOptions map[string]interface{} `yaml:"format_options"`
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
	ScaleMode        string                 `yaml:"scale_mode"`
	Background       string                 `yaml:"background"`
	Profile          string                 `yaml:"profile"`
	PipelineAppend   []PipelineStep         `yaml:"pipeline_append"`
	PipelineOverride []PipelineStep         `yaml:"pipeline_override"`
	FormatOptions    map[string]interface{} `yaml:"format_options"`
}

type PipelineStep struct {
	Stage   string `yaml:"stage"`
	Tool    string `yaml:"tool"`
	Command string `yaml:"command"`
}

type ValidationConfig struct {
	Strict  bool
	BaseDir string
}

func LoadFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}

	return &m, nil
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

			if profile := strings.TrimSpace(out.Options.Profile); profile != "" {
				if _, ok := m.Meta.Render.Profiles[profile]; !ok {
					errs = append(errs, fmt.Errorf("%s: options.profile %q does not exist in meta.render.profiles", outputRef, profile))
				}
			}

			if len(out.Options.PipelineAppend) > 0 {
				errs = append(errs, validatePipelineSteps(outputRef+" options.pipeline_append", out.Options.PipelineAppend)...)
			}
			if len(out.Options.PipelineOverride) > 0 {
				errs = append(errs, validatePipelineSteps(outputRef+" options.pipeline_override", out.Options.PipelineOverride)...)
			}
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

	if len(cfg.Profiles) == 0 {
		return errs
	}

	for name, profile := range cfg.Profiles {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Errorf("meta.render.profiles contains an empty profile name"))
			continue
		}
		err := validatePipelineSteps(fmt.Sprintf("meta.render.profiles[%q].pipeline", name), profile.Pipeline)
		errs = append(errs, err...)
	}

	if def := strings.TrimSpace(cfg.Defaults.Profile); def != "" {
		if _, ok := cfg.Profiles[def]; !ok {
			errs = append(errs, fmt.Errorf("meta.render.defaults.profile %q does not exist in meta.render.profiles", def))
		}
	}

	return errs
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
	}

	return errs
}
