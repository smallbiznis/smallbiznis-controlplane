package util

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateVerificationCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
