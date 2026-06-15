package util

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateToken returns a random 32-character hex secret used as a
// repository upload token.
func GenerateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
