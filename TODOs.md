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
  - [x] examples/complex/raw/ with at least one PNG and one SVG input.

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

## Phase 2: Manifest and Validation (verify)

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
  - [x] Validate graph tool schema (`tool`, `command`, `accepts`, `produces`, optional `scale_modes`, `size_template`, and `size_by_mode`).
  - [x] Enforce graph-only manifest keys (legacy profile/stage/pipeline controls rejected by strict decode/validation).
- [x] Implement assets verify command validation gate:
  - [x] Human-readable errors to stderr.
  - [x] Exit 0 on success, 1 on failure.

## Phase 3: Makefile Fragment Generation (gen)

- [x] Implement assets gen:
  - [x] Emit deterministic GENERATED_ASSET_FILES := ... ordering.
  - [x] Emit explicit output-to-source dependency lines.
  - [x] Emit stable, reproducible output formatting.
- [x] Add tests that snapshot generated .assets.mk output.

## Phase 4: Single Target Rendering (build)

- [x] Implement target lookup by exact output path.
- [ ] Implement image processing pipeline:
  - [ ] Load PNG/JPEG source images.
  - [ ] Rasterize SVG inputs.
  - [ ] Apply resize strategy for each scale_mode.
  - [ ] Apply optional background flattening.
  - [ ] Apply output format/compression settings.
  - [x] Support deterministic command chaining/postprocess steps (for example PNG optimization).
  - [x] Resolve effective ordered pipeline from graph tool preferences + compatibility constraints.
  - [x] Execute resolved pipeline with placeholder expansion (`{input}`, `{tmp}`, `{tmp2}`, `{output}`, derived values).
- [x] Ensure target output directory exists before writing.
- [x] Implement assets build --target <path> command.

## Phase 5: Lockfile and Determinism

- [x] Define lockfile structs with schema versioning.
- [x] Implement source SHA-256 hashing.
- [x] Prefer source hash + provenance checks over storing config_hash in lockfile.
- [x] Update only the relevant target entry during build while preserving other entries.
- [x] Write lockfile in deterministic JSON (stable key ordering, stable formatting).
- [x] Record output size_bytes.
- [x] Record provenance per output (command chain, tool versions, host OS fingerprint, key library versions).

## Phase 6: Verification (verify)

- [x] Implement assets verify:
  - [x] Compare source hashes against lockfile.
  - [x] Compare recorded source hash and provenance against lockfile.
  - [x] Compare recorded provenance (commands, tool versions, host OS fingerprint, key library versions) against current execution environment/policy.
  - [x] Verify each declared output exists and size matches lockfile when required.
  - [x] Exit non-zero with actionable mismatch diagnostics.
- [x] Add CI job step to run assets verify.

## Phase 7: Testing and Quality Gates

- [x] Unit tests:
  - [x] Manifest parse/validation edge cases.
  - [x] Hash determinism and file hash behavior.
  - [x] Lockfile read/merge/write behavior.
- [x] Golden tests:
  - [x] gen output.
  - [x] verify mismatch messages.
- [x] Integration tests:
  - [x] End-to-end verify -> gen -> build -> verify.
- [ ] Run and keep clean:
  - [x] go fmt ./...
  - [x] goimports -w .
  - [x] go vet ./...
  - [x] staticcheck ./...
  - [x] go test ./...
  - [ ] Maintain high coverage targets (>=85% repo, >=90% core packages). (Current: ~82.9%)
  - [x] Treat lint warnings as CI failures.

## Phase 8: Developer Experience and Docs

- [x] Add README with quickstart:
  - [x] Example assets.yaml.
  - [x] Example root Makefile wiring.
  - [x] Command reference (verify, gen, defaults, build).
- [x] Document failure modes and recovery flow (run make locally, commit updated assets + lockfile).
- [x] Add release checklist for new image-option semantics.
- [x] Add CI coverage upload and trend tracking with Codecov (OIDC).
- [x] Add a complex end-to-end example directory demonstrating graph tools, fallback ordering, per-output overrides, and multi-format outputs.

## Phase 9: Optional Hardening

- [x] Add gocyclo check in make analyze with threshold < 25. (Implemented with the current 30 threshold.)
- [x] Add path safety checks to prevent writes outside repository root. (Current source/output path normalization is in place.)
- [x] Add parallel-safe lockfile update strategy if build runs concurrently. (Lockfile updates now use advisory locking plus optimistic hash retry.)
  - [ ] Add same-target build serialization if build runs concurrently.
- [ ] Add feature flags or versioned options for future transform engines.
- [ ] Refactor in-place optimizer prep into explicit graph nodes:
  - [ ] Introduce a dedicated bitmap copy/stage node in `meta.render.tools` (for example `copy-bitmap`) rather than embedding `cp ... &&` in optimizer commands.
  - [ ] Keep optimizer nodes single-purpose (for example `jpegoptim` only runs optimizer flags).
  - [ ] Ensure graph resolution selects the copy node only when required by an in-place optimizer.
  - [ ] Update defaults and tests so planned/executed commands remain one command per step for clearer logs and failure attribution.

## Phase 10: UX and Onboarding Backlog

- [ ] Add `assets init` scaffolding command.
  - [ ] Detect common source layouts (for example `artwork/logo.svg`) and suggest output presets.
  - [ ] Start with a safe mode that prints proposed YAML before writing.
  - [ ] Support `--write` and `--force` explicitly to avoid accidental overwrites.
  - [ ] Add tests for deterministic generated manifest ordering/content.

- [x] Add `assets doctor` environment diagnostics command.
  - [x] Check required external binaries from effective tool graph.
  - [x] Report PATH lookup details and unavailable tools.
  - [x] Include tool version/provenance probe output and mismatch reasons.
  - [x] Provide actionable install hints per OS (macOS/Linux/Windows).

- [x] Improve `assets verify` remediation output.
  - [x] Print exact rebuild commands for failing outputs (`make <target>` and `assets build --target <target>`).
  - [x] Print files expected to be committed (affected outputs + lockfile).
  - [x] Print concise mismatch reason summary per target (source/provenance/hash/size/missing output).
  - [x] Keep output deterministic and test with golden fixtures.

- [x] Add a pre-commit hook snippet to README quickstart.
  - [x] Include copy/paste minimal hook wiring for strict `assets verify`.
  - [x] Keep repository-specific hook install section (`make hooks-install`) as canonical path.

- [ ] Add stable install guidance beyond `go install`.
  - [ ] Document release binary download flow once artifacts are published.
  - [ ] Add Homebrew tap instructions when tap/release automation exists.
  - [ ] Keep fallback `go install` path for contributors.

- [ ] Add first-class docs for copy-only outputs.
  - [ ] Document a minimal copy-only tool pattern for no-transform outputs.
  - [ ] Show example asset outputs that intentionally bypass resizing/optimization.
  - [ ] Clarify interaction with `kind: optimize` terminal steps.

- [ ] Add a deterministic “sync everything” command/workflow.
  - [ ] Evaluate `assets sync` vs documented Make target wrapper.
  - [ ] Target workflow: verify + gen + build all + verify.
  - [ ] Ensure deterministic output and clear failure boundaries for CI/local.

- [x] Fix README quickstart YAML duplicate `outputs` key typo.
  - [x] Keep both sample outputs under a single `outputs:` array.
  - [ ] Add test/docs lint check to prevent duplicate-key regressions in snippets.
