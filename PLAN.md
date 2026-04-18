# `DecodeThumbnail` implementation plan — gen2brain/heic fork

## Context

Add `DecodeThumbnail` and `DecodeThumbnailConfig` to this fork of `github.com/gen2brain/heic`. The use case is bulk iPhone HEIC processing in a Wails app: iPhone HEIF files embed a 416×312 HEVC thumbnail that decodes ~100× faster than the 5712×4284 primary. This work is intended for upstream PR, so both the bundled-WASM path and the dynamic-libheif path must be implemented and tested.

Repo: `https://github.com/Rosca75/heic` (fork of `gen2brain/heic`).

## Decisions already made (do not re-litigate)

- **Public API:** two functions, `DecodeThumbnail(r io.Reader) (image.Image, error)` and `DecodeThumbnailConfig(r io.Reader) (image.Config, error)`. Same dispatch pattern as `Decode`/`DecodeConfig` in `heic.go`.
- **Missing thumbnail:** return a new sentinel `ErrNoThumbnail = errors.New("heic: no thumbnail")`. Do not fall back to primary decode.
- **Multiple thumbnails:** take the first-found (`heif_image_handle_get_list_of_thumbnail_IDs` with `count=1`).
- **Upstream PR-ready:** implement both WASM and dynamic paths, match upstream code style, update tests in `decode_test.go`.
- **Do not** register `DecodeThumbnail` with `image.RegisterFormat` — it's opt-in, not a generic HEIF decoder.

## Implementation, in order

### Step 1 — C wrapper in `lib/heif.c`

Add a `decode_thumbnail` function next to `decode`. It should:

1. Same filetype check + context alloc + `heif_context_read_from_memory_without_copy` + `heif_context_get_primary_image_handle` as `decode`.
2. Call `heif_image_handle_get_number_of_thumbnails(primary)`. If 0, free resources and **return 2** (sentinel for "no thumbnail").
3. Call `heif_image_handle_get_list_of_thumbnail_IDs(primary, &id, 1)` to get the first thumbnail ID.
4. Call `heif_image_handle_get_thumbnail(primary, id, &thumb_handle)`. On error, return 0.
5. From there, mirror the existing `decode` body but operating on `thumb_handle` instead of the primary handle. `config_only` branch returns early after filling dimensions/colorspace.
6. Return values: **0 = failure, 1 = success, 2 = no thumbnail.** Document this in a comment.

Include header: `#include <libheif/heif.h>` is already there.

Declaration to add near the top (match style of the existing `decode` declaration):

```c
int decode_thumbnail(uint8_t *heic_in, int heic_in_size, int config_only, uint32_t *width, uint32_t *height,
    uint32_t *colorspace, uint32_t *chroma, uint32_t *is_premultiplied, uint8_t *out);
```

Factor shared logic out of `decode` and `decode_thumbnail` into a static helper if it reads cleaner — but don't over-engineer; duplication is acceptable here given the file's scope.

### Step 2 — Export it from the WASM build

Edit `lib/Makefile`, add one line to the linker flags (around line 96):

```makefile
-Wl,--export=decode_thumbnail \
```

### Step 3 — Rebuild `heif.wasm.gz`

```bash
cd lib
make clean
make               # produces heif.wasm
gzip -9 -f heif.wasm  # produces heif.wasm.gz
ls -la heif.wasm.gz
cd ..
```

The `//go:embed lib/heif.wasm.gz` in `decode.go:21` will pick up the new file automatically on next `go build`.

### Step 4 — WASM-path Go wrapper in `decode.go`

Add `decodeThumbnail(r io.Reader, configOnly bool) (image.Image, image.Config, error)`. Structure:

- Copy the body of `decode` (decode.go:24–231) as the starting point.
- Change the `mod.ExportedFunction("decode")` lookup to `"decode_thumbnail"`.
- After the first `_decode.Call` (the config/probe call), check the return value:
  - `res[0] == 0` → `ErrDecode` (same as today).
  - `res[0] == 2` → `ErrNoThumbnail` (new sentinel, defined in `heic.go`).
  - `res[0] == 1` → continue as normal.
- Apply the same check after the second `_decode.Call` (the full decode).
- Everything else — memory layout, plane copying, `image.YCbCr`/`Gray`/`RGBA`/`NRGBA` assembly — is identical.

Don't duplicate the 200-line function body wholesale if you can avoid it: a shared helper that takes the export name as an argument, or a `thumbnail bool` parameter, will keep the diff small.

### Step 5 — Dynamic-path Go wrapper in `decode_dynamic.go`

Add three purego bindings in `init()` (decode_dynamic.go:223–235), following the existing `purego.RegisterLibFunc` pattern:

```go
purego.RegisterLibFunc(&_heifImageHandleGetNumberOfThumbnails, libheif, "heif_image_handle_get_number_of_thumbnails")
purego.RegisterLibFunc(&_heifImageHandleGetListOfThumbnailIDs, libheif, "heif_image_handle_get_list_of_thumbnail_IDs")
purego.RegisterLibFunc(&_heifImageHandleGetThumbnail, libheif, "heif_image_handle_get_thumbnail")
```

Add the function-pointer vars (decode_dynamic.go:248–265 area) with signatures derived from `libheif/heif.h`:

```go
_heifImageHandleGetNumberOfThumbnails func(*heifImageHandle) int
_heifImageHandleGetListOfThumbnailIDs func(*heifImageHandle, *uint32, int) int
_heifImageHandleGetThumbnail          func(*heifImageHandle, uint32, **heifImageHandle) uintptr
```

(`heif_item_id` is a `uint32_t` typedef in libheif.)

Add the Go-side wrappers (mirror the pattern at decode_dynamic.go:267–339):

```go
func heifImageHandleGetNumberOfThumbnails(h *heifImageHandle) int { ... }
func heifImageHandleGetListOfThumbnailIDs(h *heifImageHandle, ids []uint32) int { ... }
func heifImageHandleGetThumbnail(h *heifImageHandle, id uint32, out **heifImageHandle) heifError { ... }
```

Add `decodeThumbnailDynamic(r io.Reader, configOnly bool) (image.Image, image.Config, error)`. Mirror `decodeDynamic` (decode_dynamic.go:16–187), but after obtaining the primary `handle`:

1. `n := heifImageHandleGetNumberOfThumbnails(handle)`. If `n == 0`, release the handle and return `ErrNoThumbnail`.
2. `ids := make([]uint32, 1); heifImageHandleGetListOfThumbnailIDs(handle, ids)`.
3. `var thumb *heifImageHandle; e := heifImageHandleGetThumbnail(handle, ids[0], &thumb)`. Check `e.Code`.
4. `defer heifImageHandleRelease(thumb)`, then continue with `thumb` as the working handle for all subsequent calls (`heifImageHandleGetWidth(thumb)`, `heifImageHandleGetPreferredDecodingColorspace(thumb, ...)`, `heifDecodeImage(thumb, ...)`, etc.).

The primary handle should also still be released (the existing `defer heifImageHandleRelease(handle)` stays).

### Step 6 — Public API in `heic.go`

Add:

```go
// ErrNoThumbnail is returned by DecodeThumbnail / DecodeThumbnailConfig when
// the HEIF file contains no embedded thumbnail.
var ErrNoThumbnail = errors.New("heic: no thumbnail")
```

Add two public functions modeled on `Decode` (heic.go:17–35) and `DecodeConfig` (heic.go:37–55). Same `if dynamic && !ForceWasmMode` dispatch to `decodeThumbnailDynamic` / `decodeThumbnail`.

Do **not** touch the `init()` at heic.go:133 — do not add `image.RegisterFormat` for thumbnails.

### Step 7 — Tests in `decode_test.go`

First, determine whether any existing `testdata/*.heic` file has an embedded thumbnail:

```bash
for f in testdata/*.heic; do
  echo "=== $f ==="
  heif-info "$f" | grep -i thumb
done
```

If one of them does, reuse it. If not (likely — these look like minimal test fixtures), generate a small test file with a thumbnail:

```bash
# Take any existing testdata image, round-trip through heif-enc with a thumbnail.
# Or use a small public-domain JPEG:
heif-enc --thumb 160 -o testdata/thumb.heic <some-input>
```

Keep the file small (ideally <50 KB). Add an `//go:embed testdata/thumb.heic` line matching the existing pattern at decode_test.go:18–28.

Add four tests mirroring the existing style:

- `TestDecodeThumbnail` — calls `decodeThumbnail(bytes.NewReader(testThumb), false)`, JPEG-encodes the result like `TestDecode`.
- `TestDecodeThumbnailConfig` — calls `decodeThumbnail(..., true)`, asserts non-zero `Width` and `Height`.
- `TestDecodeThumbnailDynamic` — same but through `decodeThumbnailDynamic`, gated by `requireDynamic(t)` (decode_test.go:102).
- `TestDecodeThumbnailMissing` — feed `testHeic` (which has no thumbnail) to `decodeThumbnail` and assert `errors.Is(err, ErrNoThumbnail)`. Do the same for the dynamic path.

## Verification

```bash
# Full unit tests (WASM path)
go test ./...

# Dynamic path (libheif installed via apt above)
go test -run Dynamic ./...

# Manual smoke test against an iPhone HEIC if available
# (Write a 10-line main.go that opens the file, calls DecodeThumbnail,
#  and writes the result to thumb.jpg. Verify visually.)
```

Both paths must produce an identical-shape `image.Image` (YCbCr 4:2:0 for iPhone thumbnails, in practice) with dimensions matching what `heif-info` reports for the file.

## PR preparation

1. Squash/tidy commits. Upstream commit messages are terse — match the style of recent commits (`git log --oneline -20`).
2. Update `README.md` if it enumerates supported functions.
3. Open PR against `gen2brain/heic:main`. Title suggestion: `Add DecodeThumbnail and DecodeThumbnailConfig`. Body: summarize API additions, note WASM module was rebuilt (wasi-sdk version), confirm both paths are tested.
4. Expect upstream review to push back on any of: duplicated code between `decode`/`decodeThumbnail`, the sentinel-return-code convention in C, purego signature nits. Be prepared to iterate.

## Risks and gotchas

- **WASI SDK version drift:** the rebuilt `heif.wasm.gz` binary will differ from the committed one even if the C source is identical, because different toolchain versions produce different codegen. If the PR diff shows a huge unexplained binary change, note the wasi-sdk version used in the PR description.
- **libheif version on the Ubuntu box:** the dynamic path has a version gate at decode_dynamic.go:62 (`versionMajor == 1 && versionMinor >= 17`). `heif_image_handle_get_list_of_thumbnail_IDs` has been in libheif since forever; no similar gate needed. But verify with `pkg-config --modversion libheif`.
- **`heif_error` struct return via `uintptr`:** the existing code at decode_dynamic.go:290 does `*(*heifError)(unsafe.Pointer(&ret))` on a `uintptr`. This is how upstream does it — follow the pattern; don't try to "fix" it.
- **Windows dynamic path is disabled** (decode_dynamic.go:190). Don't waste time testing the dynamic variant on Windows. The WASM path covers Windows.

