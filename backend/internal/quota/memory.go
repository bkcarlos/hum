package quota

import (
	"context"
	"sync"
)

// MemoryStore is an in-process Store for local dev and tests. It is correct on a
// single instance but does NOT share state across Cloud Run instances or survive
// a restart — production uses the Firestore store. Safe for concurrent use.
type MemoryStore struct {
	mu     sync.Mutex
	cfg    Config
	user   map[string]int  // key: day + "|" + sub
	global map[string]int  // key: day
	banned map[string]bool // key: sub
}

// NewMemoryStore seeds the store with an initial config.
func NewMemoryStore(initial Config) *MemoryStore {
	return &MemoryStore{
		cfg:    initial,
		user:   map[string]int{},
		global: map[string]int{},
		banned: map[string]bool{},
	}
}

func (m *MemoryStore) GetConfig(_ context.Context) (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg, nil
}

func (m *MemoryStore) SetConfig(_ context.Context, c Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = c
	return nil
}

func (m *MemoryStore) Reserve(_ context.Context, sub, day string, cfg Config) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d := Decision{UserLimit: cfg.PerUserDailyLimit, GlobalLimit: cfg.GlobalDailyLimit}
	uk := day + "|" + sub
	d.UserUsed = m.user[uk]
	d.GlobalUsed = m.global[day]

	switch {
	case !cfg.Enabled:
		d.Reason = ReasonDisabled
	case m.banned[sub]:
		d.Reason = ReasonBanned
	case cfg.GlobalDailyLimit > 0 && d.GlobalUsed >= cfg.GlobalDailyLimit:
		d.Reason = ReasonGlobal
	case cfg.PerUserDailyLimit > 0 && d.UserUsed >= cfg.PerUserDailyLimit:
		d.Reason = ReasonUser
	default:
		m.user[uk] = d.UserUsed + 1
		m.global[day] = d.GlobalUsed + 1
		d.UserUsed++
		d.GlobalUsed++
		d.Allowed = true
	}
	return d, nil
}

func (m *MemoryStore) Refund(_ context.Context, sub, day string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	uk := day + "|" + sub
	if m.user[uk] > 0 {
		m.user[uk]--
	}
	if m.global[day] > 0 {
		m.global[day]--
	}
	return nil
}

func (m *MemoryStore) GetUserUsage(_ context.Context, sub, day string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.user[day+"|"+sub], nil
}

func (m *MemoryStore) GetGlobalUsage(_ context.Context, day string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.global[day], nil
}

func (m *MemoryStore) IsBanned(_ context.Context, sub string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.banned[sub], nil
}

func (m *MemoryStore) SetBanned(_ context.Context, sub string, banned bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if banned {
		m.banned[sub] = true
	} else {
		delete(m.banned, sub)
	}
	return nil
}
