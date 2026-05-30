package lockfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type File struct {
	// Version is the lockfile schema version.
	// Expected value is "1.0" for the current schema.
	Version string `json:"version"`
	// LastUpdatedAt is when the lockfile was last modified, in UTC RFC3339 format.
	// Expected example: "2026-05-30T12:34:56Z".
	LastUpdatedAt string `json:"last_updated_at"`
	// Files maps generated output paths to their recorded build metadata.
	// Keys are repository-relative output paths exactly as declared in the manifest.
	Files map[string]GeneratedRef `json:"files"`
}

type SourceRef struct {
	// SHA256 is the lowercase hex SHA-256 digest of the source file content.
	// Expected value is a 64-character hex string.
	SHA256 string `json:"sha256"`
	// SizeBytes is the byte length of the source file at lockfile generation time.
	// Expected value is non-negative and should match the file size on disk.
	SizeBytes int64 `json:"size_bytes"`
}

type GeneratedRef struct {
	// Sources maps source paths to source file metadata used to produce this output.
	// Keys are repository-relative source paths exactly as declared in the manifest.
	Sources map[string]SourceRef `json:"sources"`
	// Provenance records executed command chain and detected tool/runtime versions.
	// This may be nil when provenance was not collected.
	Provenance *Provenance `json:"provenance,omitempty"`
	// SHA256 is the lowercase hex SHA-256 digest of the generated output content.
	// Expected value is a 64-character hex string.
	SHA256 string `json:"sha256"`
	// SizeBytes is the byte length of the generated output file.
	// Expected value is non-negative and should match the output size on disk.
	SizeBytes int64 `json:"size_bytes"`
}

type Provenance struct {
	// CommandChain lists the exact commands executed to produce the output, in order.
	// Expected entries are deterministic command strings after placeholder expansion.
	CommandChain []string `json:"command_chain"`
	// Tools maps tool/runtime identifiers to detected versions or fingerprints.
	// Expected keys include tool names (for example, "resvg") and may include host markers.
	Tools map[string]string `json:"tools"`
}

func Load(path string) (*File, error) {
	return loadUnlocked(path)
}

func loadUnlocked(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, err
	}

	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Files == nil {
		f.Files = make(map[string]GeneratedRef)
	}
	if f.Version == "" {
		f.Version = "1.0"
	}
	return &f, nil
}

func New() *File {
	return &File{
		Version: "1.0",
		Files:   make(map[string]GeneratedRef),
	}
}

func (f *File) UpsertOutput(sources map[string]SourceRef, outputPath, outputSHA string, sizeBytes int64, provenance *Provenance) {
	if f.Files == nil {
		f.Files = make(map[string]GeneratedRef)
	}

	sourcesCopy := make(map[string]SourceRef, len(sources))
	for path, src := range sources {
		sourcesCopy[path] = src
	}

	f.Files[outputPath] = GeneratedRef{
		Sources:    sourcesCopy,
		Provenance: provenance,
		SHA256:     outputSHA,
		SizeBytes:  sizeBytes,
	}
	f.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (f *File) Save(path string) error {
	return withLock(path, func() error {
		return f.saveUnlocked(path)
	})
}

func (f *File) saveUnlocked(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func withLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(path+".lck", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	return fn()
}

// Update acquires an exclusive file lock and runs a read-modify-write
// transaction against the lockfile.
// TODO: Add multi-process stress tests that run concurrent `assets build`
// commands against the same lockfile and validate no updates are lost.
func Update(path string, mutate func(f *File) error) error {
	return withLock(path, func() error {
		f, err := loadUnlocked(path)
		if err != nil {
			return err
		}
		if err := mutate(f); err != nil {
			return err
		}
		return f.saveUnlocked(path)
	})
}
