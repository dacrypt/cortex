#!/bin/bash
# Script to install PDF metadata extraction tools required for tests

set -e

echo "Installing PDF metadata extraction tools..."

# Detect OS
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    echo "Detected macOS"
    
    if ! command -v brew &> /dev/null; then
        echo "Error: Homebrew is required but not installed."
        echo "Install it from https://brew.sh"
        exit 1
    fi
    
    echo "Installing poppler (provides pdfinfo)..."
    brew install poppler || echo "poppler may already be installed"
    
    echo "Installing exiftool..."
    brew install exiftool || echo "exiftool may already be installed"
    
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    echo "Detected Linux"
    
    if command -v apt-get &> /dev/null; then
        # Debian/Ubuntu
        echo "Installing poppler-utils (provides pdfinfo)..."
        sudo apt-get update
        sudo apt-get install -y poppler-utils
        
        echo "Installing exiftool..."
        sudo apt-get install -y libimage-exiftool-perl
        
    elif command -v yum &> /dev/null; then
        # RHEL/CentOS
        echo "Installing poppler-utils (provides pdfinfo)..."
        sudo yum install -y poppler-utils
        
        echo "Installing exiftool..."
        sudo yum install -y perl-Image-ExifTool
        
    elif command -v dnf &> /dev/null; then
        # Fedora
        echo "Installing poppler-utils (provides pdfinfo)..."
        sudo dnf install -y poppler-utils
        
        echo "Installing exiftool..."
        sudo dnf install -y perl-Image-ExifTool
        
    else
        echo "Error: Unsupported Linux distribution. Please install manually:"
        echo "  - poppler-utils (provides pdfinfo)"
        echo "  - libimage-exiftool-perl or perl-Image-ExifTool (provides exiftool)"
        exit 1
    fi
    
else
    echo "Error: Unsupported OS: $OSTYPE"
    echo "Please install manually:"
    echo "  - pdfinfo (part of poppler-utils)"
    echo "  - exiftool"
    exit 1
fi

# Verify installation
echo ""
echo "Verifying installation..."

if command -v pdfinfo &> /dev/null; then
    echo "✓ pdfinfo is installed: $(pdfinfo -v 2>&1 | head -1)"
else
    echo "✗ pdfinfo is not found in PATH"
    exit 1
fi

if command -v exiftool &> /dev/null; then
    echo "✓ exiftool is installed: $(exiftool -ver 2>&1)"
else
    echo "✗ exiftool is not found in PATH"
    exit 1
fi

echo ""
echo "✓ All PDF metadata extraction tools are installed!"






