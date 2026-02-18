package providers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	// MasterKey is loaded from environment variable MODELTUNNEL_MASTER_KEY
	// If not set, a default key is used (NOT SECURE for production)
	MasterKey []byte
)

func init() {
	// Load master key from environment
	keyStr := os.Getenv("MODELTUNNEL_MASTER_KEY")
	if keyStr == "" {
		// Fallback to a default key - should be changed in production
		keyStr = "modeltunnel-default-master-key-32bytes"
	}

	// Ensure key is 32 bytes for AES-256
	MasterKey = make([]byte, 32)
	copy(MasterKey, []byte(keyStr))
}

// Encrypt encrypts plaintext using AES-256-GCM
func Encrypt(plaintext string) (string, error) {
	if len(MasterKey) == 0 {
		return "", errors.New("master key not initialized")
	}

	block, err := aes.NewCipher(MasterKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func Decrypt(ciphertext string) (string, error) {
	if len(MasterKey) == 0 {
		return "", errors.New("master key not initialized")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(MasterKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MaskKey masks an API key for display (shows only last 4 characters)
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}

// ValidateMasterKey checks if the master key is set and secure
func ValidateMasterKey() error {
	keyStr := os.Getenv("MODELTUNNEL_MASTER_KEY")
	if keyStr == "" {
		return errors.New("MODELTUNNEL_MASTER_KEY environment variable not set. Using default key (INSECURE for production)")
	}

	// Check if using default key
	defaultKey := "modeltunnel-default-master-key-32bytes"
	if subtle.ConstantTimeCompare([]byte(keyStr), []byte(defaultKey)) == 1 {
		return errors.New("using default master key. Please set MODELTUNNEL_MASTER_KEY environment variable for security")
	}

	return nil
}
