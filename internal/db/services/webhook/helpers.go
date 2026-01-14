package webhook

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Return as hex string (64 characters)
	return hex.EncodeToString(bytes), nil
}
