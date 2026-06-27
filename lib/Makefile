SHIM = target/wasm32-unknown-unknown/release/heic_shim.wasm

wasm2go: ../libheic.go

$(SHIM): lib.rs Cargo.toml
	cargo build --target wasm32-unknown-unknown --release

../libheic.go: $(SHIM)
	wasm2go -pkg heic -unsafe -o $@ $(SHIM)
	sed -i -E '/^const data[0-9]/! { s/\bModule\b/module/g; s/\bMemory\b/memory/g; s/\bNew\b/newModuleRaw/g; }' $@

.PHONY: wasm2go clean

clean:
	cargo clean
	rm -f ../libheic.go
