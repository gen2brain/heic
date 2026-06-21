LIBHEIF_VERSION = v1.23.0
LIBDE265_VERSION = v1.1.1

LIBHEIF_SRC = $(PWD)/libheif
LIBHEIF_BUILD = $(LIBHEIF_SRC)/build

LIBDE265_SRC = $(PWD)/libde265
LIBDE265_BUILD = $(LIBDE265_SRC)/build

WASI_SDK_PATH = /opt/wasi-sdk
export CC = $(WASI_SDK_PATH)/bin/clang --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot
export CXX = $(WASI_SDK_PATH)/bin/clang++ --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot

CMAKE_TOOLCHAIN_FILE=$(WASI_SDK_PATH)/share/cmake/wasi-sdk-p1.cmake

BIN := heif.wasm

all: $(BIN)

$(LIBHEIF_SRC):
	git clone -b $(LIBHEIF_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/strukturag/libheif
	mkdir -p $(LIBHEIF_BUILD)
	test -d $@

$(LIBDE265_SRC):
	git clone -b $(LIBDE265_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/strukturag/libde265
	mkdir -p $(LIBDE265_BUILD)
	test -d $@

$(LIBDE265_BUILD)/libde265/libde265.a: $(LIBDE265_SRC)
	cd $(LIBDE265_BUILD); \
	cmake $(LIBDE265_SRC) \
		-DCMAKE_BUILD_TYPE=MinSizeRel \
		-DBUILD_SHARED_LIBS=0 \
		-DENABLE_SDL=0 \
		-DENABLE_DECODER=0 \
		-DENABLE_ENCODER=0 \
		-DCMAKE_CXX_FLAGS="-D_WASI_EMULATED_SIGNAL -Wno-unused-variable" \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBDE265_BUILD); \
	$(MAKE) -j$(shell nproc) VERBOSE=1

	cp $(LIBDE265_BUILD)/libde265/de265-version.h $(LIBDE265_SRC)/libde265/

$(LIBHEIF_BUILD)/libheif/libheif.a: $(LIBHEIF_SRC)
	cd $(LIBHEIF_BUILD); \
	cmake $(LIBHEIF_SRC) \
		-DCMAKE_BUILD_TYPE=MinSizeRel \
		-DBUILD_SHARED_LIBS=0 \
		-DBUILD_TESTING=0 \
		-DENABLE_PLUGIN_LOADING=0 \
		-DWITH_LIBDE265=1 \
		-DWITH_X265=0 \
		-DWITH_X264=0 \
		-DWITH_KVAZAAR=0 \
		-DWITH_DAV1D=0 \
		-DWITH_AOM_DECODER=0 \
		-DWITH_AOM_ENCODER=0 \
		-DWITH_SvtEnc=0 \
		-DWITH_RAV1E=0 \
		-DWITH_JPEG_DECODER=0 \
		-DWITH_JPEG_ENCODER=0 \
		-DWITH_OpenJPEG_DECODER=0 \
		-DWITH_OpenJPEG_ENCODER=0 \
		-DWITH_FFMPEG_DECODER=0 \
		-DWITH_LIBSHARPYUV=0 \
		-DWITH_EXAMPLES=0 \
		-DWITH_GDK_PIXBUF=0 \
		-DENABLE_MULTITHREADING_SUPPORT=0 \
		-DLIBDE265_INCLUDE_DIR=$(LIBDE265_SRC) \
		-DLIBDE265_LIBRARY=-L$(LIBDE265_BUILD)/libde265 \
		-DCMAKE_C_FLAGS="-D__EMSCRIPTEN_STANDALONE_WASM__=1 -include $(PWD)/wasi-compat.h" \
		-DCMAKE_CXX_FLAGS="-D__EMSCRIPTEN_STANDALONE_WASM__=1 -include $(PWD)/wasi-compat.h" \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBHEIF_BUILD); \
	$(MAKE) -j$(shell nproc) heif VERBOSE=1

$(BIN): $(LIBDE265_BUILD)/libde265/libde265.a $(LIBHEIF_BUILD)/libheif/libheif.a
	$(CC) \
		-Oz \
		-Wl,--no-entry \
		-Wl,--export=malloc \
		-Wl,--export=free \
		-Wl,--export=decode \
		-Wl,--strip-debug \
		-Wl,--wrap=de265_start_worker_threads \
		-mexec-model=reactor \
		-mnontrapping-fptoint \
		-I${LIBHEIF_SRC}/libheif/api \
		-I${LIBHEIF_BUILD} \
		-o $@ \
		-Wall \
		heif.c \
		${LIBHEIF_BUILD}/libheif/libheif.a \
		${LIBDE265_BUILD}/libde265/libde265.a \
		-lstdc++

.PHONY: clean

clean:
	-rm -rf $(LIBHEIF_SRC) $(LIBDE265_SRC) $(BIN)
