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

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		states:   make(map[string]*OAuthState),
		sessions: make(map[string]*models.UserSession),
	}

	go store.startCleanupRoutine()

	return store
}

// === OAuth State Management (CSRF Protection) ===

func (m *MemoryStore) GenerateState(provider, sessionID string) (*OAuthState, error) {
	state, err := GenerateSecureState()
	if err != nil {
		return nil, err
	}

	oauthState := &OAuthState{
		State:     state,
		Provider:  provider,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.states[state] = oauthState
	return oauthState, nil
}

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

func (m *MemoryStore) DeleteState(state string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.states, state)
	return nil
}

// === Session Management ===

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

func (m *MemoryStore) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredSessions()
		m.cleanupExpiredStates()
	}
}

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

func (m *MemoryStore) cleanupExpiredStates() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for state, oauthState := range m.states {
		if !oauthState.IsValid() {
			delete(m.states, state)
		}
	}
}
