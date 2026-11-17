package auth

import (
	"all-me-backend/pkg/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func createTestService(tokenURL string) *Service {
	mockOneDrive := &mockAuthProvider{tokenURL: tokenURL, provider: "onedrive"}
	mockGoogleDrive := &mockAuthProvider{tokenURL: tokenURL, provider: "googledrive"}
	return NewService(mockGoogleDrive, mockOneDrive)
}

func TestAuthService_HandleCallback_Success(t *testing.T) {
	// Create mock token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-www-form-urlencoded" {
			t.Errorf("Expected content type 'application/x-www-form-urlencoded', got '%s'", contentType)
		}

		// Return mock token response
		response := map[string]interface{}{
			"access_token":  "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "Files.Read",
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	service := createTestService(server.URL)

	// Create a session first
	session := &models.UserSession{
		SessionID: "test-session",
	}
	err := service.store.StoreSession(session)
	if err != nil {
		t.Fatalf("Failed to store session: %v", err)
	}

	// Generate a valid state
	state, err := service.store.GenerateState("onedrive", "test-session")
	if err != nil {
		t.Fatalf("Failed to generate state: %v", err)
	}

	// Test callback handling
	token, newSessionID, err := service.HandleCallback("onedrive", "test-code", state.State, "test-session")
	if err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}

	// The session moves to a new ID; the pre-login ID is no longer valid
	if newSessionID == "" || newSessionID == "test-session" {
		t.Errorf("Expected a new session ID, got %q", newSessionID)
	}
	if service.ResolveSession("test-session") {
		t.Error("Expected the pre-login session ID to be invalid after login")
	}
	if stored, err := service.GetSessionToken(newSessionID, "onedrive"); err != nil || stored.AccessToken != "mock-access-token" {
		t.Errorf("Expected token stored under the new session ID, got %v, %v", stored, err)
	}

	// Verify token properties
	if token.AccessToken != "mock-access-token" {
		t.Errorf("Expected access token 'mock-access-token', got '%s'", token.AccessToken)
	}

	if token.Provider != "onedrive" {
		t.Errorf("Expected provider 'onedrive', got '%s'", token.Provider)
	}
}

func TestAuthService_HandleCallback_InvalidState(t *testing.T) {
	service := createTestService("")

	_, _, err := service.HandleCallback("onedrive", "test-code", "invalid-state", "test-session")
	if err == nil {
		t.Error("Expected error for invalid state, got nil")
	}

	if !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("Expected 'invalid state' error, got: %v", err)
	}
}

func TestAuthService_HandleCallback_ExpiredState(t *testing.T) {
	service := createTestService("")

	// Create a session first
	session := &models.UserSession{
		SessionID: "test-session",
	}
	err := service.store.StoreSession(session)
	if err != nil {
		t.Fatalf("Failed to store session: %v", err)
	}

	// Generate state and manually expire it
	state, err := service.store.GenerateState("onedrive", "test-session")
	if err != nil {
		t.Fatalf("Failed to generate state: %v", err)
	}

	// Manually expire the state by setting its expiry to the past
	state.ExpiresAt = time.Now().Add(-1 * time.Hour)
	service.store.states[state.State] = state

	_, _, err = service.HandleCallback("onedrive", "test-code", state.State, "test-session")
	if err == nil {
		t.Error("Expected error for expired state, got nil")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected 'expired' error, got: %v", err)
	}
}

func TestAuthService_InitiateOAuth_UnsupportedProvider(t *testing.T) {
	service := createTestService("")

	_, err := service.InitiateOAuth("unsupported", "test-session")
	if err == nil {
		t.Error("Expected error for unsupported provider, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("Expected 'unsupported provider' error, got: %v", err)
	}
}

func TestAuthService_HandleCallback_UnsupportedProvider(t *testing.T) {
	service := createTestService("")

	_, _, err := service.HandleCallback("unsupported", "test-code", "test-state", "test-session")
	if err == nil {
		t.Error("Expected error for unsupported provider, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("Expected 'unsupported provider' error, got: %v", err)
	}
}

func TestAuthService_HandleCallback_ProviderMismatch(t *testing.T) {
	service := createTestService("")

	// Create a session first
	session := &models.UserSession{
		SessionID: "test-session",
	}
	err := service.store.StoreSession(session)
	if err != nil {
		t.Fatalf("Failed to store session: %v", err)
	}

	// Generate state for onedrive
	state, err := service.store.GenerateState("onedrive", "test-session")
	if err != nil {
		t.Fatalf("Failed to generate state: %v", err)
	}

	// Try to use the state with a different provider
	_, _, err = service.HandleCallback("googledrive", "test-code", state.State, "test-session")
	if err == nil {
		t.Error("Expected error for provider mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "provider mismatch") {
		t.Errorf("Expected 'provider mismatch' error, got: %v", err)
	}
}

// mockAuthProvider is a test implementation of AuthProvider
type mockAuthProvider struct {
	tokenURL string
	provider string
}

func (m *mockAuthProvider) GetOAuthConfig() *models.OAuthConfig {
	return &models.OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
		Scopes:       []string{"Files.Read"},
		AuthURL:      "https://example.com/auth",
		TokenURL:     m.tokenURL,
		Provider:     m.provider,
	}
}

func (m *mockAuthProvider) BuildAuthURL(state string) (string, error) {
	config := m.GetOAuthConfig()
	return config.AuthURL + "?client_id=" + config.ClientID + "&state=" + state, nil
}

// Token reads (every data request) run concurrently with logins and sign-outs that modify the token map;
// an unlocked read would make the Go runtime abort with "concurrent map read and map write".
func TestSessionTokens_ConcurrentReadAndWrite(t *testing.T) {
	service := createTestService("")
	sessionID, err := service.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	userSession, _ := service.store.GetSession(sessionID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20000; i++ {
			service.store.mutex.Lock()
			userSession.SetToken("onedrive", &models.Token{AccessToken: "t", Provider: "onedrive"})
			userSession.SetToken("googledrive", &models.Token{AccessToken: "t", Provider: "googledrive"})
			service.store.mutex.Unlock()
			_, _ = service.SignOutProvider(sessionID, "googledrive")
		}
	}()
	for i := 0; i < 20000; i++ {
		_, _ = service.GetSessionToken(sessionID, "onedrive")
	}
	<-done
}
