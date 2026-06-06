package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const appleJWKSURL = "https://appleid.apple.com/auth/keys"

// appleJWKS fetches and caches Apple's JSON Web Key Set (rotating RSA public
// keys) and resolves a token's signing key by `kid`, refreshing on miss/staleness.
type appleJWKS struct {
	url     string
	client  *http.Client
	ttl     time.Duration
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func newAppleJWKS(httpTimeout time.Duration) *appleJWKS {
	if httpTimeout <= 0 {
		httpTimeout = 10 * time.Second
	}
	return &appleJWKS{
		url:    appleJWKSURL,
		client: &http.Client{Timeout: httpTimeout},
		ttl:    time.Hour,
		keys:   map[string]*rsa.PublicKey{},
	}
}

func (j *appleJWKS) keyfunc() jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("auth: token missing kid")
		}
		if k := j.cached(kid); k != nil {
			return k, nil
		}
		if err := j.refresh(); err != nil {
			return nil, err
		}
		if k := j.cached(kid); k != nil {
			return k, nil
		}
		return nil, errors.New("auth: unknown apple signing key")
	}
}

// cached returns the key for kid, or nil if absent or the cache is stale.
func (j *appleJWKS) cached(kid string) *rsa.PublicKey {
	j.mu.Lock()
	defer j.mu.Unlock()
	if time.Since(j.fetched) > j.ttl {
		return nil
	}
	return j.keys[kid]
}

func (j *appleJWKS) refresh() error {
	resp, err := j.client.Get(j.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("auth: apple JWKS fetch failed: " + resp.Status)
	}
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		e := 0
		for _, b := range eb {
			e = e<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
	}
	j.mu.Lock()
	j.keys = keys
	j.fetched = time.Now()
	j.mu.Unlock()
	return nil
}
