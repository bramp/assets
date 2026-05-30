# System Design Document: Declarative Asset Pipeline & Registry (`assets`)

This document provides a comprehensive blueprint for building a lightweight, metadata-driven asset pipeline tool named `assets`. It handles asset validation, legal/compliance tracking, and on-demand image processing by interfacing natively with `Make`.

---

## 1. Executive Summary & Design Philosophy

`assets` is a command-line tool written in **Go**. It treats assets as code by coupling an asset's legal metadata (ownership, copyright, license) directly with its build instructions.

### Core Tenets:

* **Separation of Concerns:** The Go tool manages parsing, semantic validation, and individual image transformations. `Make` manages the execution graph, parallelization (`-j`), and file-system timestamp tracking.
* **Hermetic & Flexible Configuration:** Transformation parameters (scaling, colors, formats) are read dynamically from the manifest file per target output, rather than hardcoded into CLI flags.
* **Deterministic Verification:** A lockfile records the state of source files and configurations. CI/CD pipelines can instantly verify asset freshness using text-based checks, eliminating expensive or non-deterministic graphic compilation in headless environments.

---

## 2. Architecture & Data Flow

```
                     ┌─────────────┐
                     │ assets.yaml │
                     └──────┬──────┘
                            │
                            ▼
 ┌──────────────────────────┴──────────────────────────┐
 │                      Go Tool                        │
 │                    (assets)                      │
 └──────┬───────────────────┬───────────────────┬──────┘
        │                   │                   │
        ▼ [gen]             ▼ [check]           ▼ [build]
 ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
 │  .assets.mk  │    │ Compliance   │    │ Invoked by   │
 └──────┬───────┘    │  Validation  │    │ Make on      │
        │            └──────────────┘    │ Out-of-Date  │
        ▼                                └──────┬───────┘
 ┌──────────────┐                               ▲
 │ Make Engine  ├───────────────────────────────┘
 └──────────────┘

```

---

## 3. Data Schemas

### 3.1 The Asset Manifest (`assets.yaml`)

The source of truth for the asset ecosystem. The render configuration defines a graph of tool operations and minimal policy for picking the best path.

```yaml
meta:
  project: "Core App"
  render:
    defaults:
      # Ordered preferences used to break shortest-path ties.
      tools: ["resvg", "rsvg-convert", "inkscape", "vips-transform", "magick-transform", "vips-encode", "magick-encode", "oxipng", "gifsicle", "jpegoptim", "cwebp"]
      strict_renderer_versions: true
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
        command: "oxipng -o 4 --strip safe {output}"
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

assets:
  - id: "app_logo"
    source: "raw_sources/logo.svg"
    owner: "Andrew Brampton"
    copyright: "© 2026 Andrew Brampton"
    license: "Proprietary"
    outputs:
      - path: "assets/images/logo_512.png"
        width: 512
        height: 512
        options:
          # Per-output override may be scalar or list.
          tools: ["inkscape", "resvg", "rsvg-convert", "magick-transform", "magick-encode"]
          scale_mode: "fit"
          background: "transparent"
          format_options:
            png_compression: 9
      - path: "assets/images/logo_128_ie.png"
        width: 128
        height: 128
        options:
          scale_mode: "fill"
          background: "#FFFFFF"

  - id: "search_icon"
    source: "raw_sources/icons/search.png"
    owner: "Material Design Authors"
    copyright: "© Google LLC"
    license: "Apache-2.0"
    outputs:
      - path: "assets/icons/search_24.png"
        width: 24
        height: 24
        options:
          scale_mode: "fit"
          background: "transparent"

```

### 3.2 Graph-Based Pipeline Planning

Pipeline construction is modeled as a shortest-path planning problem over a graph of operation candidates.

State model:

1. Source extension and output extension.
2. Target dimensions and per-output options.

Operation model:

1. Tool catalog entries (`meta.render.tools`) define graph edges.
2. Each candidate declares `tool`, `command`, `accepts`, `produces`, and optional `scale_modes` constraints.
3. Rasterize candidates may declare `sets_size` as a command-fragment template (for example `"--width {width} --height {height}"`) to indicate they can directly satisfy target dimensions.
4. Commands may combine concerns (for example rasterize plus resize, or rasterize directly to final output format).

Planner policy (minimal and deterministic):

1. Start from a stage graph implied by source and target format classes.
2. Skip unnecessary stages when a prior stage already satisfies the goal (for example rasterizer has `sets_size` and supports requested `scale_mode`).
3. Resolve the shortest compatible path from source format to target format.
4. If a stage preference is set to `none/off`, that stage is disabled.
5. If no valid path exists (for example no conversion-capable stages match source and target extensions), fail with a clear error.

Resize intent semantics (`options.scale_mode`):

1. `fit`: preserve aspect ratio and fit fully inside the target box.
2. `fill`: preserve aspect ratio and cover the target box (cropping allowed).
3. `stretch`: ignore aspect ratio and force exact dimensions.
4. `crop`: preserve aspect ratio and crop to exact dimensions.

Preference inputs:

1. Defaults: `meta.render.defaults.tools` (string or list).
2. Per-output overrides: `outputs[].options.tools` (string or list).
3. Override and defaults use identical tie-break semantics for equal-length paths.

Fallback behavior:

1. If a preference item is `auto`, expand to defaults for that stage.
2. Manual chains via `options.pipeline_override` remain supported and bypass planning.

### 3.3 The Lockfile (`assets.lock`)

The lockfile tracks the state of the repository's assets. It guarantees that the downstream binaries exactly match the source files and options defined in the manifest. It is written in deterministic JSON (keys sorted alphabetically).

Freshness and reproducibility rely on source file hashes, output file size/existence checks, and recorded command provenance (command chain, tool versions, and host/runtime fingerprint).

The lockfile also records command provenance for each output, including:

1. Executed command chain (normalized command strings after placeholder expansion strategy is fixed).
2. Tool identities and versions.
3. Runtime fingerprint details gathered before running commands (for example host OS and key linked library versions).

```json
{
  "version": "1.0",
  "generated_at": "2026-05-25T10:55:32Z",
  "assets": {
    "app_logo": {
      "source_path": "raw_sources/logo.svg",
      "source_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "outputs": {
        "assets/images/logo_128_ie.png": {
          "provenance": {
            "command_chain": [
              "resvg --dpi=96 {input} {tmp}",
              "vips resize {tmp} {output} {scale}",
              "oxipng -o 4 --strip safe {output}"
            ],
            "tools": {
              "host_uname": "Linux buildbox 6.8.0-31-generic #31-Ubuntu SMP x86_64 GNU/Linux",
              "resvg": "0.43.0",
              "vips": "8.15.2",
              "oxipng": "9.1.2",
              "librsvg": "2.58.1",
              "libvips": "8.15.2"
            }
          },
          "size_bytes": 14220
        },
        "assets/images/logo_512.png": {
          "size_bytes": 52104
        }
      }
    },
    "search_icon": {
      "source_path": "raw_sources/icons/search.png",
      "source_sha256": "4a5e6f7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f",
      "outputs": {
        "assets/icons/search_24.png": {
          "size_bytes": 3108
        }
      }
    }
  }
}

```

---

## 4. Command-Line Interface (CLI) Specification

The Go application executable `assets` must implement the following structural interface:

### 4.1 `assets check`

Performs quick, non-destructive semantic evaluation of the project posture.

* **Actions:**
1. Validates that `assets.yaml` matches the required structural schema.
2. In strict mode, ensures required legal metadata fields (`owner`, `copyright`, `license`) are populated for every asset block.
3. In loose mode, allows missing legal metadata fields while still validating structure and source presence.
4. Confirms that all declared `source` files exist on disk.


* **Exit Codes:** `0` on compliance, `1` on failure (emits human-readable errors to `stderr`).

### 4.2 `assets gen`

Generates the Makefile dependency fragments based on the asset manifest definitions.

* **Actions:** Reads `assets.yaml` and prints standard Makefile structures to `stdout`.
* **Output Format:**
* Defines a global `GENERATED_ASSET_FILES` variable containing a space-separated list of all output paths.
* Writes explicit target dependency blocks mapping every output path back to its source file.


### 4.3 `assets defaults`

Prints a recommended `meta.render` configuration snippet for use in `assets.yaml`.

* **Actions:** Emits a copy/paste friendly YAML block to `stdout` that defines the tool graph catalog (`meta.render.tools`), per-tool format capabilities, and ordered defaults (`meta.render.defaults.tools`).
* **Flags:** No stage/tool tuning flags; configuration is authored in `assets.yaml`.


### 4.4 `assets build --target <path>`

Executes the discrete rendering transformation for a *single* target asset path.

* **Actions:**
1. Opens `assets.yaml` and locates the specific output block matching the `--target` path string.
2. Resolves the target's effective rendering policy from output overrides, manifest defaults, and stage catalogs.
3. Plans the shortest valid conversion path based on format constraints, vector/raster classification, and stage preferences.
4. Executes planned steps in order:
  - Expand placeholders using the target context (`{input}`, `{tmp}`, `{tmp2}`, `{output}`, and derived values).
  - Run each step command with deterministic execution settings.
  - Ensure the pipeline writes the requested output path.
5. Updates the target asset path entry inside `assets.lock` with the fresh `source_sha256`, recorded provenance (including chosen command chain), and final file size in bytes.



---

## 5. Build System Orchestration (The Orchestration Wrapper)

The developer coordinates execution exclusively through a root-level static `Makefile`.

```makefile
# Makefile
.PHONY: all check-assets clean

# Dynamic bootstrap hook
-include .assets.mk

# Core orchestration target
all: $(GENERATED_ASSET_FILES)
	@echo "✓ System synchronization successful."

# Dynamic rule engine updates. If assets.yaml is altered, Make calls 
# assets to refresh .assets.mk, then hot-reloads its execution loop.
.assets.mk: assets.yaml
	@assets gen > .assets.mk

# Rule mapping how any generic asset target in the generated manifest is processed
$(GENERATED_ASSET_FILES):
  @assets build --target $@

check-assets:
	@assets check
	@echo "✓ All legal metadata and manifest constraints conform to standard compliance thresholds."

clean:
	rm -f $(GENERATED_ASSET_FILES) .assets.mk

```

---

## 6. Verification & Presubmit Logic

To assert asset verification states inside CI/CD tools (e.g., GitHub Actions) without forcing image-generation libraries into the runner, the pipeline relies on the following decoupled steps:

1. **Metadata Enforcement:** Run `make check-assets` to guarantee legal and file-presence verification rules hold true.
2. **Freshness State Check:** Run a custom Go validation step (or light shell wrapper) that mimics Make's `-q` capability against the lockfile:
```bash
# CI verification command executed by the runner
assets verify

```


The `verify` command maps live filesystem content hashes of sources against `assets.lock`. If any live source hash or configuration definition diverges from the value preserved inside `assets.lock`, it errors out, letting the developer know they must run `make` locally to update their downstream assets before pushing.