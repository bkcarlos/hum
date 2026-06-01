package applemusic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// writeTempP8 generates a P-256 key and writes it as a PKCS#8 PEM (.p8).
func writeTempP8(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	path := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write p8: %v", err)
	}
	return key, path
}

func TestTokenManager_MintsVerifiableES256(t *testing.T) {
	key, path := writeTempP8(t)

	tm, err := NewTokenManager("TEAMID1234", "KEYID56789", path, time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	tokStr, err := tm.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	parsed, err := jwt.Parse(tokStr, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodECDSA); !ok {
			t.Fatalf("unexpected signing method: %v", tok.Header["alg"])
		}
		return &key.PublicKey, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token did not verify: %v", err)
	}
	if got := parsed.Header["kid"]; got != "KEYID56789" {
		t.Errorf("kid = %v, want KEYID56789", got)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["iss"] != "TEAMID1234" {
		t.Errorf("iss = %v, want TEAMID1234", claims["iss"])
	}
}

func TestTokenManager_CachesUntilNearExpiry(t *testing.T) {
	_, path := writeTempP8(t)
	// TTL must exceed the 1h refresh buffer for caching to apply (real Apple
	// tokens live ~180 days; ECDSA signing is randomized so re-mints differ).
	tm, err := NewTokenManager("TEAM", "KID", path, 48*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	a, _ := tm.Token()
	b, _ := tm.Token()
	if a != b {
		t.Errorf("expected cached token to be reused")
	}
}
