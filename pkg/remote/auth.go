package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// TokenInfo contains metadata about an authentication token
type TokenInfo struct {
	Token     string
	Hash      string // SHA256 hash of the token for storage
	CreatedAt time.Time
	ExpiresAt *time.Time // Optional expiration
	UserID    string     // Optional user identifier
}

// GenerateToken generates a cryptographically secure random token
func GenerateToken(length int) (string, error) {
	if length < 16 {
		return "", fmt.Errorf("token length must be at least 16 bytes")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 URL-safe string
	token := base64.URLEncoding.EncodeToString(bytes)
	return token, nil
}

// HashToken creates a SHA256 hash of a token for secure storage
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ValidateTokenFormat checks if a token has a valid format
func ValidateTokenFormat(token string) error {
	if len(token) < 16 {
		return fmt.Errorf("token too short (minimum 16 characters)")
	}

	// Attempt to decode from base64
	_, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		// Token might not be base64 encoded, which is ok for simple tokens
		// Just check it contains only valid characters
		for _, c := range token {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_') {
				return fmt.Errorf("token contains invalid characters")
			}
		}
	}

	return nil
}

// CreateTokenInfo creates a new TokenInfo struct
func CreateTokenInfo(token string, userID string, expiresIn time.Duration) *TokenInfo {
	info := &TokenInfo{
		Token:     token,
		Hash:      HashToken(token),
		CreatedAt: time.Now(),
		UserID:    userID,
	}

	if expiresIn > 0 {
		expiry := time.Now().Add(expiresIn)
		info.ExpiresAt = &expiry
	}

	return info
}

// IsExpired checks if a token has expired
func (t *TokenInfo) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false // Never expires
	}
	return time.Now().After(*t.ExpiresAt)
}

// TimeUntilExpiration returns the duration until expiration
func (t *TokenInfo) TimeUntilExpiration() *time.Duration {
	if t.ExpiresAt == nil {
		return nil
	}
	duration := time.Until(*t.ExpiresAt)
	return &duration
}

// TokenManager manages authentication tokens
type TokenManager struct {
	tokens map[string]*TokenInfo // token -> TokenInfo
}

// NewTokenManager creates a new token manager
func NewTokenManager() *TokenManager {
	return &TokenManager{
		tokens: make(map[string]*TokenInfo),
	}
}

// AddToken adds a token to the manager
func (m *TokenManager) AddToken(info *TokenInfo) {
	m.tokens[info.Token] = info
}

// ValidateToken checks if a token is valid and not expired
func (m *TokenManager) ValidateToken(token string) (bool, *TokenInfo) {
	info, exists := m.tokens[token]
	if !exists {
		return false, nil
	}

	if info.IsExpired() {
		return false, info
	}

	return true, info
}

// RemoveToken removes a token from the manager
func (m *TokenManager) RemoveToken(token string) {
	delete(m.tokens, token)
}

// CleanupExpiredTokens removes all expired tokens
func (m *TokenManager) CleanupExpiredTokens() int {
	removed := 0
	for token, info := range m.tokens {
		if info.IsExpired() {
			delete(m.tokens, token)
			removed++
		}
	}
	return removed
}

// GetTokenCount returns the number of active tokens
func (m *TokenManager) GetTokenCount() int {
	return len(m.tokens)
}
