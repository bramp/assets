package lockfile

const currentVersion = "1.0"

// File is the persisted lockfile payload for generated outputs.
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

// SourceRef captures source file metadata used during generation.
type SourceRef struct {
	// SHA256 is the lowercase hex SHA-256 digest of the source file content.
	// Expected value is a 64-character hex string.
	SHA256 string `json:"sha256"`
	// SizeBytes is the byte length of the source file at lockfile generation time.
	// Expected value is non-negative and should match the file size on disk.
	SizeBytes int64 `json:"size_bytes"`
}

// GeneratedRef stores metadata for one generated output artifact.
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

// Provenance records command chain and tool versions used for generation.
type Provenance struct {
	// CommandChain lists the exact commands executed to produce the output, in order.
	// Expected entries are deterministic command strings after placeholder expansion.
	CommandChain []string `json:"command_chain"`
	// Tools maps tool/runtime identifiers to detected versions or fingerprints.
	// Expected keys include tool names (for example, "resvg") and may include host markers.
	Tools map[string]string `json:"tools"`
}

// New returns an initialized lockfile value with schema defaults.
func New() *File {
	return &File{
		Version: currentVersion,
		Files:   make(map[string]GeneratedRef),
	}
}

func (f *File) ensureDefaults() {
	if f.Version == "" {
		f.Version = currentVersion
	}
	if f.Files == nil {
		f.Files = make(map[string]GeneratedRef)
	}
}
