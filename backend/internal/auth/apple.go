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
//
// Audiences is the set of accepted `aud` values: the native iOS app's bundle id
// AND (optionally) the web Services ID used by Sign in with Apple JS — they are
// different client ids but, within one Apple Developer team, resolve to the same
// `sub` per user, so web + iOS free-tier quota is unified. The token's aud must
// match ANY one of them (OR), so we can't use jwt.WithAudience (which is AND when
// repeated) — we check membership manually after parsing.
type AppleVerifier struct {
	Audiences []string // accepted `aud` values (iOS bundle id [+ web Services ID])
	KeyFunc   jwt.Keyfunc
	Now       func() time.Time
}

// NewAppleVerifier builds a verifier backed by Apple's live JWKS endpoint.
func NewAppleVerifier(audiences []string, httpTimeout time.Duration) *AppleVerifier {
	return &AppleVerifier{Audiences: audiences, KeyFunc: newAppleJWKS(httpTimeout).keyfunc()}
}

// appleIDClaims adds the `email` claim (present when the email scope is granted)
// to the standard registered claims.
type appleIDClaims struct {
	Email string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// Verify checks the token's signature against Apple's keys and validates issuer
// (apple) and expiry, confirms the audience is one we accept, then returns the
// stable `sub` and the `email` claim (empty if the email scope wasn't granted).
func (v *AppleVerifier) Verify(identityToken string) (sub, email string, err error) {
	if len(v.Audiences) == 0 {
		return "", "", errors.New("auth: no apple audiences configured")
	}
	timeFunc := time.Now
	if v.Now != nil {
		timeFunc = v.Now
	}
	claims := &appleIDClaims{}
	// No jwt.WithAudience here — it would require the token to carry EVERY listed
	// aud (AND). We accept any one of them, checked below.
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithTimeFunc(timeFunc),
	)
	if _, err := parser.ParseWithClaims(identityToken, claims, v.KeyFunc); err != nil {
		return "", "", err
	}
	if claims.Subject == "" {
		return "", "", errors.New("auth: apple token missing sub")
	}
	if !audienceAllowed(claims.Audience, v.Audiences) {
		return "", "", errors.New("auth: apple token audience not accepted")
	}
	return claims.Subject, claims.Email, nil
}

// audienceAllowed reports whether the token's aud claim contains any accepted aud.
func audienceAllowed(tokenAud jwt.ClaimStrings, allowed []string) bool {
	for _, a := range tokenAud {
		for _, w := range allowed {
			if a == w {
				return true
			}
		}
	}
	return false
}
