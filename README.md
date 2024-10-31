## heic
[![Status](https://github.com/gen2brain/heic/actions/workflows/test.yml/badge.svg)](https://github.com/gen2brain/heic/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/gen2brain/heic.svg)](https://pkg.go.dev/github.com/gen2brain/heic)

Go decoder for [HEIC Image File Format](https://en.wikipedia.org/wiki/High_Efficiency_Image_File_Format) (HEVC in HEIF).

Based on [libheif](https://github.com/strukturag/libheif) and [libde265](https://github.com/strukturag/libde265) compiled to [WASM](https://en.wikipedia.org/wiki/WebAssembly) and used with [wazero](https://wazero.io/) runtime (CGo-free).

The library will first try to use a dynamic/shared library (if installed) via [purego](https://github.com/ebitengine/purego) and will fall back to WASM.

### Build tags

* `nodynamic` - do not use dynamic/shared library (use only WASM)
