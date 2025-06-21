ifneq (,$(wildcard ./.env))
    include .env
    export
endif

<<<<<<< HEAD
BUILD_CMD = go build -ldflags "-w -linkmode external -extldflags '-lstdc++'" -buildmode pie
=======
# Build commands
BUILD_CMD = go build -ldflags "-w -linkmode external -extldflags '-lstdc++'" -buildmode pie
DEBUG_CMD = go build -ldflags="-linkmode external -extld=/usr/bin/clang -extldflags '-lstdc++ -Wl,-no_compact_unwind' -compressdwarf=false"
>>>>>>> 515bc7c (Initial commit)
BUILD_SRC = ./cmd/beatportdl
BUILD_DIR = ./bin

ZIG_CC = zig cc
ZIG_CXX = zig c++

MACOS_SDK_PATH ?= /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk
<<<<<<< HEAD

all: darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64

darwin-arm64:
	@echo "Building for macOS ARM64"
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_LDFLAGS="-F${MACOS_SDK_PATH}/System/Library/Frameworks -L${MACOS_SDK_PATH}/usr/lib" \
	CC="${ZIG_CC} -target aarch64-macos ${MACOS_ARM64_LIB_PATH} -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	CXX="${ZIG_CXX} -target aarch64-macos ${MACOS_ARM64_LIB_PATH} -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-darwin-arm64 ${BUILD_SRC}

darwin-amd64:
	@echo "Building for macOS AMD64"
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=amd64 \
	CGO_LDFLAGS="-F${MACOS_SDK_PATH}/System/Library/Frameworks -L${MACOS_SDK_PATH}/usr/lib" \
	CC="${ZIG_CC} -target x86_64-macos ${MACOS_AMD64_LIB_PATH} -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	CXX="${ZIG_CXX} -target x86_64-macos ${MACOS_AMD64_LIB_PATH} -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-darwin-amd64 ${BUILD_SRC}

linux-amd64:
	@echo "Building for Linux AMD64"
=======
HOMEBREW_PATH ?= /opt/homebrew

# For Apple Silicon Macs, we'll focus on arm64 build
ifeq ($(shell uname -m),arm64)
all: darwin-arm64
debug: darwin-arm64-debug
else
all: darwin-amd64 linux-amd64 linux-arm64 windows-amd64
debug: darwin-amd64-debug linux-amd64-debug linux-arm64-debug windows-amd64-debug
endif

# Release builds (optimized, no debug info)
darwin-arm64:
	@echo "Building for macOS ARM64 (Release)"
	# go clean -cache
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_LDFLAGS="-F${MACOS_SDK_PATH}/System/Library/Frameworks -L${MACOS_SDK_PATH}/usr/lib -L${HOMEBREW_PATH}/lib" \
	CC="${ZIG_CC} -target aarch64-macos -L${HOMEBREW_PATH}/lib -I${HOMEBREW_PATH}/include -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	CXX="${ZIG_CXX} -target aarch64-macos -L${HOMEBREW_PATH}/lib -I${HOMEBREW_PATH}/include -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-darwin-arm64 ${BUILD_SRC}

darwin-amd64:
	@echo "Building for macOS AMD64 (Release)"
	@echo "Skipping AMD64 build on Apple Silicon Mac. Run on Intel Mac or use a different build approach for cross-compilation."
	@exit 0

linux-amd64:
	@echo "Building for Linux AMD64 (Release)"
>>>>>>> 515bc7c (Initial commit)
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	CC="${ZIG_CC} -target x86_64-linux-gnu ${LINUX_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	CXX="${ZIG_CXX} -target x86_64-linux-gnu ${LINUX_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-linux-amd64 ${BUILD_SRC}

linux-arm64:
<<<<<<< HEAD
	@echo "Building for Linux ARM64"
=======
	@echo "Building for Linux ARM64 (Release)"
>>>>>>> 515bc7c (Initial commit)
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=arm64 \
	CC="${ZIG_CC} -target aarch64-linux-gnu ${LINUX_ARM64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	CXX="${ZIG_CXX} -target aarch64-linux-gnu ${LINUX_ARM64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-linux-arm64 ${BUILD_SRC}

windows-amd64:
<<<<<<< HEAD
	@echo "Building for Windows AMD64"
=======
	@echo "Building for Windows AMD64 (Release)"
>>>>>>> 515bc7c (Initial commit)
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=windows \
	GOARCH=amd64 \
	CC="${ZIG_CC} -target x86_64-windows-gnu ${WINDOWS_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall -Wno-deprecated" \
	CXX="${ZIG_CXX} -target x86_64-windows-gnu ${WINDOWS_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall -Wno-deprecated" \
<<<<<<< HEAD
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-windows-amd64.exe ${BUILD_SRC}
=======
	${BUILD_CMD} -o=${BUILD_DIR}/beatportdl-windows-amd64.exe ${BUILD_SRC}

# Debug builds (with debug symbols and no optimizations)
darwin-arm64-debug:
	@echo "Building for macOS ARM64 (Debug)"
	# go clean -cache
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	CGO_LDFLAGS="-F${MACOS_SDK_PATH}/System/Library/Frameworks -L${MACOS_SDK_PATH}/usr/lib -L${HOMEBREW_PATH}/lib" \
	CC="${ZIG_CC} -target aarch64-macos -L${HOMEBREW_PATH}/lib -I${HOMEBREW_PATH}/include -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	CXX="${ZIG_CXX} -target aarch64-macos -L${HOMEBREW_PATH}/lib -I${HOMEBREW_PATH}/include -isysroot ${MACOS_SDK_PATH} -iwithsysroot /usr/include -iframeworkwithsysroot /System/Library/Frameworks" \
	${DEBUG_CMD} -o=${BUILD_DIR}/beatportdl-darwin-arm64-debug ${BUILD_SRC}

darwin-amd64-debug:
	@echo "Building for macOS AMD64 (Debug)"
	@echo "Skipping AMD64 debug build on Apple Silicon Mac. Run on Intel Mac or use a different build approach for cross-compilation."
	@exit 0

linux-amd64-debug:
	@echo "Building for Linux AMD64 (Debug)"
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	CC="${ZIG_CC} -target x86_64-linux-gnu ${LINUX_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	CXX="${ZIG_CXX} -target x86_64-linux-gnu ${LINUX_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	${DEBUG_CMD} -o=${BUILD_DIR}/beatportdl-linux-amd64-debug ${BUILD_SRC}

linux-arm64-debug:
	@echo "Building for Linux ARM64 (Debug)"
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=arm64 \
	CC="${ZIG_CC} -target aarch64-linux-gnu ${LINUX_ARM64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	CXX="${ZIG_CXX} -target aarch64-linux-gnu ${LINUX_ARM64_LIB_PATH} -DTAGLIB_STATIC -Wall" \
	${DEBUG_CMD} -o=${BUILD_DIR}/beatportdl-linux-arm64-debug ${BUILD_SRC}

windows-amd64-debug:
	@echo "Building for Windows AMD64 (Debug)"
	go clean -cache
	CGO_ENABLED=1 \
	GOOS=windows \
	GOARCH=amd64 \
	CC="${ZIG_CC} -target x86_64-windows-gnu ${WINDOWS_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall -Wno-deprecated" \
	CXX="${ZIG_CXX} -target x86_64-windows-gnu ${WINDOWS_AMD64_LIB_PATH} -DTAGLIB_STATIC -Wall -Wno-deprecated" \
	${DEBUG_CMD} -o=${BUILD_DIR}/beatportdl-windows-amd64-debug.exe ${BUILD_SRC}

# Utility targets
clean:
	@echo "Cleaning build artifacts"
	rm -f ${BUILD_DIR}/beatportdl-*

help:
	@echo "Available targets:"
	@echo "  all                - Build release version for current platform"
	@echo "  debug              - Build debug version for current platform"
	@echo "  darwin-arm64       - Build release for macOS ARM64"
	@echo "  darwin-arm64-debug - Build debug for macOS ARM64"
	@echo "  linux-amd64        - Build release for Linux AMD64"
	@echo "  linux-amd64-debug  - Build debug for Linux AMD64"
	@echo "  linux-arm64        - Build release for Linux ARM64"
	@echo "  linux-arm64-debug  - Build debug for Linux ARM64"
	@echo "  windows-amd64      - Build release for Windows AMD64"
	@echo "  windows-amd64-debug- Build debug for Windows AMD64"
	@echo "  clean              - Remove build artifacts"
	@echo "  help               - Show this help message"

.PHONY: all debug clean help darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64 darwin-arm64-debug darwin-amd64-debug linux-amd64-debug linux-arm64-debug windows-amd64-debug
>>>>>>> 515bc7c (Initial commit)
