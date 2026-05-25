# Implementation TODO (for assets)

## Phase 0: Bootstrap

- [x] Initialize module as github.com/bramp/assets.
- [x] Set Go version to latest stable used by your updater workflow (go 1.26).
- [x] Add project skeleton:
  - [x] cmd/assets/main.go
  - [x] internal/manifest/
  - [x] internal/lockfile/
  - [x] internal/commands/
  - [x] internal/render/
  - [x] internal/hash/
- [x] Add baseline sample files for local iteration:
  - [x] assets.yaml
  - [x] raw_sources/ with at least one PNG and one SVG input.

## Phase 1: Tooling and Standards

- [x] Add Makefile with required targets:
  - [x] all: format analyze test
  - [x] format: go fmt ./... && goimports -w .
  - [x] analyze: go vet ./... && staticcheck ./...
  - [x] test: go test ./...
  - [x] test-ci: go test -v ./...
  - [x] fix: go fmt ./... && go fix ./...
  - [x] upgrade: go mod tidy && go get -u ./... && go mod tidy
- [x] Add CI workflow for Go tests and analysis:
  - [x] Use actions/setup-go@v5.
  - [x] Install staticcheck and goimports.
  - [x] Cache Go modules.
  - [x] Run make test-ci.
- [x] Add .github/dependabot.yml with cooldown policy.

## Phase 2: Manifest and Validation (check)

- [x] Define manifest structs matching design schema (meta, assets, outputs, options, format_options).
- [x] Implement YAML parsing with strict unknown-field detection.
- [x] Implement semantic validation:
  - [x] Ensure strict mode requires legal metadata fields (owner, copyright, license).
  - [x] Ensure loose mode allows missing legal metadata fields.
  - [x] Ensure source paths exist.
  - [x] Ensure output paths are unique across all assets.
  - [x] Ensure dimensions are positive integers.
  - [x] Validate scale_mode enum (fit, fill, stretch, crop).
  - [x] Validate background value (transparent or #RRGGBB).
- [x] Implement assets check command:
  - [x] Human-readable errors to stderr.
  - [x] Exit 0 on success, 1 on failure.

## Phase 3: Makefile Fragment Generation (gen)

- [ ] Implement assets gen:
  - [ ] Emit deterministic GENERATED_ASSET_FILES := ... ordering.
  - [ ] Emit explicit output-to-source dependency lines.
  - [ ] Emit stable, reproducible output formatting.
- [ ] Add tests that snapshot generated .assets.mk output.

## Phase 4: Single Target Rendering (build-target)

- [ ] Implement target lookup by exact output path.
- [ ] Implement image processing pipeline:
  - [ ] Load PNG/JPEG source images.
  - [ ] Rasterize SVG inputs.
  - [ ] Apply resize strategy for each scale_mode.
  - [ ] Apply optional background flattening.
  - [ ] Apply output format/compression settings.
- [ ] Ensure target output directory exists before writing.
- [ ] Implement assets build-target --target <path> command.

## Phase 5: Lockfile and Determinism

- [ ] Define lockfile structs with schema versioning.
- [ ] Implement source SHA-256 hashing.
- [ ] Implement config_hash from deterministic serialization of output dimensions + options.
- [ ] Update only the relevant target entry during build-target while preserving other entries.
- [ ] Write lockfile in deterministic JSON (stable key ordering, stable formatting).
- [ ] Record output size_bytes.

## Phase 6: Verification (verify-lock)

- [ ] Implement assets verify-lock:
  - [ ] Compare source hashes against lockfile.
  - [ ] Compare computed config_hash against lockfile.
  - [ ] Verify each declared output exists and size matches lockfile when required.
  - [ ] Exit non-zero with actionable mismatch diagnostics.
- [ ] Add CI job step to run assets verify-lock.

## Phase 7: Testing and Quality Gates

- [ ] Unit tests:
  - [ ] Manifest parse/validation edge cases.
  - [ ] Hash determinism and config hash stability.
  - [ ] Lockfile read/merge/write behavior.
- [ ] Golden tests:
  - [ ] gen output.
  - [ ] verify-lock mismatch messages.
- [ ] Integration tests:
  - [ ] End-to-end check -> gen -> build-target -> verify-lock.
- [ ] Run and keep clean:
  - [ ] go fmt ./...
  - [ ] goimports -w .
  - [ ] go vet ./...
  - [ ] staticcheck ./...
  - [ ] go test ./...
  - [ ] Maintain high coverage targets (>=85% repo, >=90% core packages).
  - [ ] Treat lint warnings as CI failures.

## Phase 8: Developer Experience and Docs

- [ ] Add README with quickstart:
  - [ ] Example assets.yaml.
  - [ ] Example root Makefile wiring.
  - [ ] Command reference (check, gen, build-target, verify-lock).
- [ ] Document failure modes and recovery flow (run make locally, commit updated assets + lockfile).
- [ ] Add release checklist for new image-option semantics.

## Phase 9: Optional Hardening

- [ ] Add gocyclo check in make analyze with threshold < 25.
- [ ] Add path safety checks to prevent writes outside repository root.
- [ ] Add parallel-safe lockfile update strategy if build-target runs concurrently.
- [ ] Add feature flags or versioned options for future transform engines.
