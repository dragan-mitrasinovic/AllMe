package auth

import "all-me-backend/pkg/models"

// Provider defines the interface needed from an OAuth provider
type Provider interface {
	GetOAuthConfig() *models.OAuthConfig
	BuildAuthURL(state string) (string, error)
}
