# Complex Example

This example demonstrates a graph-first render configuration with:

- Mixed source formats (`.svg`, `.png`, `.jpg`, `.gif`)
- Multiple candidate tools per conversion edge
- Built-in renderer defaults (no explicit `meta.render.tools` block), with a couple of per-output override examples (string and list forms)
- Scale-mode-aware tool selection
- Optimizer-only outputs (`png -> png`, `gif -> gif`, `jpg -> jpg`, `webp -> webp`)
- Command placeholders (`{input}`, `{tmp}`, `{tmp2}`, `{output}`, `{width}`, `{height}`, `{sets_size}`, `{resize_args}`)

## Layout

- `assets.yaml`: full manifest
- `raw/`: sample source files
- `Makefile`: minimal local workflow for this example

## Try It

From repository root:

```bash
assets check --manifest examples/complex/assets.yaml --strict
assets gen --manifest examples/complex/assets.yaml > examples/complex/.assets.mk
assets build --manifest examples/complex/assets.yaml --target examples/complex/out/images/logo_256.png
assets verify --manifest examples/complex/assets.yaml
```

Or from inside `examples/complex/`:

```bash
make
make check
```

`make` now performs a full build for all declared outputs by regenerating `.assets.mk` and running `assets build` for each target in `GENERATED_ASSET_FILES`.

If `assets` is not on your `PATH`, override the command used by the Makefile:

```bash
make ASSETS='go run ../../cmd/assets' check
make ASSETS='go run ../../cmd/assets'
```

Note:
- This example uses standard renderer tool definitions (for example `resvg`, `vips`, `magick`, `oxipng`, `gifsicle`, `jpegoptim`, `cwebp`), so running full builds requires those tools installed.
- The resolver can still plan without availability checks in tests via `ResolvePipelineWithOptions(..., ResolveOptions{CheckAvailability: false})`.
