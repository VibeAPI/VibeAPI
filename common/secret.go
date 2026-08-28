package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const encryptedSecretPrefix = "enc:v1:"

func secretEncryptionKey() []byte {
	sum := sha256.Sum256([]byte("new-api:prompt-audit:v1\x00" + CryptoSecret))
	return sum[:]
}

func EncryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(secretEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return encryptedSecretPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) <= len(encryptedSecretPrefix) || value[:len(encryptedSecretPrefix)] != encryptedSecretPrefix {
		return "", fmt.Errorf("unsupported encrypted secret format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(value[len(encryptedSecretPrefix):])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(secretEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted secret is truncated")
	}
	plaintext, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
