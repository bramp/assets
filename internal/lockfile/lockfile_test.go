package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

// TODO: Add cross-process concurrency tests that run multiple build/update
// writers against the same lockfile path and assert no entry loss.
// TODO: Add lock acquisition timeout/interruption tests once lock strategy
// includes bounded wait behavior.

func TestLoad_NotExistsReturnsNew(t *testing.T) {
	t.Parallel()

	f, err := Load(filepath.Join(t.TempDir(), "missing.lock"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Version != "1.0" {
		t.Fatalf("unexpected version: %q", f.Version)
	}
	if f.Files == nil {
		t.Fatal("expected initialized files map")
	}
}

func TestUpsertSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.lock")

	f := New()
	f.UpsertOutput(
		map[string]SourceRef{"raw/in.svg": {SHA256: "deadbeef", SizeBytes: 321}},
		"assets/out.png",
		"feedface",
		1234,
		&Provenance{
			CommandChain: []string{"tool-a in out", "tool-b out"},
			Tools:        map[string]string{"host_uname": "Darwin test", "tool-a": "1.0.0"},
		},
	)
	if err := f.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load round-trip: %v", err)
	}

	o := loaded.Files["assets/out.png"]
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

func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad lockfile: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestSave_ErrorWhenParentIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}

	f := New()
	if err := f.Save(filepath.Join(parent, "assets.lock")); err == nil {
		t.Fatal("expected save error when parent path is not a directory")
	}
}

func TestSave_GoldenOutput(t *testing.T) {
	t.Parallel()

	f := New()
	f.UpsertOutput(
		map[string]SourceRef{"raw/logo.svg": {SHA256: "abc123", SizeBytes: 1111}},
		"assets/images/logo_128.png",
		"aaa111",
		2048,
		&Provenance{
			CommandChain: []string{"resvg --width 128 --height 128 raw/logo.svg assets/images/logo_128.png", "oxipng -o 3 --strip safe --out assets/images/logo_128.png assets/images/logo_128.png"},
			Tools: map[string]string{
				"host_uname": "Darwin test",
				"resvg":      "0.42.0",
				"oxipng":     "9.1.3",
			},
		},
	)
	f.UpsertOutput(
		map[string]SourceRef{"raw/photo.jpg": {SHA256: "def456", SizeBytes: 2222}},
		"assets/images/photo_1024.jpg",
		"bbb222",
		8192,
		&Provenance{
			CommandChain: []string{"magick raw/photo.jpg assets/images/photo_1024.jpg", "jpegoptim --strip-all assets/images/photo_1024.jpg"},
			Tools: map[string]string{
				"host_uname": "Darwin test",
				"magick":     "7.1.1",
				"jpegoptim":  "1.5.5",
			},
		},
	)
	// Keep golden output stable for snapshot comparisons.
	f.LastUpdatedAt = "2026-01-02T03:04:05Z"

	path := filepath.Join(t.TempDir(), "assets.lock")
	if err := f.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved lockfile: %v", err)
	}

	goldenPath := filepath.Join("testdata", "lockfile.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}
