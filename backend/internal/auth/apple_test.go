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
		Audiences: []string{"com.carlosbk.hum"},
		Now:       func() time.Time { return now },
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

	sub, _, err := v.Verify(signAppleToken(t, key, validClaims(now)))
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
	if _, _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected wrong-audience token to be rejected")
	}
}

func TestApple_MultiAudienceAcceptsWebServicesID(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)
	// Accept both the iOS bundle id and the web Services ID (OR-match).
	v.Audiences = []string{"com.carlosbk.hum", "com.carlosbk.hum.web"}

	// A web token (aud = Services ID) is accepted and yields the same sub shape.
	c := validClaims(now)
	c.Audience = jwt.ClaimStrings{"com.carlosbk.hum.web"}
	if sub, _, err := v.Verify(signAppleToken(t, key, c)); err != nil || sub != "001234.abcdef.5678" {
		t.Fatalf("web aud: sub=%q err=%v", sub, err)
	}
	// The iOS token (aud = bundle id) still works.
	if _, _, err := v.Verify(signAppleToken(t, key, validClaims(now))); err != nil {
		t.Fatalf("ios aud: %v", err)
	}
	// An aud in neither set is still rejected.
	c2 := validClaims(now)
	c2.Audience = jwt.ClaimStrings{"com.someone.else"}
	if _, _, err := v.Verify(signAppleToken(t, key, c2)); err == nil {
		t.Fatal("expected an unlisted audience to be rejected")
	}
}

func TestApple_ReturnsEmail(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, appleIDClaims{Email: "user@example.com", RegisteredClaims: validClaims(now)})
	tok.Header["kid"] = "testkid"
	signed, _ := tok.SignedString(key)
	sub, email, err := v.Verify(signed)
	if err != nil || sub != "001234.abcdef.5678" || email != "user@example.com" {
		t.Fatalf("sub=%q email=%q err=%v", sub, email, err)
	}
}

func TestApple_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	c := validClaims(now)
	c.Issuer = "https://evil.example.com"
	if _, _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected wrong-issuer token to be rejected")
	}
}

func TestApple_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now)

	c := validClaims(now)
	c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute)) // already expired
	if _, _, err := v.Verify(signAppleToken(t, key, c)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestApple_WrongSigningKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	v := testVerifier(t, key, now) // verifier trusts `key`...

	// ...but the token is signed by `other`.
	if _, _, err := v.Verify(signAppleToken(t, other, validClaims(now))); err == nil {
		t.Fatal("expected token signed by an untrusted key to be rejected")
	}
}
