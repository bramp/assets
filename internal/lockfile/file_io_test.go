package lockfile_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bramp/assets/internal/lockfile"
)

func TestSave_ErrorWhenParentIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}

	ls, err := lockfile.Open(filepath.Join(parent, "assets.lock"))
	if err != nil {
		return
	}
	defer func() { _ = ls.Close() }()

	ls.UpsertOutput("assets/out.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/in.svg": {SHA256: "abc123", SizeBytes: 1}},
		SHA256:    "deadbeef",
		SizeBytes: 100,
	})
	if err := ls.Save(); err == nil {
		t.Fatal("expected save error when parent path is not a directory")
	}
}

func TestSave_ZeroValueFileUsesDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed empty lockfile: %v", err)
	}

	ls, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open saved lockfile: %v", err)
	}
	defer func() { _ = ls.Close() }()

	snapshot := ls.Snapshot()
	if snapshot.Version != "1.0" {
		t.Fatalf("unexpected version: %q", snapshot.Version)
	}
	if snapshot.Files == nil {
		t.Fatal("expected files map to be initialized")
	}
}

func TestSave_GoldenOutput(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")
	ls, err := lockfile.Open(path)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer func() { _ = ls.Close() }()

	ls.UpsertOutput("assets/images/logo_128.png", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/logo.svg": {SHA256: "abc123", SizeBytes: 1111}},
		SHA256:    "aaa111",
		SizeBytes: 2048,
		Provenance: &lockfile.Provenance{
			CommandChain: []string{
				"resvg --width 128 --height 128 raw/logo.svg assets/images/logo_128.png",
				"oxipng -o 3 --strip safe --out assets/images/logo_128.png assets/images/logo_128.png",
			},
			Tools: map[string]string{
				"host_uname": "Darwin test",
				"resvg":      "0.42.0",
				"oxipng":     "9.1.3",
			},
		},
	})
	ls.UpsertOutput("assets/images/photo_1024.jpg", lockfile.GeneratedRef{
		Sources:   map[string]lockfile.SourceRef{"raw/photo.jpg": {SHA256: "def456", SizeBytes: 2222}},
		SHA256:    "bbb222",
		SizeBytes: 8192,
		Provenance: &lockfile.Provenance{
			CommandChain: []string{
				"magick raw/photo.jpg assets/images/photo_1024.jpg",
				"jpegoptim --strip-all assets/images/photo_1024.jpg",
			},
			Tools: map[string]string{
				"host_uname": "Darwin test",
				"magick":     "7.1.1",
				"jpegoptim":  "1.5.5",
			},
		},
	})
	if err := ls.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved lockfile: %v", err)
	}

	var got lockfile.File
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("decode saved lockfile: %v", err)
	}
	// Keep golden output stable for snapshot comparisons.
	got.LastUpdatedAt = "2026-01-02T03:04:05Z"
	gotBytes, err = json.MarshalIndent(&got, "", "  ")
	if err != nil {
		t.Fatalf("encode normalized lockfile: %v", err)
	}
	gotBytes = append(gotBytes, '\n')

	goldenPath := filepath.Join("testdata", "lockfile.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(gotBytes) != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", string(gotBytes), string(want))
	}
}
