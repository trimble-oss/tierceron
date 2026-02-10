package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCEChallenge holds the PKCE code verifier and challenge
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}

// GeneratePKCEChallenge creates a new PKCE code verifier and challenge using S256 method
func GeneratePKCEChallenge() (*PKCEChallenge, error) {
	// Generate a random 32-byte code verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Base64 URL encode the verifier
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Create SHA256 hash of the verifier for the challenge
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// GenerateState creates a random state value for CSRF protection
func GenerateState() (string, error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(stateBytes), nil
}

// GenerateNonce creates a random nonce value for token validation
func GenerateNonce() (string, error) {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(nonceBytes), nil
}
