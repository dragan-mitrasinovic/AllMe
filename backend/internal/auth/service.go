package auth

import (
	"all-me-backend/pkg/models"
	"crypto/subtle"
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

// HandleCallback processes the OAuth callback, exchanges the code for a token and stores it in the
// session that started the login. cookieSessionID is the session cookie sent with the callback request:
// it must match the session the state was issued to, so a login link can't be completed in another
// browser. On success the session moves to a new ID, which is returned and must be sent as the new cookie.
func (s *Service) HandleCallback(provider, code, state, cookieSessionID string) (*models.Token, string, error) {
	if !s.validateProvider(provider) {
		return nil, "", errors.New("unsupported provider: " + provider)
	}

	oauthState, err := s.store.ValidateState(state)
	if err != nil {
		return nil, "", err
	}

	// Verify provider matches the one in state
	if oauthState.Provider != provider {
		return nil, "", errors.New("provider mismatch in OAuth state")
	}

	defer s.store.DeleteState(state)

	if cookieSessionID == "" ||
		subtle.ConstantTimeCompare([]byte(oauthState.SessionID), []byte(cookieSessionID)) != 1 {
		return nil, "", errors.New("session mismatch in OAuth state")
	}

	userSession, err := s.store.GetSession(oauthState.SessionID)
	if err != nil {
		return nil, "", err
	}

	config, err := s.getProviderConfig(oauthState.Provider)
	if err != nil {
		return nil, "", err
	}

	token, err := s.exchangeCodeForToken(config, code)
	if err != nil {
		return nil, "", err
	}

	s.store.mutex.Lock()
	userSession.SetToken(oauthState.Provider, token)
	s.store.mutex.Unlock()

	rotated, err := s.store.RotateSessionID(oauthState.SessionID)
	if err != nil {
		return nil, "", err
	}

	return token, rotated.SessionID, nil
}

// ResolveSession reports whether sessionID belongs to a live session.
func (s *Service) ResolveSession(sessionID string) bool {
	return s.store.SessionExists(sessionID)
}

// CreateSession starts a new, empty session and returns its ID.
func (s *Service) CreateSession() (string, error) {
	newSession, err := s.store.CreateSession()
	if err != nil {
		return "", err
	}
	return newSession.SessionID, nil
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

// SignOutProvider removes the token for a specific provider from the session. When no provider is left
// signed in, the whole session is deleted and sessionEnded is true.
func (s *Service) SignOutProvider(sessionID, provider string) (sessionEnded bool, err error) {
	if !s.validateProvider(provider) {
		return false, errors.New("unsupported provider: " + provider)
	}

	userSession, err := s.store.GetSession(sessionID)
	if err != nil {
		// Session doesn't exist - this is okay, just return success
		return true, nil
	}

	s.store.mutex.Lock()
	delete(userSession.Tokens, provider)
	remaining := len(userSession.Tokens)
	s.store.mutex.Unlock()

	if remaining == 0 {
		s.store.DeleteSession(sessionID)
		return true, nil
	}
	return false, nil
}
