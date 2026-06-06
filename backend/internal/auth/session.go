// Package auth verifies Sign in with Apple identity tokens and issues/validates
// the server's own session tokens. The Apple `sub` (a stable, app-scoped user id)
// becomes the quota key; it never leaves the server except inside our session JWT.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const sessionIssuer = "hum"

// IssueSession mints an HS256 session token carrying the Apple user id (sub).
// The client sends it as `Authorization: Bearer <token>` on free-tier requests.
func IssueSession(secret []byte, sub string, ttl time.Duration, now time.Time) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("auth: empty session secret")
	}
	if sub == "" {
		return "", errors.New("auth: empty subject")
	}
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		Issuer:    sessionIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// VerifySession validates a session token and returns its subject (Apple sub).
// It pins the algorithm to HS256 (no alg-confusion) and the issuer to ours.
func VerifySession(secret []byte, token string, now time.Time) (string, error) {
	claims := &jwt.RegisteredClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(sessionIssuer),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if _, err := parser.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return secret, nil
	}); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("auth: session missing subject")
	}
	return claims.Subject, nil
}
