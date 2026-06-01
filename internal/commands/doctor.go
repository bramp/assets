package commands

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/bramp/assets/internal/lockfile"
	"github.com/bramp/assets/internal/manifest"
	"github.com/bramp/assets/internal/render"
)

const multipleVersionsMarker = "<multiple-versions>"

type doctorToolStatus struct {
	name      string
	binary    string
	kind      string
	available bool
	version   string
}

// RunDoctor inspects local tool availability and version drift against lockfile provenance.
func RunDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)

	manifestPath := fs.String("manifest", "assets.yaml", "Path to assets manifest")
	lockPath := fs.String("lock", "assets.lock", "Path to lockfile")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "doctor: unexpected positional arguments: %v\n", fs.Args())
		return 1
	}

	m, err := manifest.LoadFile(*manifestPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "doctor: failed to load manifest %q: %v\n", *manifestPath, err)
		return 1
	}

	baseDir := filepath.Dir(*manifestPath)
	lockVersions, lockErr := loadLockToolVersions(filepath.Join(baseDir, *lockPath))
	if lockErr != nil {
		_, _ = fmt.Fprintf(stderr, "doctor: warning: failed to read lockfile %q: %v\n", *lockPath, lockErr)
	}

	statuses := collectDoctorToolStatuses(m.Meta.Render.Tools)
	missing := make([]doctorToolStatus, 0)
	mismatched := make([]string, 0)

	for _, status := range statuses {
		if !status.available {
			missing = append(missing, status)
			continue
		}

		if expected, ok := lockVersions[status.binary]; ok {
			switch {
			case expected == multipleVersionsMarker:
				mismatched = append(
					mismatched,
					fmt.Sprintf("tool %q has multiple recorded versions in lockfile provenance", status.binary),
				)
			case status.version == "":
				mismatched = append(
					mismatched,
					fmt.Sprintf("tool %q current version unknown, lockfile expects %q", status.binary, expected),
				)
			case status.version != expected:
				mismatched = append(
					mismatched,
					fmt.Sprintf(
						"tool %q version mismatch: current=%q lockfile=%q",
						status.binary,
						status.version,
						expected,
					),
				)
			}
		}
	}

	printDoctorSummary(stdout, statuses, missing, mismatched)

	if len(missing) > 0 || len(mismatched) > 0 {
		return 1
	}
	return 0
}

func collectDoctorToolStatuses(tools map[string]manifest.ToolSpec) []doctorToolStatus {
	keys := make([]string, 0, len(tools))
	for name := range tools {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	repo := render.NewToolRepository()
	statuses := make([]doctorToolStatus, 0, len(keys))
	for _, name := range keys {
		spec := tools[name]
		binary := doctorBinary(spec.Tool)
		if strings.TrimSpace(binary) == "" {
			continue
		}
		status := doctorToolStatus{
			name:      name,
			binary:    binary,
			kind:      spec.KindOrDefault(),
			available: repo.Available(spec.Tool),
		}
		if status.available {
			status.version = repo.Version(render.ResolvedStep{
				Tool:        spec.Tool,
				VersionArgs: spec.VersionArgs,
			})
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func doctorBinary(toolName string) string {
	binary := strings.TrimSpace(toolName)
	if binary == "" {
		return ""
	}
	parts := strings.Fields(binary)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func loadLockToolVersions(path string) (map[string]string, error) {
	lf, err := lockfile.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lf.Close() }()

	out := map[string]string{}
	for _, ref := range lf.Snapshot().Files {
		if ref.Provenance == nil {
			continue
		}
		for toolName, version := range ref.Provenance.Tools {
			normTool := strings.TrimSpace(toolName)
			normVersion := strings.TrimSpace(version)
			if normTool == "" || normTool == "host_uname" || normVersion == "" {
				continue
			}
			if existing, ok := out[normTool]; ok {
				if existing != normVersion {
					out[normTool] = multipleVersionsMarker
				}
				continue
			}
			out[normTool] = normVersion
		}
	}

	return out, nil
}

func printDoctorSummary(
	out io.Writer,
	statuses []doctorToolStatus,
	missing []doctorToolStatus,
	mismatched []string,
) {
	_, _ = fmt.Fprintf(out, "doctor: checked %d configured tools\n", len(statuses))

	for _, status := range statuses {
		availability := "missing"
		if status.available {
			availability = "ok"
		}
		version := status.version
		if strings.TrimSpace(version) == "" {
			version = "<unknown>"
		}
		_, _ = fmt.Fprintf(
			out,
			"doctor: tool=%q binary=%q kind=%q status=%s version=%q\n",
			status.name,
			status.binary,
			status.kind,
			availability,
			version,
		)
	}

	if len(missing) > 0 {
		_, _ = fmt.Fprintln(out, "doctor: missing tools:")
		for _, status := range missing {
			_, _ = fmt.Fprintf(out, "doctor:   %q not found in PATH (binary %q)\n", status.name, status.binary)
			_, _ = fmt.Fprintf(out, "doctor:     hint: %s\n", installHint(status.binary))
		}
	}

	if len(mismatched) > 0 {
		sort.Strings(mismatched)
		_, _ = fmt.Fprintln(out, "doctor: lockfile version mismatches:")
		for _, msg := range mismatched {
			_, _ = fmt.Fprintf(out, "doctor:   %s\n", msg)
		}
	}
}

func installHint(binary string) string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("brew install %s", binary)
	case "linux":
		return fmt.Sprintf("install %s via your distro package manager (for example: apt install %s)", binary, binary)
	case "windows":
		return fmt.Sprintf("install %s via winget/choco/scoop and ensure PATH is updated", binary)
	default:
		return fmt.Sprintf("install %s and ensure it is available in PATH", binary)
	}
}
