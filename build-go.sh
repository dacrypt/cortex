#!/bin/bash

# build-go.sh - Compila binarios Go para todas las plataformas

set -e

BINARY_NAME="cortex-backend"
OUTPUT_DIR="bin"
SOURCE_DIR="examples/go-backend"

echo "Building Go binaries for all platforms..."

# Crear directorio de salida
mkdir -p $OUTPUT_DIR

# Verificar que Go esté instalado
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go first."
    exit 1
fi

# Verificar que existe el código fuente
if [ ! -f "$SOURCE_DIR/main.go" ]; then
    echo "Error: Source file not found: $SOURCE_DIR/main.go"
    exit 1
fi

echo "Compiling for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-linux-amd64 $SOURCE_DIR/main.go

echo "Compiling for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-darwin-amd64 $SOURCE_DIR/main.go

echo "Compiling for macOS ARM64 (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o $OUTPUT_DIR/$BINARY_NAME-darwin-arm64 $SOURCE_DIR/main.go

echo "Compiling for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-windows-amd64.exe $SOURCE_DIR/main.go

# Hacer ejecutables (Unix)
chmod +x $OUTPUT_DIR/$BINARY_NAME-linux-amd64
chmod +x $OUTPUT_DIR/$BINARY_NAME-darwin-amd64
chmod +x $OUTPUT_DIR/$BINARY_NAME-darwin-arm64

echo ""
echo "✓ Binaries compiled successfully in $OUTPUT_DIR/:"
ls -lh $OUTPUT_DIR/










