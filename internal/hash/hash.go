package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// TODO(bramp): Is than an existing helper to do this?
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
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
