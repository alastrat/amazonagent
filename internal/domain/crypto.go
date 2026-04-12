package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// AESEncryptor provides AES-256-GCM encryption for credential storage.
// If no key is provided, it operates in plaintext passthrough mode (dev only).
type AESEncryptor struct {
	gcm     cipher.AEAD
	devMode bool
}

// NewAESEncryptor creates an encryptor from a hex-encoded 32-byte key.
// If keyHex is empty, returns a passthrough encryptor that stores plaintext (dev mode).
func NewAESEncryptor(keyHex string) (*AESEncryptor, error) {
	if keyHex == "" {
		env := os.Getenv("ENVIRONMENT")
		if env == "production" || env == "prod" {
			return nil, fmt.Errorf("ENCRYPTION_KEY must be set in production")
		}
		slog.Warn("crypto: ENCRYPTION_KEY not set — credentials will be stored in PLAINTEXT (dev mode only)")
		return &AESEncryptor{devMode: true}, nil
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &AESEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext and returns a base64-encoded ciphertext.
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	if e.devMode {
		return plaintext, nil
	}

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext and returns plaintext.
func (e *AESEncryptor) Decrypt(encoded string) (string, error) {
	if e.devMode {
		return encoded, nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
