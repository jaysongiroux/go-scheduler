package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKeySize    = errors.New("encryption key must be 32 bytes")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// Service handles encryption and decryption of sensitive data
type Service struct {
	key []byte
}

// NewService creates a new encryption service with the provided key
func NewService(key []byte) (*Service, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}
	return &Service{key: key}, nil
}

// EncryptCredentials encrypts plaintext credentials using AES-256-GCM
// Returns the ciphertext as a byte slice (IV + ciphertext + tag)
func (s *Service) EncryptCredentials(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and append nonce to the beginning
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// DecryptCredentials decrypts ciphertext credentials using AES-256-GCM
// Expects ciphertext as a byte slice (IV + ciphertext + tag)
func (s *Service) DecryptCredentials(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptCredentialsBase64 encrypts plaintext and returns base64-encoded string
// Useful for storing in text fields
func (s *Service) EncryptCredentialsBase64(plaintext string) (string, error) {
	ciphertext, err := s.EncryptCredentials(plaintext)
	if err != nil {
		return "", err
	}
	if ciphertext == nil {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredentialsBase64 decrypts base64-encoded ciphertext
func (s *Service) DecryptCredentialsBase64(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	return s.DecryptCredentials(ciphertext)
}
