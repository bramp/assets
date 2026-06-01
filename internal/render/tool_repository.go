package render

import (
	"context"
	"os/exec"
	"strings"
	"sync"
)

// ToolRepository provides runtime metadata and capability checks for tools.
//
// A shared repository instance allows resolve and provenance to reuse the same
// availability/version probes without repeating subprocess calls.
type ToolRepository interface {
	Available(toolName string) bool
	Version(step ResolvedStep) string
}

// NewToolRepository returns a process-local repository backed by command probes.
func NewToolRepository() ToolRepository {
	return &commandToolRepository{
		available: map[string]bool{},
		versions:  map[string]string{},
	}
}

type commandToolRepository struct {
	mu        sync.Mutex
	available map[string]bool
	versions  map[string]string
	lookPath  func(string) error
	runProbe  func(string, []string) string
}

func (r *commandToolRepository) Available(toolName string) bool {
	binary := firstCommandToken(toolName)
	if binary == "" {
		return false
	}

	r.mu.Lock()
	if ok, seen := r.available[binary]; seen {
		r.mu.Unlock()
		return ok
	}
	r.mu.Unlock()

	lookPath := r.lookPath
	if lookPath == nil {
		lookPath = defaultLookPath
	}
	err := lookPath(binary)
	ok := err == nil

	r.mu.Lock()
	r.available[binary] = ok
	r.mu.Unlock()

	return ok
}

// TODO(bramp): Add a version command to the tool config and use that instead of best-effort guessing, which may be slow and unreliable.
func (r *commandToolRepository) Version(step ResolvedStep) string {
	binary := firstCommandToken(step.Tool)
	if binary == "" {
		return ""
	}

	r.mu.Lock()
	if value, seen := r.versions[binary]; seen {
		r.mu.Unlock()
		return value
	}
	r.mu.Unlock()

	versionArgsSet := versionArgCandidates(step.VersionArgs)

	if !r.Available(step.Tool) {
		r.mu.Lock()
		r.versions[binary] = ""
		r.mu.Unlock()
		return ""
	}

	for _, args := range versionArgsSet {
		runProbe := r.runProbe
		if runProbe == nil {
			runProbe = runVersionProbe
		}
		line := runProbe(binary, args)
		r.mu.Lock()
		r.versions[binary] = line
		r.mu.Unlock()
		if line != "" {
			return line
		}
	}

	return ""
}

func defaultLookPath(binary string) error {
	_, err := exec.LookPath(binary)
	return err
}

func versionArgCandidates(explicit []string) [][]string {
	if len(explicit) > 0 {
		return [][]string{explicit}
	}
	//nolint:goconst // Keep probe candidates inlined here for local readability.
	return [][]string{{"--version"}, {"-version"}, {"version"}}
}

func runVersionProbe(binary string, args []string) string {
	out, err := exec.CommandContext(context.Background(), binary, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return ""
	}
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return line
}
