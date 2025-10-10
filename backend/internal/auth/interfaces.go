package auth

import "all-me-backend/pkg/models"

// Provider defines the interface for authentication operations with cloud providers
type Provider interface {
	GetOAuthConfig() *models.OAuthConfig
	BuildAuthURL(state string) (string, error)
}
