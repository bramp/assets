package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func FileSHA256(path string) (string, error) {
	hash, _, err := FileSHA256AndSize(path)
	return hash, err
}

// FileSHA256AndSize returns a SHA256 hash and byte size in one read pass.
func FileSHA256AndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
