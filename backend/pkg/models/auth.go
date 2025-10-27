package models

import (
	"time"
)

// Token represents an OAuth token for cloud storage providers
type Token struct {
	AccessToken string `json:"access_token"`
	Provider    string `json:"provider"` // "onedrive" or "googledrive"
	Scope       string `json:"scope,omitempty"`
}

// OAuthConfig holds OAuth configuration for a specific provider
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURI  string   `json:"redirect_uri"`
	Scopes       []string `json:"scopes"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	Provider     string   `json:"provider"`
}

// UserSession represents a user's session with authentication tokens for multiple providers
type UserSession struct {
	SessionID    string            `json:"session_id"`
	Tokens       map[string]*Token `json:"tokens"` // map of provider -> token
	CreatedAt    time.Time         `json:"created_at"`
	LastAccessed time.Time         `json:"last_accessed"`
}

// IsExpired checks if the session has expired (24-hour TTL)
func (s *UserSession) IsExpired() bool {
	return time.Since(s.LastAccessed) > 24*time.Hour
}

// UpdateLastAccessed updates the last accessed timestamp
func (s *UserSession) UpdateLastAccessed() {
	s.LastAccessed = time.Now()
}

// GetToken retrieves the token for a specific provider
func (s *UserSession) GetToken(provider string) *Token {
	if s.Tokens == nil {
		return nil
	}
	return s.Tokens[provider]
}

// SetToken sets the token for a specific provider
func (s *UserSession) SetToken(provider string, token *Token) {
	if s.Tokens == nil {
		s.Tokens = make(map[string]*Token)
	}
	s.Tokens[provider] = token
}

// HasTokenForProvider checks if a valid token exists for the provider
func (s *UserSession) HasTokenForProvider(provider string) bool {
	token := s.GetToken(provider)
	return token != nil && token.Provider == provider
}

// SessionStore interface for retrieving sessions
type SessionStore interface {
	GetSessionToken(sessionID, provider string) (*Token, error)
}
