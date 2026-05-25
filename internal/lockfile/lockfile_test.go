package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NotExistsReturnsNew(t *testing.T) {
	t.Parallel()

	f, err := Load(filepath.Join(t.TempDir(), "missing.lock"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Version != "1.0" {
		t.Fatalf("unexpected version: %q", f.Version)
	}
	if f.Assets == nil {
		t.Fatal("expected initialized assets map")
	}
}

func TestUpsertSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "assets.lock")

	f := New()
	f.UpsertOutput(
		"asset-a",
		"raw/in.svg",
		"deadbeef",
		"assets/out.png",
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

	a := loaded.Assets["asset-a"]
	if a.SourcePath != "raw/in.svg" || a.SourceSHA256 != "deadbeef" {
		t.Fatalf("unexpected asset metadata: %+v", a)
	}
	o := a.Outputs["assets/out.png"]
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
