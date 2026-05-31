package lockfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bramp/assets/internal/lockfile"
)

// TODO: Add cross-process concurrency tests that run multiple build/update
// writers against the same lockfile path and assert no entry loss.
// TODO: Add lock acquisition timeout/interruption tests once lock strategy
// includes bounded wait behavior.

func TestOpen_NotExistsReturnsNew(t *testing.T) {
	t.Parallel()

	ls, err := lockfile.Open(filepath.Join(t.TempDir(), "missing.lock"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = ls.Close() }()

	f := ls.Snapshot()
	if f.Version != "1.0" {
		t.Fatalf("unexpected version: %q", f.Version)
	}
	if f.Files == nil {
		t.Fatal("expected initialized files map")
	}
}

func TestUpsertSaveOpen_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.lock")

	ls, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ls.UpsertOutput("assets/out.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/in.svg": {SHA256: "deadbeef", SizeBytes: 321}},
		SHA256:    "feedface",
		SizeBytes: 1234,
		Provenance: &lockfile.Provenance{
			CommandChain: []string{"tool-a in out", "tool-b out"},
			Tools:        map[string]string{"host_uname": "Darwin test", "tool-a": "1.0.0"},
		},
	})
	saveErr := ls.Save()
	if saveErr != nil {
		t.Fatalf("save: %v", saveErr)
	}
	closeErr := ls.Close()
	if closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	loaded, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open round-trip: %v", err)
	}
	defer func() { _ = loaded.Close() }()

	loadedSnapshot := loaded.Snapshot()

	o := loadedSnapshot.Files["assets/out.png"]
	src, ok := o.Sources["raw/in.svg"]
	if len(o.Sources) != 1 || !ok || src.SHA256 != "deadbeef" || src.SizeBytes != 321 {
		t.Fatalf("unexpected output source metadata: %+v", o.Sources)
	}
	if o.SHA256 != "feedface" {
		t.Fatalf("unexpected output hash: %q", o.SHA256)
	}
	if o.SizeBytes != 1234 {
		t.Fatalf("unexpected size bytes: %d", o.SizeBytes)
	}
	if o.Provenance == nil || len(o.Provenance.CommandChain) != 2 {
		t.Fatalf("unexpected provenance: %+v", o.Provenance)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if len(bytes) == 0 || bytes[len(bytes)-1] != '\n' {
		t.Fatal("expected newline-terminated lockfile")
	}
}

func TestOpen_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad lockfile: %v", err)
	}

	if _, err := lockfile.Open(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestSession_DeleteOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.lock")

	seed, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}

	seed.UpsertOutput("assets/out.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/in.svg": {SHA256: "deadbeef", SizeBytes: 321}},
		SHA256:    "feedface",
		SizeBytes: 1234,
	})
	if err := seed.Save(); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("seed close: %v", err)
	}

	ls, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ls.DeleteOutput("assets/out.png")
	saveErr := ls.Save()
	if saveErr != nil {
		t.Fatalf("save delete: %v", saveErr)
	}
	closeErr := ls.Close()
	if closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	loaded, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = loaded.Close() }()

	if _, ok := loaded.Snapshot().Files["assets/out.png"]; ok {
		t.Fatal("expected output to be deleted")
	}
}

func TestSession_SaveConflict(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")

	writerA, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open writer A: %v", err)
	}
	defer func() { _ = writerA.Close() }()

	writerB, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open writer B: %v", err)
	}
	defer func() { _ = writerB.Close() }()

	writerA.UpsertOutput("assets/a.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/a.svg": {SHA256: "aaa", SizeBytes: 1}},
		SHA256:    "111",
		SizeBytes: 10,
	})
	writerASaveErr := writerA.Save()
	if writerASaveErr != nil {
		t.Fatalf("writer A save: %v", writerASaveErr)
	}

	writerB.UpsertOutput("assets/b.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/b.svg": {SHA256: "bbb", SizeBytes: 2}},
		SHA256:    "222",
		SizeBytes: 20,
	})
	writerBSaveErr := writerB.Save()
	if !errors.Is(writerBSaveErr, lockfile.ErrConflict) {
		t.Fatalf("expected ErrConflict, got: %v", writerBSaveErr)
	}
}

func TestSession_SaveAfterClose(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")
	ls, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ls.UpsertOutput("assets/out.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/in.svg": {SHA256: "deadbeef", SizeBytes: 321}},
		SHA256:    "feedface",
		SizeBytes: 1234,
	})
	closeErr := ls.Close()
	if closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
	saveErr := ls.Save()
	if saveErr == nil {
		t.Fatal("expected save error after close")
	}
}
