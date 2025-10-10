package auth

import (
	"all-me-backend/pkg/models"
	"errors"
	"sync"
	"time"
)

// MemoryStore provides in-memory storage for OAuth states and sessions
type MemoryStore struct {
	// OAuth states for CSRF protection (short-lived)
	states map[string]*OAuthState // state -> OAuthState

	// User sessions (long-lived)
	sessions map[string]*models.UserSession // sessionID -> session (with tokens)

	mutex sync.RWMutex
}

// NewMemoryStore creates a new in-memory store and starts cleanup goroutine
func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		states:   make(map[string]*OAuthState),
		sessions: make(map[string]*models.UserSession),
	}

	// Start cleanup goroutine
	go store.startCleanupRoutine()

	return store
}

// === OAuth State Management (CSRF Protection) ===

// GenerateState creates a new OAuth state for the given provider and session
func (m *MemoryStore) GenerateState(provider, sessionID string) (*OAuthState, error) {
	state, err := GenerateSecureState()
	if err != nil {
		return nil, err
	}

	oauthState := &OAuthState{
		State:     state,
		Provider:  provider,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(10 * time.Minute), // State expires in 10 minutes
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.states[state] = oauthState
	return oauthState, nil
}

// ValidateState validates an OAuth state and returns the associated information
func (m *MemoryStore) ValidateState(state string) (*OAuthState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	oauthState, exists := m.states[state]
	if !exists {
		return nil, errors.New("invalid state")
	}

	if !oauthState.IsValid() {
		return nil, errors.New("state expired")
	}

	return oauthState, nil
}

// DeleteState removes an OAuth state
func (m *MemoryStore) DeleteState(state string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.states, state)
	return nil
}

// === Session Management ===

// StoreSession stores a user session with timestamps
func (m *MemoryStore) StoreSession(session *models.UserSession) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Set timestamps if this is a new session
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	if session.LastAccessed.IsZero() {
		session.LastAccessed = time.Now()
	}

	m.sessions[session.SessionID] = session
	return nil
}

// GetSession retrieves a user session by session ID and updates last accessed time
func (m *MemoryStore) GetSession(sessionID string) (*models.UserSession, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	// Check if session is expired
	if session.IsExpired() {
		delete(m.sessions, sessionID)
		return nil, errors.New("session expired")
	}

	// Update last accessed time
	session.UpdateLastAccessed()

	return session, nil
}

// GetSessionCount returns the number of active sessions
func (m *MemoryStore) GetSessionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.sessions)
}

// startCleanupRoutine runs periodic cleanup of expired sessions and OAuth states
func (m *MemoryStore) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredSessions()
		m.cleanupExpiredStates()
	}
}

// cleanupExpiredSessions removes expired sessions (24-hour TTL)
func (m *MemoryStore) cleanupExpiredSessions() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for sessionID, session := range m.sessions {
		if now.Sub(session.LastAccessed) > 24*time.Hour {
			delete(m.sessions, sessionID)
		}
	}
}

// cleanupExpiredStates removes expired OAuth states (10-minute TTL)
func (m *MemoryStore) cleanupExpiredStates() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for state, oauthState := range m.states {
		if !oauthState.IsValid() {
			delete(m.states, state)
		}
	}
}
