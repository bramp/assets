# assets

Declarative asset pipeline and registry for image processing, metadata validation, and deterministic lockfile verification.

## Why This Project Exists

Modern apps accumulate many generated assets (logos, icons, screenshots, variants) that are easy to let drift. This project exists to make that process predictable and auditable.

It provides:

1. Asset management: a single manifest to declare source files and generated outputs.
2. Licensing/compliance tracking: owner/copyright/license metadata can be validated consistently.
3. A consistent processing pipeline: standardized build steps for transforming and compressing assets.
4. A cheap freshness check in CI: verify committed outputs and lockfile state without re-running full image processing on every build.

## When To Use

Use `assets` when you want to treat generated image assets like build artifacts that are versioned in git.

It is especially useful as a cheap CI gate to confirm generated assets are current and checked in:

1. Developers run `make` locally to regenerate changed outputs and update `assets.lock`.
2. CI runs `assets verify` to fail fast if committed assets/lockfile are stale.
3. CI can perform this check without re-running expensive image pipelines for every build.

## Quickstart

### 0) Install or run the CLI

Option A (preferred): install `assets` into your Go bin path.

```bash
go install github.com/bramp/assets/cmd/assets@latest
assets --help
```

Option B: run without installing.

```bash
alias assets='go run github.com/bramp/assets/cmd/assets@latest --'
assets --help
```

### 1) Create assets.yaml

Start with the smallest manifest and rely on sane built-in defaults.
You can customize the render pipeline later (see "Customize Pipeline").

```yaml
meta:
  project: "My App"

assets:
  - id: "logo"
    source: "raw/logo.svg"
    owner: "Example Org"
    copyright: "Copyright 2026"
    license: "Proprietary"
    outputs:
      - path: "assets/images/logo_128.png"
        width: 128
        height: 128
        options:
          scale_mode: "fit"
          background: "transparent"
```

### 2) Add root Makefile wiring

```makefile
.PHONY: all check-assets clean

# Load generated asset dependency rules if present.
-include .assets.mk

# Build all declared generated assets.
all: $(GENERATED_ASSET_FILES)
  @echo "assets up to date"

# Regenerate dependency rules whenever manifest changes.
.assets.mk: assets.yaml
  @assets gen > .assets.mk

# Build each output file via the assets CLI.
$(GENERATED_ASSET_FILES):
  @assets build --target $@

# Validate manifest and metadata policy in strict mode.
check-assets:
  @assets check --strict

# Remove generated assets and generated Makefile fragment.
clean:
  rm -f $(GENERATED_ASSET_FILES) .assets.mk
```

### 3) Run workflow locally

```bash
# Validate manifest schema and required legal metadata.
assets check --strict

# Build all declared outputs (Make regenerates .assets.mk as needed).
make

# Confirm lockfile, outputs, and provenance are in sync.
assets verify
```

## Customize Pipeline

When defaults are not enough, define render profiles and select one per output.

```yaml
meta:
  project: "My App"
  render:
    defaults:
      profile: "basic"
    profiles:
      basic:
        pipeline:
          # Convert SVG/vector input into an intermediate raster image.
          - stage: "rasterize"
            tool: "resvg"
            command: "resvg {input} {tmp}"
          # Resize or otherwise transform the intermediate image.
          - stage: "transform"
            tool: "vips"
            command: "vips resize {tmp} {tmp2} {scale}"
          # Encode the final output file at the target path.
          - stage: "encode"
            tool: "vips"
            command: "vips copy {tmp2} {output}"

assets:
  - id: "logo"
    source: "raw/logo.svg"
    outputs:
      - path: "assets/images/logo_128.png"
        width: 128
        height: 128
        options:
          profile: "basic"
          scale_mode: "fit"
          background: "transparent"
```

## Commands

- assets check
- assets gen
- assets build --target <path>
- assets verify

### assets check

Validates manifest structure and semantics.

- Use strict mode to enforce legal metadata:

```bash
# Strictly validate manifest and legal metadata.
assets check --strict
```

### assets gen

Emits deterministic Makefile fragment to stdout.

This is primarily useful for debugging or for non-Make workflows.

```bash
# Regenerate deterministic Makefile rules from assets.yaml.
assets gen > .assets.mk
```

### assets build

Builds one output and updates assets.lock provenance/size/source hash entry.

```bash
# Build a single declared output target.
assets build --target assets/images/logo_128.png
```

### assets verify

Verifies manifest, sources, lockfile, and output size/provenance alignment.

```bash
# Verify outputs and lockfile provenance are current.
assets verify
```

## Failure And Recovery

Common failures:

- source hash mismatch: source file changed after lockfile generation
- provenance mismatch: pipeline command chain/tool versions changed
- missing output: generated file missing from disk
- size mismatch: output differs from lockfile entry

Recovery flow:

1. Run build locally for affected targets (or run make).
2. Re-run assets verify.
3. Commit updated generated outputs plus assets.lock.

## Release Checklist

When changing rendering semantics or pipeline placeholders:

1. Update DESIGN.md with schema/behavior changes.
2. Add or update manifest validation tests.
3. Add or update build and verify tests.
4. Verify lockfile provenance behavior remains deterministic.
5. Run:
  - go fmt ./...
  - goimports -w .
  - go vet ./...
  - staticcheck ./...
  - go test ./...

## Coverage Trend Tracking

Coverage trend reporting is configured via Codecov and GitHub Actions:

1. CI generates `coverage.out` using `go test ./... -coverprofile=coverage.out -covermode=atomic`.
2. CI uploads coverage with `codecov/codecov-action` using OIDC (no long-lived token required).
3. Threshold policy is stored in `codecov.yml`.

## Development Plan

See TODO checklist in TODOs.md.

## Design

System design and architecture are documented in DESIGN.md.
