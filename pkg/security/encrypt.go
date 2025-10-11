package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Fungsi Encrypt menggunakan AES-GCM
func Encrypt(plainText string, secret []byte) (string, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize()) // Nonce size dari AES-GCM (biasanya 12 byte)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Enkripsi menggunakan AES-GCM
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plainText), nil)

	// Gabungkan nonce + ciphertext
	finalCiphertext := append(nonce, ciphertext...)

	// Encode ke base64 agar bisa disimpan sebagai string
	return base64.StdEncoding.EncodeToString(finalCiphertext), nil
}

// Fungsi Decrypt menggunakan AES-GCM
func Decrypt(encryptedText string, secret []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Pisahkan nonce dan ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Dekripsi
	plainText, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}
