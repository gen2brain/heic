LIBHEIF_VERSION = v1.18.2
LIBDE265_VERSION = v1.0.15

LIBHEIF_SRC = $(PWD)/libheif
LIBHEIF_BUILD = $(LIBHEIF_SRC)/build

LIBDE265_SRC = $(PWD)/libde265
LIBDE265_BUILD = $(LIBDE265_SRC)/build

WASI_SDK_PATH = /opt/wasi-sdk
export CC = $(WASI_SDK_PATH)/bin/clang --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot
export CXX = $(WASI_SDK_PATH)/bin/clang++ --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot

CMAKE_TOOLCHAIN_FILE=$(WASI_SDK_PATH)/share/cmake/wasi-sdk.cmake

BIN := heif.wasm

all: $(BIN)

$(LIBHEIF_SRC):
	git clone -b $(LIBHEIF_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/strukturag/libheif
	sed -i '/HeifFile::read_from_file/,+15d' $(LIBHEIF_SRC)/libheif/file.cc
	sed -i '/HeifFile::read_from_file/d' $(LIBHEIF_SRC)/libheif/file.cc
	sed -i '/heif_file_writer_write(/,+13d' $(LIBHEIF_SRC)/libheif/api/libheif/heif.cc
	sed -i '/heif_file_writer_write/d' $(LIBHEIF_SRC)/libheif/api/libheif/heif.cc
	sed -i '/target_colorspace = heif_colorspace_RGB/d' $(LIBHEIF_SRC)/libheif/context.cc
	sed -i '/de265_start_worker_threads/d' $(LIBHEIF_SRC)/libheif/plugins/decoder_libde265.cc
	sed -i '227i return get_decoder_plugin_libde265();' $(LIBHEIF_SRC)/libheif/plugin_registry.cc
	mkdir -p $(LIBHEIF_BUILD)
	test -d $@

$(LIBDE265_SRC):
	git clone -b $(LIBDE265_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/strukturag/libde265
	sed -i '/^find_package(Threads/d' $(LIBDE265_SRC)/CMakeLists.txt
	sed -i '/^target_link_libraries/d' $(LIBDE265_SRC)/libde265/CMakeLists.txt
	sed -i '/static std::mutex/,+4d' $(LIBDE265_SRC)/libde265/de265.cc
	sed -i '/std::mutex/d' $(LIBDE265_SRC)/libde265/de265.cc
	python3 -c "p='$(LIBDE265_SRC)/libde265/CMakeLists.txt'; open(p,'w').write(open(p).read().replace('add_subdirectory (encoder)','if(ENABLE_ENCODER)\nadd_subdirectory (encoder)\nendif()'))"
	python3 -c "p='$(LIBDE265_SRC)/libde265/threads.h'; t=open(p).read(); open(p,'w').write(t.replace('#include <pthread.h>\n\ntypedef pthread_t        de265_thread;\ntypedef pthread_mutex_t  de265_mutex;\ntypedef pthread_cond_t   de265_cond;','#ifdef __wasi__\ntypedef int de265_thread;\ntypedef int de265_mutex;\ntypedef int de265_cond;\n#else\n#include <pthread.h>\ntypedef pthread_t        de265_thread;\ntypedef pthread_mutex_t  de265_mutex;\ntypedef pthread_cond_t   de265_cond;\n#endif'))"
	python3 -c "p='$(LIBDE265_SRC)/libde265/threads.cc'; t=open(p).read(); old='int  de265_thread_create(de265_thread* t, void *(*start_routine) (void *), void *arg) { return pthread_create(t,NULL,start_routine,arg); }'; new='#if defined(__wasi__)\nint  de265_thread_create(de265_thread* t, void *(*start_routine) (void *), void *arg) { return 0; }\nvoid de265_thread_join(de265_thread t) {}\nvoid de265_thread_destroy(de265_thread* t) {}\nvoid de265_mutex_init(de265_mutex* m) {}\nvoid de265_mutex_destroy(de265_mutex* m) {}\nvoid de265_mutex_lock(de265_mutex* m) {}\nvoid de265_mutex_unlock(de265_mutex* m) {}\nvoid de265_cond_init(de265_cond* c) {}\nvoid de265_cond_destroy(de265_cond* c) {}\nvoid de265_cond_broadcast(de265_cond* c,de265_mutex* m) {}\nvoid de265_cond_wait(de265_cond* c,de265_mutex* m) {}\nvoid de265_cond_signal(de265_cond* c) {}\n#else\nint  de265_thread_create(de265_thread* t, void *(*start_routine) (void *), void *arg) { return pthread_create(t,NULL,start_routine,arg); }'; open(p,'w').write(t.replace(old,new))"
	mkdir -p $(LIBDE265_BUILD)
	test -d $@

$(LIBDE265_BUILD)/libde265/libde265.a: $(LIBDE265_SRC)
	cd $(LIBDE265_BUILD); \
	cmake $(LIBDE265_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
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
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-DBUILD_TESTING=0 \
		-DENABLE_PLUGIN_LOADING=0 \
		-DWITH_LIBDE265=1 \
		-DWITH_X265=0 \
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
		-DCMAKE_C_FLAGS="-D__EMSCRIPTEN_STANDALONE_WASM__=1" \
		-DCMAKE_CXX_FLAGS="-D__EMSCRIPTEN_STANDALONE_WASM__=1" \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBHEIF_BUILD); \
	$(MAKE) -j$(shell nproc) VERBOSE=1

$(BIN): $(LIBDE265_BUILD)/libde265/libde265.a $(LIBHEIF_BUILD)/libheif/libheif.a
	$(CC) \
		-O3 \
		-Wl,--no-entry \
		-Wl,--export=malloc \
		-Wl,--export=free \
		-Wl,--export=decode \
		-Wl,--export=decode_thumbnail \
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
