package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const appleIssuer = "https://appleid.apple.com"

// AppleVerifier validates a Sign in with Apple identity token and returns the
// stable user id (sub). KeyFunc supplies Apple's public keys — JWKS-backed in
// prod (NewAppleVerifier), a static key in tests. Now is injectable for tests.
type AppleVerifier struct {
	BundleID string // expected `aud` — the iOS app's bundle id
	KeyFunc  jwt.Keyfunc
	Now      func() time.Time
}

// NewAppleVerifier builds a verifier backed by Apple's live JWKS endpoint.
func NewAppleVerifier(bundleID string, httpTimeout time.Duration) *AppleVerifier {
	return &AppleVerifier{BundleID: bundleID, KeyFunc: newAppleJWKS(httpTimeout).keyfunc()}
}

// Verify checks the token's signature against Apple's keys and validates
// issuer (apple), audience (our bundle id) and expiry, then returns `sub`.
func (v *AppleVerifier) Verify(identityToken string) (string, error) {
	if v.BundleID == "" {
		return "", errors.New("auth: apple bundle id not configured")
	}
	timeFunc := time.Now
	if v.Now != nil {
		timeFunc = v.Now
	}
	claims := &jwt.RegisteredClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.BundleID),
		jwt.WithTimeFunc(timeFunc),
	)
	if _, err := parser.ParseWithClaims(identityToken, claims, v.KeyFunc); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("auth: apple token missing sub")
	}
	return claims.Subject, nil
}
