package remote

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		expectErr bool
	}{
		{"Valid length 16", 16, false},
		{"Valid length 32", 32, false},
		{"Valid length 64", 64, false},
		{"Invalid length 8", 8, true},
		{"Invalid length 0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateToken(tt.length)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(token) < tt.length {
				t.Errorf("Token too short: got %d, want at least %d", len(token), tt.length)
			}
		})
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token-123"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("Invalid hash length: got %d, want 64", len(hash1))
	}

	// Different tokens should produce different hashes
	differentToken := "different-token-456"
	hash3 := HashToken(differentToken)

	if hash1 == hash3 {
		t.Errorf("Different tokens should produce different hashes")
	}
}

func TestValidateTokenFormat(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		expectErr bool
	}{
		{"Valid base64 token", "dGVzdC10b2tlbi0xMjM0NTY3ODkwMTIzNDU2", false},
		{"Valid alphanumeric token", "test-token-1234567890", false},
		{"Valid with underscores", "test_token_12345678", false},
		{"Too short", "short", true},
		{"Invalid characters", "token with spaces", true},
		{"Invalid special chars", "token@with#special", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokenFormat(tt.token)

			if tt.expectErr && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestTokenInfoExpiration(t *testing.T) {
	token := "test-token"

	t.Run("Never expires", func(t *testing.T) {
		info := CreateTokenInfo(token, "user1", 0)
		if info.IsExpired() {
			t.Errorf("Token should never expire")
		}
		if info.TimeUntilExpiration() != nil {
			t.Errorf("Time until expiration should be nil for non-expiring tokens")
		}
	})

	t.Run("Not yet expired", func(t *testing.T) {
		info := CreateTokenInfo(token, "user1", 1*time.Hour)
		if info.IsExpired() {
			t.Errorf("Token should not be expired yet")
		}

		remaining := info.TimeUntilExpiration()
		if remaining == nil {
			t.Errorf("Time until expiration should not be nil")
		}
		if *remaining <= 0 {
			t.Errorf("Time until expiration should be positive")
		}
	})

	t.Run("Expired", func(t *testing.T) {
		info := CreateTokenInfo(token, "user1", 1*time.Millisecond)
		time.Sleep(10 * time.Millisecond) // Wait for it to expire
		if !info.IsExpired() {
			t.Errorf("Token should be expired")
		}
	})
}

func TestTokenManager(t *testing.T) {
	manager := NewTokenManager()

	token1 := "token-1"
	token2 := "token-2"
	expiredToken := "expired-token"

	// Add tokens
	manager.AddToken(CreateTokenInfo(token1, "user1", 1*time.Hour))
	manager.AddToken(CreateTokenInfo(token2, "user2", 1*time.Hour))
	manager.AddToken(CreateTokenInfo(expiredToken, "user3", 1*time.Millisecond)) // Will expire

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	t.Run("Validate valid token", func(t *testing.T) {
		valid, info := manager.ValidateToken(token1)
		if !valid {
			t.Errorf("Token should be valid")
		}
		if info == nil || info.UserID != "user1" {
			t.Errorf("Token info incorrect")
		}
	})

	t.Run("Validate invalid token", func(t *testing.T) {
		valid, _ := manager.ValidateToken("nonexistent-token")
		if valid {
			t.Errorf("Token should be invalid")
		}
	})

	t.Run("Validate expired token", func(t *testing.T) {
		valid, info := manager.ValidateToken(expiredToken)
		if valid {
			t.Errorf("Expired token should be invalid")
		}
		if info == nil {
			t.Errorf("Token info should still be returned for expired tokens")
		}
	})

	t.Run("Cleanup expired tokens", func(t *testing.T) {
		removed := manager.CleanupExpiredTokens()
		if removed != 1 {
			t.Errorf("Expected 1 expired token to be removed, got %d", removed)
		}

		// Verify expired token is gone
		valid, _ := manager.ValidateToken(expiredToken)
		if valid {
			t.Errorf("Expired token should have been removed")
		}
	})

	t.Run("Remove token", func(t *testing.T) {
		manager.RemoveToken(token1)
		valid, _ := manager.ValidateToken(token1)
		if valid {
			t.Errorf("Removed token should be invalid")
		}
	})

	t.Run("Get token count", func(t *testing.T) {
		count := manager.GetTokenCount()
		if count != 1 { // Only token2 should remain
			t.Errorf("Expected 1 token, got %d", count)
		}
	})
}
