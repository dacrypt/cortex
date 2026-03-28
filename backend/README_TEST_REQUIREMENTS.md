# Test Requirements

## PDF Metadata Extraction Tests

The PDF metadata extraction tests require the following external tools:

### Required Tools

1. **pdfinfo** - Part of poppler-utils
   - Extracts comprehensive PDF metadata (title, author, pages, etc.)
   - Used by `PDFExtractor` for primary metadata extraction

2. **exiftool** - Image and metadata extraction tool
   - Extracts XMP metadata and additional PDF properties
   - Used by `PDFExtractor` as a fallback/extended metadata source

### Installation

#### macOS

```bash
brew install poppler exiftool
```

Or use the installation script:

```bash
make install-pdf-tools
```

#### Linux (Debian/Ubuntu)

```bash
sudo apt-get update
sudo apt-get install poppler-utils libimage-exiftool-perl
```

#### Linux (RHEL/CentOS/Fedora)

```bash
# RHEL/CentOS
sudo yum install poppler-utils perl-Image-ExifTool

# Fedora
sudo dnf install poppler-utils perl-Image-ExifTool
```

Or use the installation script:

```bash
make install-pdf-tools
```

### Running Tests

After installing the tools, run the PDF extractor tests:

```bash
# Run all tests (will fail if tools are missing)
make test

# Or run only PDF extractor tests
make test-pdf

# Or run directly
go test -v ./internal/infrastructure/metadata -run TestPDFExtractor
```

### Verification

Verify that the tools are installed:

```bash
pdfinfo -v
exiftool -ver
```

### Test Behavior

The tests will **fail** if the required tools are not available. This ensures that:

1. The development environment has all necessary dependencies
2. The extractor is tested with real tools, not mocked
3. CI/CD pipelines can verify tool availability

If you see errors like:

```
pdfinfo is required but not found. Install it with: brew install poppler (macOS)...
```

Run `make install-pdf-tools` to install the missing tools.






