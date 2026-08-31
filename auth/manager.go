package auth

import (
	"context"
	"sync"
	"time"

	"aegion-dynamic/apismith/auth/cognito"
)

// Manager holds process-local authentication state. JWTs live in memory only
// and are never written to disk. The browser also keeps its own copy; this is
// used by the CLI and as a server-side cache for the current console session.
type Manager struct {
	mu        sync.RWMutex
	tokens    *cognito.Tokens
	clientID  string
	issuedAt  time.Time
	expiresAt time.Time
}

// NewManager returns an empty auth manager.
func NewManager() *Manager {
	return &Manager{}
}

// Generate fetches tokens from Cognito and stores them in memory.
func (m *Manager) Generate(ctx context.Context, cfg cognito.Config) (*cognito.Tokens, error) {
	tokens, err := cognito.Generate(ctx, cfg)
	if err != nil {
		return nil, err
	}
	m.Set(cfg.ClientID, tokens)
	return tokens, nil
}

// Set replaces in-memory tokens. Refresh tokens are kept for the process
// lifetime but are never logged or written to disk.
func (m *Manager) Set(clientID string, tokens *cognito.Tokens) {
	if m == nil || tokens == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = tokens
	m.clientID = clientID
	m.issuedAt = time.Now()
	if tokens.ExpiresIn > 0 {
		m.expiresAt = m.issuedAt.Add(time.Duration(tokens.ExpiresIn) * time.Second)
	} else {
		m.expiresAt = time.Time{}
	}
}

// AccessToken returns the current access token if it has not expired.
func (m *Manager) AccessToken() (string, bool) {
	if m == nil {
		return "", false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tokens == nil || m.tokens.AccessToken == "" {
		return "", false
	}
	if !m.expiresAt.IsZero() && time.Now().After(m.expiresAt) {
		return "", false
	}
	return m.tokens.AccessToken, true
}

// Status is a public, non-sensitive snapshot of authentication state.
type Status struct {
	Authenticated bool      `json:"authenticated"`
	ClientID      string    `json:"client_id,omitempty"`
	IssuedAt      time.Time `json:"issued_at,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	TokenPreview  string    `json:"token_preview,omitempty"`
}

// Snapshot returns UI-safe status. The JWT itself is not included.
func (m *Manager) Snapshot() Status {
	if m == nil {
		return Status{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tokens == nil || m.tokens.AccessToken == "" {
		return Status{Authenticated: false}
	}
	expired := !m.expiresAt.IsZero() && time.Now().After(m.expiresAt)
	preview := m.tokens.AccessToken
	if len(preview) > 24 {
		preview = preview[:12] + "…" + preview[len(preview)-8:]
	}
	return Status{
		Authenticated: !expired,
		ClientID:      m.clientID,
		IssuedAt:      m.issuedAt,
		ExpiresAt:     m.expiresAt,
		TokenPreview:  preview,
	}
}

// Clear drops in-memory tokens.
func (m *Manager) Clear() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = nil
	m.clientID = ""
	m.issuedAt = time.Time{}
	m.expiresAt = time.Time{}
}
