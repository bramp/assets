package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bramp/assets/internal/commands"
)

func TestRunDoctor_ArgErrors(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exit := commands.RunDoctor([]string{"--unknown"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected parse failure, got %d", exit)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := commands.RunDoctor([]string{"extra"}, &stdout, &stderr); exit != 1 {
		t.Fatalf("expected positional argument failure, got %d", exit)
	}
}

func TestRunDoctor_MissingTool(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writeDoctorManifest(t, dir, "definitely-missing-binary")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := commands.RunDoctor([]string{"--manifest", manifestPath}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf(
			"expected doctor failure for missing binary, got %d\nstdout=%s\nstderr=%s",
			exit,
			stdout.String(),
			stderr.String(),
		)
	}
	if !strings.Contains(stdout.String(), "missing tools:") {
		t.Fatalf("expected missing tools section, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "definitely-missing-binary") {
		t.Fatalf("expected missing binary name in output, got: %s", stdout.String())
	}
}

func TestRunDoctor_VersionMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writeDoctorManifest(t, dir, "go")
	lockPath := filepath.Join(dir, "assets.lock")
	lockContent := `{
  "version": "1.0",
  "files": {
    "out/out.txt": {
      "sources": {
        "raw/in.txt": {
          "sha256": "abc",
          "size_bytes": 1
        }
      },
      "provenance": {
        "command_chain": ["go version"],
        "tools": {
          "go": "go version go0.0.0"
        }
      },
      "sha256": "def",
      "size_bytes": 1
    }
  }
}
`
	if err := os.WriteFile(lockPath, []byte(lockContent), 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := commands.RunDoctor([]string{"--manifest", manifestPath, "--lock", "assets.lock"}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf(
			"expected doctor mismatch failure, got %d\nstdout=%s\nstderr=%s",
			exit,
			stdout.String(),
			stderr.String(),
		)
	}
	if !strings.Contains(stdout.String(), "lockfile version mismatches") {
		t.Fatalf("expected version mismatch section, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `tool "go" version mismatch`) {
		t.Fatalf("expected go version mismatch details, got: %s", stdout.String())
	}
}

func writeDoctorManifest(t *testing.T, dir string, toolBinary string) string {
	t.Helper()

	sourcePath := filepath.Join(dir, "raw", "in.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	manifestBody := "meta:\n" +
		"  project: \"doctor-test\"\n" +
		"  render:\n" +
		"    defaults:\n" +
		"      tools: [\"copy\"]\n" +
		"    tools:\n" +
		"      copy:\n" +
		"        tool: \"" + toolBinary + "\"\n" +
		"        command: \"" + toolBinary + " version\"\n" +
		"        accepts: [\".txt\"]\n" +
		"        produces: [\".txt\"]\n" +
		"assets:\n" +
		"  - source: \"raw/in.txt\"\n" +
		"    outputs:\n" +
		"      - path: \"out/out.txt\"\n" +
		"        width: 1\n" +
		"        height: 1\n" +
		"        options:\n" +
		"          scale_mode: \"fit\"\n" +
		"          background: \"transparent\"\n"

	manifestPath := filepath.Join(dir, "assets.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestBody), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	return manifestPath
}
