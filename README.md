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

Start with the smallest manifest.
For render pipeline defaults, run `assets defaults` and copy the snippet under `meta.render`.
You can customize the pipeline later (see "Customize Pipeline").

```bash
# Print a recommended stage-tool catalog and defaults.
assets defaults
```

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

When defaults are not enough, define stage catalogs and pick tools independently.

Graph tool selection is composable:

- Global defaults: set ordered tool preference list in `meta.render.defaults.tools`.
- Per output: override with `outputs[].options.tools` as either a string or list.
- Tool compatibility metadata: set `accepts` and `produces` per tool entry so the resolver can find the shortest valid conversion path.
- Size capability metadata: set `sets_size` to a command fragment template (for example `"--width {width} --height {height}"`) so tools can optionally honor target width/height.
- Mode capability metadata: set `scale_modes` per tool entry so the resolver can require aspect-ratio behavior compatibility.
- Selection behavior: shortest valid path wins; preference order breaks ties.

Resize intent (`options.scale_mode`) semantics:

- `fit`: preserve aspect ratio; fully visible within target box.
- `fill`: preserve aspect ratio; cover target box (cropping may occur).
- `stretch`: ignore aspect ratio; force exact target width/height.
- `crop`: preserve aspect ratio and crop to target box.

```yaml
meta:
  project: "My App"
  render:
    defaults:
      tools: ["resvg", "rsvg-convert", "inkscape", "vips-transform", "magick-transform", "vips-encode", "magick-encode", "oxipng", "gifsicle", "jpegoptim", "cwebp"]
    tools:
      resvg:
        tool: "resvg"
        accepts: [".svg"]
        produces: [".png", ".webp", ".jpg", ".jpeg"]
        scale_modes: ["fit"]
        sets_size: "--width {width} --height {height}"
        command: "resvg {sets_size} {input} {output}"
      rsvg-convert:
        tool: "rsvg-convert"
        accepts: [".svg"]
        produces: [".png"]
        scale_modes: ["fit"]
        command: "rsvg-convert -w {width} -h {height} -o {output} {input}"
      inkscape:
        tool: "inkscape"
        accepts: [".svg", ".eps"]
        produces: [".png"]
        scale_modes: ["fit"]
        command: "inkscape {input} --export-filename={output} --export-width={width} --export-height={height}"
      vips-transform:
        tool: "vips"
        accepts: [".png", ".jpg", ".jpeg", ".webp", ".gif"]
        produces: [".png", ".jpg", ".jpeg", ".webp", ".gif"]
        scale_modes: ["fit", "fill", "stretch", "crop"]
        command: "vips resize {input} {output} {scale}"
      magick-transform:
        tool: "magick"
        accepts: ["*"]
        produces: ["*"]
        scale_modes: ["fit", "fill", "stretch", "crop"]
        command: "magick {input} {resize_args} {output}"
      vips-encode:
        tool: "vips"
        accepts: [".png", ".jpg", ".jpeg", ".webp"]
        produces: [".png", ".jpg", ".jpeg", ".webp"]
        command: "vips copy {input} {output}"
      magick-encode:
        tool: "magick"
        accepts: ["*"]
        produces: ["*"]
        command: "magick {input} {output}"
      oxipng:
        tool: "oxipng"
        accepts: [".png"]
        produces: [".png"]
        command: "oxipng -o 3 --strip safe {output}"
      gifsicle:
        tool: "gifsicle"
        accepts: [".gif"]
        produces: [".gif"]
        command: "gifsicle -O3 -b {output}"
      jpegoptim:
        tool: "jpegoptim"
        accepts: [".jpg", ".jpeg"]
        produces: [".jpg", ".jpeg"]
        command: "jpegoptim --strip-all {output}"
      cwebp:
        tool: "cwebp"
        accepts: [".webp"]
        produces: [".webp"]
        command: "cwebp -quiet -q 82 {output} -o {output}"

assets:
  - id: "logo"
    source: "raw/logo.svg"
    outputs:
      - path: "assets/images/logo_128.png"
        width: 128
        height: 128
        options:
          tools: ["inkscape", "resvg", "rsvg-convert", "magick-transform", "magick-encode"]
          scale_mode: "fit"
          background: "transparent"
```

Example behavior:

- `assets/images/logo.png` resolves the shortest valid graph path from source format to `.png`, then uses `defaults.tools` ordering to break ties.
- `assets/images/anim.gif` resolves similarly for `.gif` and can choose a different chain based on tool capabilities.
- Per-output override still wins (for example `options.tools: ["resvg", "magick-encode"]`).

## Complex Example

For a full graph-first setup with mixed source formats, fallback tools, scale mode constraints, and optimizer-only outputs, see:

- `examples/complex/assets.yaml`
- `examples/complex/README.md`

## Commands

- assets check
- assets gen
- assets defaults
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
# Output includes commented planned command chains per target.
assets gen > .assets.mk
```

### assets defaults

Prints a recommended `meta.render` snippet you can paste into `assets.yaml`.

```bash
assets defaults
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
