package lockfile

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

const (
	dirPerm = 0o750
)

type hashDigest [sha256.Size]byte

// loadUnlocked reads and decodes the current lockfile content without taking
// the sidecar advisory lock. Callers that need a stable read for write-CAS must
// re-read after acquiring the lock.
func loadUnlocked(path string) (*File, hashDigest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), hashDigest{}, nil
		}
		return nil, hashDigest{}, err
	}

	f, err := decodeFile(b)
	if err != nil {
		return nil, hashDigest{}, err
	}
	return f, sha256.Sum256(b), nil
}

// writeCopyReplaceIfHashMatches stages next to a temp file, acquires the
// sidecar advisory lock, re-reads and hashes the current lockfile, and only
// atomically replaces path when expectedHash still matches current content.
// This provides optimistic conflict detection plus atomic rename durability.
func writeCopyReplaceIfHashMatches(path string, expectedHash hashDigest, next *File, beforeReplace func()) error {
	// Stage bytes first so lock hold time only covers compare-and-replace.
	tmpPath, err := stageTemp(path, next)
	if err != nil {
		return fmt.Errorf("stage temp file: %w", err)
	}
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	// This is an advisory sidecar lock: it only serializes writers that
	// cooperate by acquiring this same <lockfile>.lock path before writes.
	// Non-cooperative writers can still modify the lockfile directly; hash-CAS
	// validation in Save/Update detects those races as conflicts.
	unlock, err := lock(path)
	if err != nil {
		return err
	}
	defer unlock()

	_, currentHash, err := loadUnlocked(path)
	if err != nil {
		return fmt.Errorf("reload lockfile under lock: %w", err)
	}
	// Hash mismatch means another writer changed the file after caller's read.
	// Return a typed conflict so callers can retry with fresh state.
	if currentHash != expectedHash {
		return &ConflictError{Path: path, ExpectedHash: expectedHash, ActualHash: currentHash}
	}
	if beforeReplace != nil {
		beforeReplace()
	}

	// Rename in the same directory is atomic on POSIX filesystems.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %q with %q: %w", path, tmpPath, err)
	}
	tmpPath = ""

	// Sync the parent directory so the rename metadata is durably recorded.
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}

	return nil
}

func stageTemp(path string, f *File) (string, error) {
	// Stage in the target directory so final rename stays on the same filesystem
	// and can be atomic.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	// Keep canonical formatting stable for hashing and readable diffs.
	b = append(b, '\n')
	if _, err := tmp.Write(b); err != nil {
		return "", err
	}
	// Sync staged content before swap so data reaches storage prior to rename.
	if err := tmp.Sync(); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	cleanup = false
	return tmpPath, nil
}

func lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return nil, fmt.Errorf("ensure lock directory: %w", err)
	}

	// Use a sidecar file lock so lockfile content remains valid JSON at all times.
	lockPath := path + ".lock"
	fl := flock.New(lockPath)
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", lockPath, err)
	}

	return func() {
		_ = fl.Unlock()
		// Best-effort cleanup: remove sidecar lockfile after releasing the lock.
		// If another process races to acquire/create it, removal may fail and is
		// intentionally ignored because lock correctness does not depend on unlink.
		_ = os.Remove(lockPath)
	}, nil
}

func syncDir(dir string) error {
	// Directory sync persists metadata operations like rename/link updates.
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := d.Sync(); err != nil {
		// Some platforms/filesystems do not support directory sync.
		if errors.Is(err, os.ErrInvalid) {
			return nil
		}
		return err
	}
	return nil
}

func decodeFile(b []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	f.ensureDefaults()
	return &f, nil
}

func hashCanonical(f *File) (hashDigest, error) {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return hashDigest{}, err
	}
	b = append(b, '\n')
	return sha256.Sum256(b), nil
}
