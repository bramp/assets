package lockfile

// Package lockfile contains assets.lock data structures and serialization logic.
//
// Purpose
//
// The lockfile records build results for generated assets so verification can
// be fast, deterministic, and auditable. It lets "assets verify" detect drift
// between the manifest, source files, generated outputs, and execution
// environment without re-running expensive transforms in CI.
//
// Schema intent
//
// The lockfile is keyed by generated file path (`files`) because the generated
// artifact is the stable verification target. Each generated file entry stores:
//   - sources: map keyed by source path with per-source hash and size metadata
//   - sha256: content hash of the generated file on disk
//   - size_bytes: output byte size used for quick mismatch detection
//   - provenance: command chain and tool versions used to build output
//
// Design principles
//
//   - Output-centric indexing: prioritize lookup by generated file path.
//   - Deterministic serialization: stable JSON formatting and key ordering.
//   - Incremental updates: upserts mutate only touched generated file entries.
//   - Reproducibility checks: source hashes + provenance + output hash/size.
//   - Human-readable diagnostics: schema should aid debugging in CI failures.
