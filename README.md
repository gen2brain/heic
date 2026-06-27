## heic
[![Status](https://github.com/gen2brain/heic/actions/workflows/test.yml/badge.svg)](https://github.com/gen2brain/heic/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/gen2brain/heic.svg)](https://pkg.go.dev/github.com/gen2brain/heic)

Go decoder for [HEIC Image File Format](https://en.wikipedia.org/wiki/High_Efficiency_Image_File_Format) (HEVC in HEIF).

Based on the Rust [heic](https://crates.io/crates/heic) decoder compiled to [WASM](https://en.wikipedia.org/wiki/WebAssembly) and run with [wazero](https://github.com/tetratelabs/wazero) (CGo-free).

The library will first try to use a [libheif](https://github.com/strukturag/libheif) dynamic/shared library (if installed) via [purego](https://github.com/ebitengine/purego) and will fall back to the embedded WASM.

### Build tags

* `nodynamic` - do not use dynamic/shared library (use only WASM)
* `wasm2go` - transpile the WASM to Go with [wasm2go](https://github.com/ncruces/wasm2go) instead of running it with wazero
