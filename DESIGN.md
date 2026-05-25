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
        ▼ [gen]             ▼ [check]           ▼ [build-target]
 ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
 │  .assets.mk  │    │ Compliance   │    │ Invoked by   │
 └──────┬───────┘    │  Validation  │    │ Make on      │
        │            └──────────────┘    │ Out-of-Date  │
        ▼                                └──────┬───────┘
 ┌──────────────┐                               │
 │ Make Engine  ├───────────────────────────────┘
 └──────────────┘

```

---

## 3. Data Schemas

### 3.1 The Asset Manifest (`assets.yaml`)

The source of truth for the asset ecosystem. Outputs support arbitrary rendering options that are passed down to the underlying image processing engine.

```yaml
meta:
  project: "Core App"
  render:
    defaults:
      profile: "balanced"
      strict_renderer_versions: true
    profiles:
      balanced:
        pipeline:
          - stage: "rasterize"
            tool: "resvg"
            command: "resvg --dpi=96 {input} {tmp}"
          - stage: "transform"
            tool: "vips"
            command: "vips resize {tmp} {tmp2} {scale}"
          - stage: "encode"
            tool: "vips"
            command: "vips copy {tmp2} {output}"
        format_options:
          png_compression: 8
          jpeg_quality: 88
      quality:
        pipeline:
          - stage: "rasterize"
            tool: "resvg"
            command: "resvg --dpi=96 {input} {tmp}"
          - stage: "transform"
            tool: "vips"
            command: "vips resize {tmp} {tmp2} {scale}"
          - stage: "encode"
            tool: "vips"
            command: "vips copy {tmp2} {output}"
        format_options:
          png_compression: 9
          jpeg_quality: 92
      fast:
        pipeline:
          - stage: "rasterize"
            tool: "librsvg"
            command: "rsvg-convert -o {tmp} {input}"
          - stage: "transform"
            tool: "vips"
            command: "vips resize {tmp} {tmp2} {scale}"
          - stage: "encode"
            tool: "vips"
            command: "vips copy {tmp2} {output}"
        format_options:
          png_compression: 6
          jpeg_quality: 82

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
          scale_mode: "fit"          # Options: fit, fill, stretch, crop
          background: "transparent"  # Options: transparent, hex code (#FFFFFF)
          profile: "quality"         # Optional: override default render profile
          pipeline_append:
            - stage: "postprocess"
              tool: "oxipng"
              command: "oxipng -o 4 --strip safe {output}"
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

### 3.2 Rendering Tool Resolution

The toolchain is policy-driven and resolved in this order:

1. Output-level override (`options.profile` and output `format_options`).
2. Output-level pipeline controls (`options.pipeline_override`, else `options.pipeline_append`).
3. Manifest defaults (`meta.render.defaults` and selected profile `pipeline`).
4. Built-in application defaults when values are omitted.

The rendering pipeline is generic, ordered, and supports command chaining:

1. Any number of steps may be declared in order.
2. Each step is executed exactly as listed after placeholder expansion.
3. Recommended (but not strictly required) stage kinds are `rasterize`, `transform`, `encode`, and `postprocess`.
4. Typical SVG flow is `rasterize -> transform -> encode`, optionally followed by `postprocess`.
5. Typical bitmap flow may start at `transform` then `encode`.

Command-chain policy:

1. Chains are declarative and ordered.
2. Each step must declare `tool` and `command`; `stage` is recommended for validation/readability.
3. Placeholders (for example `{input}`, `{tmp}`, `{output}`) are expanded by `assets`.
4. Environment is constrained to deterministic defaults (for example locale, timezone, and fixed temp path conventions where practical).

This means SVG->PNG and SVG->JPEG should normally share early pipeline steps (for example rasterize/transform), while encode options differ by output format.

### 3.3 The Lockfile (`assets.lock`)

The lockfile tracks the state of the repository's assets. It guarantees that the downstream binaries exactly match the source files and options defined in the manifest. It is written in deterministic JSON (keys sorted alphabetically).

The `config_hash` is a SHA-256 hash calculated from a deterministic serialization of the resolved output configuration, including dimensions, output options, selected render profile, full resolved command chain, and effective format settings. This ensures that if a developer changes an option (for example `background: "transparent"` to `background: "#FFFFFF"`), changes rendering tool policy, or changes command chaining behavior, the tool detects that the target needs to be rebuilt.

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
          "config_hash": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2",
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
          "config_hash": "8f439281a9c34e2b10f8482d38e9102c348a912e384b102c48e9102c3481a293",
          "size_bytes": 52104
        }
      }
    },
    "search_icon": {
      "source_path": "raw_sources/icons/search.png",
      "source_sha256": "4a5e6f7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f",
      "outputs": {
        "assets/icons/search_24.png": {
          "config_hash": "7d3a1b4e5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f",
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



### 4.3 `assets build-target --target <path>`

Executes the discrete rendering transformation for a *single* target asset path.

* **Actions:**
1. Opens `assets.yaml` and locates the specific output block matching the `--target` path string.
2. Resolves the target's effective rendering configuration from output overrides, manifest defaults, and built-in defaults.
3. Parses the target's parameters (`width`, `height`, `options`) plus selected render profile/tool settings.
4. Executes the resolved pipeline steps in order:
  - Expand placeholders using the target context (`{input}`, `{tmp}`, `{tmp2}`, `{output}`, and derived values).
  - Run each step command with deterministic execution settings.
  - Ensure the pipeline writes the requested output path.
5. Updates the target asset path entry inside `assets.lock` with the fresh `source_sha256`, target `config_hash`, and final file size in bytes.



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
	@assets build-target --target $@

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
assets verify-lock

```


The `verify-lock` command maps live filesystem content hashes of sources against `assets.lock`. If any live source hash or configuration definition diverges from the value preserved inside `assets.lock`, it errors out, letting the developer know they must run `make` locally to update their downstream assets before pushing.