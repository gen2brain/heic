SHIM = target/wasm32-unknown-unknown/release/heic_shim.wasm

all: wazero wasm2go

$(SHIM): lib.rs Cargo.toml
	cargo build --target wasm32-unknown-unknown --release

wazero: heic.wasm.gz
heic.wasm.gz: $(SHIM)
	gzip -9 -c $(SHIM) > $@

wasm2go: ../libheic.go
../libheic.go: $(SHIM)
	wasm2go -pkg heic -unsafe -tags wasm2go -o $@ $(SHIM)
	sed -i -E '/^const data[0-9]/! { s/\bModule\b/module/g; s/\bMemory\b/memory/g; s/\bNew\b/newModuleRaw/g; }' $@

.PHONY: all wazero wasm2go clean

clean:
	cargo clean
	rm -f ../libheic.go heic.wasm.gz
