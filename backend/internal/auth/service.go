package auth

import (
	"all-me-backend/pkg/models"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Service handles OAuth authentication for cloud storage providers
type Service struct {
	store           *MemoryStore
	httpClient      *http.Client
	googleDriveAuth Provider
	oneDriveAuth    Provider
}

func NewService(googleDriveAuth, oneDriveAuth Provider) *Service {
	return &Service{
		store:           NewMemoryStore(),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		googleDriveAuth: googleDriveAuth,
		oneDriveAuth:    oneDriveAuth,
	}
}

// InitiateOAuth starts the OAuth flow for a provider, returning the auth URL
func (s *Service) InitiateOAuth(provider, sessionID string) (string, error) {
	if !s.validateProvider(provider) {
		return "", errors.New("unsupported provider: " + provider)
	}

	oauthState, err := s.store.GenerateState(provider, sessionID)
	if err != nil {
		return "", err
	}

	// Build authorization URL using provider-specific implementation
	var authURL string
	switch provider {
	case "googledrive":
		authURL, err = s.googleDriveAuth.BuildAuthURL(oauthState.State)
	case "onedrive":
		authURL, err = s.oneDriveAuth.BuildAuthURL(oauthState.State)
	default:
		return "", errors.New("unsupported provider: " + provider)
	}

	if err != nil {
		return "", err
	}

	return authURL, nil
}

// HandleCallback processes the OAuth callback and exchanges code for token
func (s *Service) HandleCallback(provider, code, state string) (*models.Token, error) {
	if !s.validateProvider(provider) {
		return nil, errors.New("unsupported provider: " + provider)
	}

	oauthState, err := s.store.ValidateState(state)
	if err != nil {
		return nil, err
	}

	// Verify provider matches the one in state
	if oauthState.Provider != provider {
		return nil, errors.New("provider mismatch in OAuth state")
	}

	defer s.store.DeleteState(state)

	config, err := s.getProviderConfig(oauthState.Provider)
	if err != nil {
		return nil, err
	}

	token, err := s.exchangeCodeForToken(config, code)
	if err != nil {
		return nil, err
	}

	// Get or create session
	session, err := s.store.GetSession(oauthState.SessionID)
	if err != nil {
		session = &models.UserSession{
			SessionID: oauthState.SessionID,
			Tokens:    make(map[string]*models.Token),
		}
	}

	session.SetToken(oauthState.Provider, token)

	err = s.store.StoreSession(session)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// exchangeCodeForToken exchanges authorization code for access token
func (s *Service) exchangeCodeForToken(config *models.OAuthConfig, code string) (*models.Token, error) {
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", config.RedirectURI)
	data.Set("scope", strings.Join(config.Scopes, " "))

	req, err := http.NewRequest("POST", config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}

	token := &models.Token{
		AccessToken: tokenResponse.AccessToken,
		Provider:    config.Provider,
		Scope:       tokenResponse.Scope,
	}

	return token, nil
}

func (s *Service) getProviderConfig(provider string) (*models.OAuthConfig, error) {
	switch provider {
	case "googledrive":
		config := s.googleDriveAuth.GetOAuthConfig()
		if config.ClientID == "" || config.ClientSecret == "" {
			return nil, errors.New("OAuth configuration incomplete for provider: " + provider)
		}
		return config, nil
	case "onedrive":
		config := s.oneDriveAuth.GetOAuthConfig()
		if config.ClientID == "" || config.ClientSecret == "" {
			return nil, errors.New("OAuth configuration incomplete for provider: " + provider)
		}
		return config, nil
	default:
		return nil, errors.New("unsupported provider: " + provider)
	}
}

// validateProvider checks if a provider is supported (internal use only)
func (s *Service) validateProvider(provider string) bool {
	return provider == "googledrive" || provider == "onedrive"
}

// GetSessionToken retrieves a session and returns the token for the specified provider
func (s *Service) GetSessionToken(sessionID, provider string) (*models.Token, error) {
	return s.store.GetSessionToken(sessionID, provider)
}

// SignOutProvider removes the token for a specific provider from the session
func (s *Service) SignOutProvider(sessionID, provider string) error {
	if !s.validateProvider(provider) {
		return errors.New("unsupported provider: " + provider)
	}

	session, err := s.store.GetSession(sessionID)
	if err != nil {
		// Session doesn't exist - this is okay, just return success
		return nil
	}

	// Remove the token for this provider
	if session.Tokens != nil {
		delete(session.Tokens, provider)
	}

	// Update the session in the store
	return s.store.StoreSession(session)
}
