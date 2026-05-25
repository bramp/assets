package lockfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type File struct {
	Version     string              `json:"version"`
	GeneratedAt string              `json:"generated_at"`
	Assets      map[string]AssetRef `json:"assets"`
}

type AssetRef struct {
	SourcePath   string               `json:"source_path"`
	SourceSHA256 string               `json:"source_sha256"`
	Outputs      map[string]OutputRef `json:"outputs"`
}

type OutputRef struct {
	Provenance *Provenance `json:"provenance,omitempty"`
	SizeBytes  int64       `json:"size_bytes"`
}

type Provenance struct {
	CommandChain []string          `json:"command_chain"`
	Tools        map[string]string `json:"tools"`
}

func Load(path string) (*File, error) {
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
	if f.Assets == nil {
		f.Assets = make(map[string]AssetRef)
	}
	if f.Version == "" {
		f.Version = "1.0"
	}
	return &f, nil
}

func New() *File {
	return &File{
		Version: "1.0",
		Assets:  make(map[string]AssetRef),
	}
}

func (f *File) UpsertOutput(assetID, sourcePath, sourceSHA, outputPath string, sizeBytes int64, provenance *Provenance) {
	if f.Assets == nil {
		f.Assets = make(map[string]AssetRef)
	}

	asset := f.Assets[assetID]
	asset.SourcePath = sourcePath
	asset.SourceSHA256 = sourceSHA
	if asset.Outputs == nil {
		asset.Outputs = make(map[string]OutputRef)
	}
	asset.Outputs[outputPath] = OutputRef{
		Provenance: provenance,
		SizeBytes:  sizeBytes,
	}
	f.Assets[assetID] = asset
	f.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
}

func (f *File) Save(path string) error {
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
