package manifest

// Package manifest loads, merges, and validates assets.yaml manifests.
//
// Typical workflow:
//   - LoadFile reads a manifest from disk.
//   - Built-in render defaults are merged into user config.
//   - Manifest.Validate enforces structural, filesystem, and policy rules.
//
// Validation returns a list of independent errors so callers can show complete
// feedback in one pass rather than failing on first issue.
