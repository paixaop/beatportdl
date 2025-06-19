#!/bin/bash
MACOS_ARM64_LIB_PATH="-L/opt/homebrew/lib -I/opt/homebrew/include" make darwin-arm64-debug
#./bin/beatportdl-darwin-arm64-debug -playlist https://www.beatport.com/library
./bin/beatportdl-darwin-arm64-debug 