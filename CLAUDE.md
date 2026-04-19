# CLAUDE.md

Project-level guidance for working in this repo. Read on every session.

## What this is

A pure-Go HEIF/HEIC image decoder published as `github.com/gen2brain/heic`. Implements `image/heic.Decode` and `heic.DecodeConfig` for the standard library's `image` package, and also exposes direct functions.

Two independent backends:

- **WASM path** (`decode.go`): a bundled libheif + libde265 compiled to WebAssembly (`lib/heif.wasm.gz`, embedded via `//go:embed`), run through the `wazero` runtime. Works on every platform and every GOARCH. Zero native dependencies.
- **Dynamic path** (`decode_dynamic.go`): loads the system `libheif` at runtime via `purego`, calls the C API directly. Preferred when available — faster, no WASM interpreter overhead. Enabled only on Linux and macOS (Windows is disabled, see upstream issue #11) and only on 64-bit non-MIPS archs (see the build tag).

`heic.go` dispatches each public function to the appropriate backend at runtime based on a `dynamic` bool set at package init. Callers can force WASM with `ForceWasmMode = true`.

## Build & test

```bash
# Full suite. WASM tests always run. Dynamic tests self-skip if libheif is absent.
go test ./...

# Target one backend:
go test -run 'Dynamic' ./...     # requires libheif-dev installed
go test -v -run 'TestDecode$' ./...
```

On Ubuntu, install libheif for the dynamic path:
```bash
sudo apt install -y libheif1 libheif-dev libheif-examples
```

## Rebuilding `heif.wasm.gz`

Required whenever `lib/heif.c` or `lib/Makefile` changes. Needs the WASI SDK at `/opt/wasi-sdk`.

One-time toolchain install:
```bash
curl -LO https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-22/wasi-sdk-22.0-linux.tar.gz
sudo tar -xzf wasi-sdk-22.0-linux.tar.gz -C /opt
sudo mv /opt/wasi-sdk-22.0 /opt/wasi-sdk
```

Rebuild loop:
```bash
cd lib
make clean
make
gzip -9 -f heif.wasm   # produces heif.wasm.gz that //go:embed picks up
cd ..
go test ./...
```

The Makefile clones libheif and libde265 at pinned versions on first build (`LIBHEIF_VERSION`, `LIBDE265_VERSION` at the top). First build is slow (~5 min); subsequent builds are fast. `make clean` removes the cloned sources too — use sparingly.

## File map

- `heic.go` — public API (`Decode`, `DecodeConfig`, `ForceWasmMode`, `Init`, `Dynamic()`), error sentinels, shared constants mirroring `libheif/heif.h` enums, `init()` registers the format with `image.RegisterFormat`.
- `decode.go` — WASM backend. `initialize()` builds the `wazero.Runtime` once, `decode()` allocates WASM memory, runs the exported `decode` function twice (probe, then full), reads planes back into Go `image.Image` values.
- `decode_dynamic.go` — purego backend. `init()` probes the system `libheif`, binds symbols. Behind a build tag: `(linux || darwin || windows) && !(nodynamic || arm || 386 || mips || mipsle)`.
- `purego_{darwin,unix,windows,other}.go` — OS-specific `loadLibrary()` implementations that locate `libheif.dylib` / `libheif.so` / `libheif.dll`. The `_other.go` file is the fallback for disabled archs/OS combinations and stubs out `decodeDynamic`.
- `lib/heif.c` — thin C shim. **Only** exports a `decode` function that wraps the entire decode pipeline. libheif internals are statically linked and dead-code-eliminated to unreferenced symbols.
- `lib/Makefile` — WASM build. The `-Wl,--export=<name>` lines control which symbols survive linking. Adding a new C entry point requires adding a matching `--export` line here.
- `decode_test.go` — tests for both backends. Dynamic tests gate through `requireDynamic(t)` (skips when libheif is absent, fails in CI).
- `testdata/*.heic` — small test fixtures (10-bit, 8-bit, 12-bit, grayscale variants).

## Conventions

- **C and WASM are locked together.** If you change `lib/heif.c` or `lib/Makefile`, rebuild `heif.wasm.gz` in the same commit. Don't leave them out of sync.
- **Any new C export needs two edits**: the function in `heif.c` and the `-Wl,--export=<name>` line in the Makefile. Without the export line, `wasm-ld` will drop it.
- **The two backends must produce the same `image.Image` shape** for equivalent inputs. The existing code confirms this — the YCbCr subsample ratio, the Gray/RGBA/NRGBA branch selection, the stride calculation all mirror each other. When adding a new decode variant, keep this parity.
- **Don't mock libheif in tests.** Use real test fixtures in `testdata/` and the real library. The `requireDynamic` gate handles libheif-absent environments.
- **`image.RegisterFormat` is called exactly once** in `heic.go`'s `init()` for the generic `Decode` / `DecodeConfig` pair. Don't register additional decoders for specialized variants (thumbnails, auxiliary images, etc.) — those are opt-in API calls.
- **Errors at package boundaries** are typed sentinels (`ErrMemRead`, `ErrMemWrite`, `ErrDecode`). Wrap with `fmt.Errorf("%s: %w", ...)` inside functions; return the bare sentinel to callers when appropriate so they can `errors.Is` against it.
- **Windows GUI stdout/stderr handling**: the WASM module writes to `os.Stdout`/`os.Stderr` by default, which crashes Windows GUI apps with no console. `isWindowsGUI()` in `decode.go` detects this via the PE subsystem field and redirects to `io.Discard`. Don't remove this branch.

## Platform gotchas

- **Windows + dynamic = disabled.** Don't spend time debugging `purego` on Windows here. The WASM path is the only Windows path.
- **32-bit builds use WASM only.** The build tag on `decode_dynamic.go` excludes `386`, `arm`, `mips`, `mipsle`. This was fixed in PR #12 — don't reintroduce dynamic loading on these archs without reading that thread.
- **Wazero is single-threaded per `wazero.Runtime`**, but the runtime is shared across calls via the `initOnce` guard. Each `Decode` call instantiates a fresh module (`rt.InstantiateModule`) — this is fine but not free; if you're optimizing, the module pool is the place.

## In-flight work

If `PLAN.md` is present at the repo root, it describes a specific task currently in progress. Read it before starting work if it exists; it overrides defaults for the duration of that task.

## Upstream

This is a fork intended to PR changes back to `github.com/gen2brain/heic`. Keep diffs focused, match the existing code style, and rebuild `heif.wasm.gz` in the same commit as any C/Makefile changes.

