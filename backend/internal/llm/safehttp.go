package llm

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// DisableSSRFGuard relaxes the private-address block below. It exists ONLY so
// tests can point adapters at httptest servers on 127.0.0.1; production must
// leave it false. (BaseURL is user/attacker-supplied via the BYOK config, so the
// guard is what keeps these outbound calls from reaching internal targets.)
var DisableSSRFGuard = false

// blockedIP reports whether dialing ip would reach a non-public/internal target.
// Loopback, RFC1918 private, link-local (incl. the cloud metadata 169.254.169.254),
// unspecified, and multicast are all refused.
func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// validateBaseURL rejects a BaseURL that isn't a plain http(s) URL with a host,
// so schemes like file://, gopher://, or a bare string fail fast with a clean
// error (the IP-level block is enforced later, at dial time, by safeHTTPClient).
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("llm: 无效的 Base URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("llm: Base URL 必须是 http(s)")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("llm: Base URL 缺少主机名")
	}
	return nil
}

// safeHTTPClient builds an http.Client whose dialer refuses connections to
// internal addresses. The check runs in Dialer.Control — i.e. AFTER DNS
// resolution, against the concrete IP about to be dialed — so it also defeats
// DNS-rebinding and redirects that resolve to an internal host. Used for every
// BYOK + free-tier LLM call.
func safeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if DisableSSRFGuard {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || blockedIP(ip) {
				return fmt.Errorf("llm: 拒绝连接到非公网地址 %q", address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
		},
	}
}
