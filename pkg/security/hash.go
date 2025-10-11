package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	ArgonTime    uint32 = 1         // jumlah iterasi
	ArgonMemory  uint32 = 64 * 1024 // 64 MB
	ArgonThreads uint8  = 4
	ArgonKeyLen  uint32 = 32
	SaltLength   uint32 = 16
)

// HashSHA256 melakukan hashing menggunakan SHA-256
func HashSHA256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

func HashArgon2(password string) (string, error) {
	salt, err := GenerateSalt()
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)

	// Encode ke base64
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	fullHash := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		ArgonMemory, ArgonTime, ArgonThreads, saltB64, hashB64)

	return fullHash, nil
}

func VerifyHashArgon2(password string, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		log.Println("invalid hash format")
		return false
	}

	var memory uint32
	var time uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		log.Println("failed to parse parameters:", err)
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		log.Println("failed to decode salt:", err)
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		log.Println("failed to decode hash:", err)
		return false
	}

	computedHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))

	if subtle.ConstantTimeCompare(expectedHash, computedHash) == 1 {
		return true
	}

	log.Println("password verification failed")
	return false
}

func GenerateBase64Secret(length int) (string, error) {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		fmt.Printf("Error generating AES key: %v\n", err)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func ValidateBase64Secret(secret string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		fmt.Printf("Invalid AES secret (must be base64 encoded): %v\n", err)
		return []byte{}, err
	}

	keyLen := len(keyBytes)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		fmt.Printf("Invalid AES key size: %d bytes (must be 16, 24, or 32)\n", keyLen)
		return []byte{}, err
	}

	return keyBytes, nil
}
