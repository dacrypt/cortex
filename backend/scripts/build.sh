#!/bin/bash
# Build script for air - ensures binary is removed if build fails
set -e

BINARY="./tmp/cortexd"

# Remove old binary before building
rm -f "$BINARY"

# Build - if this fails, the script exits with error code
# and the binary won't exist, so air won't run it
go build -o "$BINARY" ./cmd/cortexd

# Only reach here if build succeeded
exit 0

