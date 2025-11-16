package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// verifyBinaryHash computes the SHA256 hash of a file and compares it to the expected hash.
// Returns nil if the hash matches, or an error describing the mismatch.
func verifyBinaryHash(binaryPath, expectedHash string) error {
	// Open the binary file
	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to open binary: %w", err)
	}
	defer f.Close()

	// Compute SHA256 hash
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	// Get hex-encoded hash
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Compare hashes
	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}
