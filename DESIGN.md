# System Design Document: Declarative Asset Pipeline & Registry (`asset-mgr`)

This document provides a comprehensive blueprint for building a lightweight, metadata-driven asset pipeline tool named `asset-mgr`. It handles asset validation, legal/compliance tracking, and on-demand image processing by interfacing natively with `Make`.

---

## 1. Executive Summary & Design Philosophy

`asset-mgr` is a command-line tool written in **Go**. It treats assets as code by coupling an asset's legal metadata (ownership, copyright, license) directly with its build instructions.

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
 │                    (asset-mgr)                      │
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
  required_fields: ["license", "owner", "copyright"]

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

### 3.2 The Lockfile (`assets.lock`)

The lockfile tracks the state of the repository's assets. It guarantees that the downstream binaries exactly match the source files and options defined in the manifest. It is written in deterministic JSON (keys sorted alphabetically).

The `config_hash` is a SHA-256 hash calculated from the serialized string of the specific output's dimension and `options` block. This ensures that if a developer changes an option (e.g., from `background: "transparent"` to `background: "#FFFFFF"`) without changing the source image, the tool detects that the target needs to be rebuilt.

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

The Go application executable `asset-mgr` must implement the following structural interface:

### 4.1 `asset-mgr check`

Performs quick, non-destructive semantic evaluation of the project posture.

* **Actions:**
1. Validates that `assets.yaml` matches the required structural schema.
2. Ensures all required fields listed in `meta.required_fields` are populated for every asset block.
3. Confirms that all declared `source` files exist on disk.


* **Exit Codes:** `0` on compliance, `1` on failure (emits human-readable errors to `stderr`).

### 4.2 `asset-mgr gen`

Generates the Makefile dependency fragments based on the asset manifest definitions.

* **Actions:** Reads `assets.yaml` and prints standard Makefile structures to `stdout`.
* **Output Format:**
* Defines a global `GENERATED_ASSET_FILES` variable containing a space-separated list of all output paths.
* Writes explicit target dependency blocks mapping every output path back to its source file.



### 4.3 `asset-mgr build-target --target <path>`

Executes the discrete rendering transformation for a *single* target asset path.

* **Actions:**
1. Opens `assets.yaml` and locates the specific output block matching the `--target` path string.
2. Parses the target's parameters (`width`, `height`, `options`).
3. Executes the image mutation (resizing, scaling, padding, background flattening) using native library bindings or system tools.
4. Updates the target asset path entry inside `assets.lock` with the fresh `source_sha256`, target `config_hash`, and final file size in bytes.



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
# asset-mgr to refresh .assets.mk, then hot-reloads its execution loop.
.assets.mk: assets.yaml
	@asset-mgr gen > .assets.mk

# Rule mapping how any generic asset target in the generated manifest is processed
$(GENERATED_ASSET_FILES):
	@asset-mgr build-target --target $@

check-assets:
	@asset-mgr check
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
asset-mgr verify-lock

```


The `verify-lock` command maps live filesystem content hashes of sources against `assets.lock`. If any live source hash or configuration definition diverges from the value preserved inside `assets.lock`, it errors out, letting the developer know they must run `make` locally to update their downstream assets before pushing.