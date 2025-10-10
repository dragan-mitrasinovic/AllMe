package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// OAuthState represents OAuth state for security during the flow
type OAuthState struct {
	State     string    `json:"state"`
	Provider  string    `json:"provider"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"` // Unix timestamp
}

// GenerateSecureState creates a cryptographically secure random state string
func GenerateSecureState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// IsValid checks if the OAuth state is still valid
func (s *OAuthState) IsValid() bool {
	return time.Now().Before(s.ExpiresAt)
}
