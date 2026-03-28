// Package hash provides hashing utilities for Cortex.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

// FileID generates a stable file ID from a relative path.
// The ID is a SHA-256 hash of the normalized path.
func FileID(relativePath string) string {
	// Normalize path separators
	normalized := filepath.ToSlash(relativePath)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// ContentHash generates a SHA-256 hash of content.
func ContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// FileContentHash generates a SHA-256 hash of a file's content.
func FileContentHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// StringHash generates a SHA-256 hash of a string.
func StringHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// ShortHash returns the first 8 characters of a hash.
func ShortHash(fullHash string) string {
	if len(fullHash) < 8 {
		return fullHash
	}
	return fullHash[:8]
}
