package auth

import (
	"testing"
	"time"
)

func TestSession_RoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	tok, err := IssueSession(secret, "apple-sub-123", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := VerifySession(secret, tok, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "apple-sub-123" {
		t.Fatalf("sub = %q, want apple-sub-123", sub)
	}
}

func TestSession_Expired(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	tok, _ := IssueSession(secret, "x", time.Hour, now)
	if _, err := VerifySession(secret, tok, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestSession_WrongSecret(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	tok, _ := IssueSession([]byte("secret-a"), "x", time.Hour, now)
	if _, err := VerifySession([]byte("secret-b"), tok, now); err == nil {
		t.Fatal("expected wrong-secret verification to fail")
	}
}

func TestSession_Tampered(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	tok, _ := IssueSession(secret, "x", time.Hour, now)
	if _, err := VerifySession(secret, tok+"x", now); err == nil {
		t.Fatal("expected tampered token to fail")
	}
}

func TestSession_EmptySecretErrors(t *testing.T) {
	if _, err := IssueSession(nil, "x", time.Hour, time.Now()); err == nil {
		t.Fatal("expected empty secret to error")
	}
}
