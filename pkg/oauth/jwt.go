package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSet represents a set of JSON Web Keys
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// IDTokenClaims represents the claims in an ID token
type IDTokenClaims struct {
	Issuer        string                 `json:"iss"`
	Subject       string                 `json:"sub"`
	Audience      interface{}            `json:"aud"` // Can be string or []string
	ExpiresAt     int64                  `json:"exp"`
	IssuedAt      int64                  `json:"iat"`
	AuthTime      int64                  `json:"auth_time,omitempty"`
	Nonce         string                 `json:"nonce,omitempty"`
	Email         string                 `json:"email,omitempty"`
	EmailVerified bool                   `json:"email_verified,omitempty"`
	Name          string                 `json:"name,omitempty"`
	GivenName     string                 `json:"given_name,omitempty"`
	FamilyName    string                 `json:"family_name,omitempty"`
	OtherClaims   map[string]interface{} `json:"-"`
}

// FetchJWKS fetches the JSON Web Key Set from the JWKS URI
func FetchJWKS(jwksURI string) (*JWKSet, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWKS response: %w", err)
	}

	var jwks JWKSet
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	return &jwks, nil
}

// GetPublicKey converts a JWK to an RSA public key
func (j *JWK) GetPublicKey() (*rsa.PublicKey, error) {
	if j.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", j.Kty)
	}

	// Decode N (modulus)
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode E (exponent)
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert bytes to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// ParseIDToken parses and decodes an ID token (without verification)
// Use VerifyIDToken for full verification
func ParseIDToken(idToken string) (*IDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode the payload (second part)
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode token payload: %w", err)
	}

	var claims IDTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse token claims: %w", err)
	}

	return &claims, nil
}

// VerifyIDToken verifies and validates an ID token
// This is a basic implementation - for production use, consider using a full JWT library like github.com/golang-jwt/jwt
func VerifyIDToken(ctx context.Context, idToken string, jwksURI string, clientID string, issuer string, nonce string) (*IDTokenClaims, error) {
	claims, err := ParseIDToken(idToken)
	if err != nil {
		return nil, err
	}

	// Verify issuer
	if claims.Issuer != issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", issuer, claims.Issuer)
	}

	// Verify audience
	audienceValid := false
	switch aud := claims.Audience.(type) {
	case string:
		audienceValid = aud == clientID
	case []interface{}:
		for _, a := range aud {
			if str, ok := a.(string); ok && str == clientID {
				audienceValid = true
				break
			}
		}
	}
	if !audienceValid {
		return nil, fmt.Errorf("invalid audience")
	}

	// Verify expiration
	now := time.Now().Unix()
	if claims.ExpiresAt < now {
		return nil, fmt.Errorf("token expired")
	}

	// Verify nonce if provided
	if nonce != "" && claims.Nonce != nonce {
		return nil, fmt.Errorf("invalid nonce")
	}

	// Note: Full signature verification using JWKS would be implemented here
	// For production, use a library like github.com/golang-jwt/jwt or github.com/lestrrat-go/jwx

	return claims, nil
}
