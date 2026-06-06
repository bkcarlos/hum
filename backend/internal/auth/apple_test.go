package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signAppleToken builds an RS256 token like Apple's, signed with a test key, and
// a matching KeyFunc that resolves it by kid — letting us exercise the claim
// validation (iss/aud/exp/sub) without hitting Apple's network JWKS.
func signAppleToken(t *testing.T, key *rsa.PrivateKey, claims jwt.RegisteredClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "testkid"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testVerifier(t *testing.T, key *rsa.PrivateKey, now time.Time) *AppleVerifier {
	t.Helper()
	return &AppleVerifier{
		BundleID: "com.carlosbk.hum",
		Now:      func() time.Time { return now },
		KeyFunc: func(tok *jwt.Token) (any, error) {
			if kid, _ := tok.Header["kid"].(string); kid != "testkid" {
				return nil, jwt.ErrTokenUnverifiable
			}
			return &key.PublicKey, nil
		},
	}
}

func validClaims(now time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    appleIssuer,
		Subject:   "001234.abcdef.5678",
		Audience:  jwt.ClaimStrings{"com.carlosbk.hum"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
}

func TestApple_ValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	sub, err := v.Verify(signAppleToken(t, key, validClaims(now)))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "001234.abcdef.5678" {
		t.Fatalf("sub = %q", sub)
	}
}

func TestApple_WrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	c := validClaims(now)
	c.Audience = jwt.ClaimStrings{"com.someone.else"}
	if _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected wrong-audience token to be rejected")
	}
}

func TestApple_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	c := validClaims(now)
	c.Issuer = "https://evil.example.com"
	if _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected wrong-issuer token to be rejected")
	}
}

func TestApple_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	c := validClaims(now)
	c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute)) // already expired
	if _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestApple_WrongSigningKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now) // verifier trusts `key`...

	// ...but the token is signed by `other`.
	if _, err := v.Verify(signAppleToken(t, other, validClaims(now))); err == nil {
		t.Fatal("expected token signed by an untrusted key to be rejected")
	}
}
