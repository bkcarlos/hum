// Package applemusic mints the Developer Token and talks to the Apple Music
// HTTP API (catalog search + library playlist creation).
//
// Two-token model (docs/requirements.md §5.3):
//   - Developer Token  — minted here from the .p8 private key (ES256 JWT). The
//     private key NEVER leaves the backend.
//   - Music User Token — obtained client-side via MusicKit JS, passed per request
//     in the Music-User-Token header; never persisted server-side.
package applemusic

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager signs and caches the Apple Music Developer Token. Minting is
// cheap but we cache and reuse until shortly before expiry.
type TokenManager struct {
	teamID     string
	keyID      string
	privateKey *ecdsa.PrivateKey
	ttl        time.Duration

	mu     sync.Mutex
	cached string
	exp    time.Time
}

// NewTokenManager loads the ES256 private key from a .p8 (PKCS#8 PEM) file.
func NewTokenManager(teamID, keyID, privateKeyPath string, ttl time.Duration) (*TokenManager, error) {
	if privateKeyPath == "" {
		return nil, fmt.Errorf("applemusic: privateKeyPath is required")
	}
	pemBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("applemusic: reading private key: %w", err)
	}
	return NewTokenManagerFromPEM(teamID, keyID, pemBytes, ttl)
}

// NewTokenManagerFromPEM builds a manager from raw .p8 (PKCS#8 PEM) bytes —
// used when the key is injected via an env var on a cloud host.
func NewTokenManagerFromPEM(teamID, keyID string, pemBytes []byte, ttl time.Duration) (*TokenManager, error) {
	if teamID == "" || keyID == "" {
		return nil, fmt.Errorf("applemusic: teamID and keyID are required")
	}
	key, err := parseP8(pemBytes)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 || ttl > 180*24*time.Hour {
		ttl = 180 * 24 * time.Hour // Apple caps Developer Token lifetime at ~6 months
	}
	return &TokenManager{teamID: teamID, keyID: keyID, privateKey: key, ttl: ttl}, nil
}

// Token returns a valid Developer Token, re-minting when within an hour of expiry.
func (m *TokenManager) Token() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cached != "" && timeNow().Before(m.exp.Add(-time.Hour)) {
		return m.cached, nil
	}
	now := timeNow()
	exp := now.Add(m.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": m.teamID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
	})
	tok.Header["kid"] = m.keyID
	signed, err := tok.SignedString(m.privateKey)
	if err != nil {
		return "", fmt.Errorf("applemusic: signing developer token: %w", err)
	}
	m.cached, m.exp = signed, exp
	return signed, nil
}

// ExpiresAt reports the current cached token's expiry (zero if never minted).
func (m *TokenManager) ExpiresAt() time.Time { return m.exp }

func parseP8(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("applemusic: no PEM block found in private key file")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("applemusic: parsing PKCS#8 key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("applemusic: private key is not ECDSA (expected ES256 .p8)")
	}
	return key, nil
}

// timeNow is a seam for testing; defaults to time.Now.
var timeNow = time.Now
