package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunVerifyLock_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}
	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 0 {
		t.Fatalf("verify-lock failed: %s", stderr.String())
	}
}

func TestRunVerifyLock_SourceMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}

	if err := os.WriteFile(filepath.Join(dir, "raw", "in.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}

	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 1 {
		t.Fatalf("expected verify-lock failure, got %d", exit)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected verify-lock stderr")
	}

	golden, err := os.ReadFile(filepath.Join("testdata", "verify_lock_source_mismatch.golden.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if diff := cmp.Diff(strings.TrimSpace(string(golden)), strings.TrimSpace(stderr.String())); diff != "" {
		t.Fatalf("verify-lock mismatch output (-want +got):\n%s", diff)
	}
}

func TestEndToEnd_CheckGenBuildVerify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	// Add strict-mode legal fields for this end-to-end validation.
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	updated := strings.ReplaceAll(string(manifestBytes), "source: \"raw/in.txt\"", "source: \"raw/in.txt\"\n    owner: \"A\"\n    copyright: \"C\"\n    license: \"L\"")
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer

	if exit := RunCheck([]string{"--manifest", manifestPath, "--strict"}, &stderr); exit != 0 {
		t.Fatalf("check failed: %s", stderr.String())
	}
	stderr.Reset()

	if exit := RunGen([]string{"--manifest", manifestPath}, &out, &stderr); exit != 0 {
		t.Fatalf("gen failed: %s", stderr.String())
	}
	if !strings.Contains(out.String(), "out/out.txt: raw/in.txt") {
		t.Fatalf("unexpected gen output: %s", out.String())
	}
	out.Reset()
	stderr.Reset()

	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}
	stderr.Reset()

	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 0 {
		t.Fatalf("verify-lock failed: %s", stderr.String())
	}
}

func TestRunVerifyLock_PositionalArgsFail(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if exit := RunVerifyLock([]string{"extra"}, &stderr); exit != 1 {
		t.Fatalf("expected positional-arg failure, got %d", exit)
	}
}

func TestRunVerifyLock_MissingOutputOnDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}

	if err := os.Remove(filepath.Join(dir, "out", "out.txt")); err != nil {
		t.Fatalf("remove output: %v", err)
	}

	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 1 {
		t.Fatalf("expected verify-lock failure, got %d", exit)
	}
	if !strings.Contains(stderr.String(), "missing on disk") {
		t.Fatalf("expected missing-on-disk error, got: %s", stderr.String())
	}
}

func TestRunVerifyLock_ProvenanceMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := writePipelineFixture(t, dir)

	var stderr bytes.Buffer
	if exit := RunBuildTarget([]string{"--manifest", manifestPath, "--target", "out/out.txt"}, &stderr); exit != 0 {
		t.Fatalf("build-target failed: %s", stderr.String())
	}

	lockPath := filepath.Join(dir, "assets.lock")
	b, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	// mutate provenance command chain so verify-lock detects mismatch
	mutated := strings.Replace(string(b), "cp {input} {output}", "cp {input} {output} #changed", 1)
	if err := os.WriteFile(lockPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	stderr.Reset()
	if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 1 {
		t.Fatalf("expected verify-lock failure, got %d", exit)
	}
	if !strings.Contains(stderr.String(), "provenance mismatch") {
		t.Fatalf("expected provenance mismatch error, got: %s", stderr.String())
	}
}

func TestRunVerifyLock_OtherFailures(t *testing.T) {
	t.Parallel()

	t.Run("manifest load failure", func(t *testing.T) {
		var stderr bytes.Buffer
		if exit := RunVerifyLock([]string{"--manifest", "missing.yaml"}, &stderr); exit != 1 {
			t.Fatalf("expected load failure, got %d", exit)
		}
	})

	t.Run("lockfile load failure", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := writePipelineFixture(t, dir)

		var stderr bytes.Buffer
		if exit := RunVerifyLock([]string{"--manifest", manifestPath, "--lock", "."}, &stderr); exit != 1 {
			t.Fatalf("expected lockfile load failure, got %d", exit)
		}
	})

	t.Run("asset missing from lockfile", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := writePipelineFixture(t, dir)

		if err := os.WriteFile(filepath.Join(dir, "assets.lock"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write lockfile: %v", err)
		}

		var stderr bytes.Buffer
		if exit := RunVerifyLock([]string{"--manifest", manifestPath}, &stderr); exit != 1 {
			t.Fatalf("expected missing-asset failure, got %d", exit)
		}
		if !strings.Contains(stderr.String(), "output \"out/out.txt\" missing from lockfile") {
			t.Fatalf("expected missing output message, got: %s", stderr.String())
		}
	})
}

func writePipelineFixture(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "raw", "in.txt")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	manifest := "meta:\n" +
		"  project: \"test\"\n" +
		"  render:\n" +
		"    defaults:\n" +
		"      tools: [\"copy\"]\n" +
		"    tools:\n" +
		"      copy:\n" +
		"        tool: \"cp\"\n" +
		"        command: \"cp {input} {output}\"\n" +
		"        accepts: [\".txt\"]\n" +
		"        produces: [\".txt\"]\n" +
		"assets:\n" +
		"  - id: \"a\"\n" +
		"    source: \"raw/in.txt\"\n" +
		"    outputs:\n" +
		"      - path: \"out/out.txt\"\n" +
		"        width: 1\n" +
		"        height: 1\n" +
		"        options:\n" +
		"          scale_mode: \"fit\"\n" +
		"          background: \"transparent\"\n"
	manifestPath := filepath.Join(dir, "assets.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifestPath
}
