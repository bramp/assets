package hash

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := FileSHA256(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("unexpected sha256: got %s want %s", got, want)
	}
}

func TestFileSHA256_NotFound(t *testing.T) {
	t.Parallel()

	if _, err := FileSHA256(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("expected not-found error")
	}
}
