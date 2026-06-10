package llm

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	for _, u := range []string{"http://api.openai.com/v1", "https://x", "https://gw.example.com:8443/v1"} {
		if err := validateBaseURL(u); err != nil {
			t.Errorf("validateBaseURL(%q) = %v, want nil", u, err)
		}
	}
	for _, u := range []string{"file:///etc/passwd", "ftp://x", "gopher://x", "://nohost", "not a url", ""} {
		if err := validateBaseURL(u); err == nil {
			t.Errorf("validateBaseURL(%q) = nil, want error", u)
		}
	}
}

func TestBlockedIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "::1", "0.0.0.0", "fe80::1"} {
		if !blockedIP(net.ParseIP(s)) {
			t.Errorf("blockedIP(%s) = false, want true (internal)", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700:4700::1111"} {
		if blockedIP(net.ParseIP(s)) {
			t.Errorf("blockedIP(%s) = true, want false (public)", s)
		}
	}
}

// With the guard ENABLED, a provider pointed at a loopback address must fail to
// connect — proving the block fires at dial time (not just on literal-IP parsing).
func TestSSRFGuard_BlocksLoopbackDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, openAIResp(intentJSON))
	}))
	defer srv.Close()

	DisableSSRFGuard = false // re-enable the guard the package TestMain turned off
	defer func() { DisableSSRFGuard = true }()

	p := newOpenAICompat(cfg(ProviderOpenAICompat, srv.URL)) // srv.URL is 127.0.0.1
	if _, err := p.ParseIntent(context.Background(), "x", nil); err == nil {
		t.Fatal("expected the SSRF guard to block a loopback dial, got nil error")
	}
}
