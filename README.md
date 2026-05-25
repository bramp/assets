# assets

Declarative asset pipeline and registry for image processing, metadata validation, and deterministic lockfile verification.

## Quickstart

### 1) Create assets.yaml

```yaml
meta:
  project: "My App"
  render:
    defaults:
      profile: "basic"
    profiles:
      basic:
        pipeline:
          - stage: "rasterize"
            tool: "resvg"
            command: "resvg {input} {tmp}"
          - stage: "transform"
            tool: "vips"
            command: "vips resize {tmp} {tmp2} {scale}"
          - stage: "encode"
            tool: "vips"
            command: "vips copy {tmp2} {output}"

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

-include .assets.mk

all: $(GENERATED_ASSET_FILES)
  @echo "assets up to date"

.assets.mk: assets.yaml
  @assets gen > .assets.mk

$(GENERATED_ASSET_FILES):
  @assets build-target --target $@

check-assets:
  @assets check --strict

clean:
  rm -f $(GENERATED_ASSET_FILES) .assets.mk
```

### 3) Run workflow locally

```bash
assets check --strict
assets gen > .assets.mk
make
assets verify-lock
```

## Commands

- assets check
- assets gen
- assets build-target --target <path>
- assets verify-lock

### assets check

Validates manifest structure and semantics.

- Use strict mode to enforce legal metadata:

```bash
assets check --strict
```

### assets gen

Emits deterministic Makefile fragment to stdout.

```bash
assets gen > .assets.mk
```

### assets build-target

Builds one output and updates assets.lock provenance/size/source hash entry.

```bash
assets build-target --target assets/images/logo_128.png
```

### assets verify-lock

Verifies manifest, sources, lockfile, and output size/provenance alignment.

```bash
assets verify-lock
```

## Failure And Recovery

Common failures:

- source hash mismatch: source file changed after lockfile generation
- provenance mismatch: pipeline command chain/tool versions changed
- missing output: generated file missing from disk
- size mismatch: output differs from lockfile entry

Recovery flow:

1. Run build locally for affected targets (or run make).
2. Re-run assets verify-lock.
3. Commit updated generated outputs plus assets.lock.

## Release Checklist

When changing rendering semantics or pipeline placeholders:

1. Update DESIGN.md with schema/behavior changes.
2. Add or update manifest validation tests.
3. Add or update build-target and verify-lock tests.
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
