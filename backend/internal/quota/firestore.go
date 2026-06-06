package quota

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Firestore layout (all top-level collections):
//
//	humQuota/config            → Config (live policy + admin sub allowlist; console-editable)
//	humQuotaGlobal/{day}       → { count }   global daily counter
//	humQuotaUser/{day}__{sub}  → { count }   per-user daily counter
//	humQuotaBans/{sub}         → { banned }  ban flag (absent = not banned)
const (
	fsColMeta        = "humQuota"
	fsDocConfig      = "config"
	fsColGlobal      = "humQuotaGlobal"
	fsColUser        = "humQuotaUser"
	fsColBans        = "humQuotaBans"
	fsConfigCacheTTL = 30 * time.Second // live config, but don't read it every request
)

type fsCounter struct {
	Count int `firestore:"count"`
}
type fsBan struct {
	Banned bool `firestore:"banned"`
}

// FirestoreStore is the durable, cross-instance quota Store for production.
// Reserve/Refund run in Firestore transactions so concurrent Cloud Run instances
// can't overshoot a cap. GetConfig is cached briefly so a per-request read of the
// live policy doesn't hit Firestore every time.
type FirestoreStore struct {
	client *firestore.Client

	mu        sync.Mutex
	cachedCfg Config
	cachedAt  time.Time
	haveCache bool
}

// NewFirestoreStore connects via Application Default Credentials (set on Cloud
// Run automatically) and seeds the config doc with `seed` if it doesn't exist yet
// (an existing, admin-edited doc is left untouched).
func NewFirestoreStore(ctx context.Context, projectID string, seed Config) (*FirestoreStore, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ref := client.Collection(fsColMeta).Doc(fsDocConfig)
	if _, err := ref.Get(ctx); status.Code(err) == codes.NotFound {
		if _, err := ref.Set(ctx, seed); err != nil {
			_ = client.Close()
			return nil, err
		}
	} else if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &FirestoreStore{client: client}, nil
}

func (s *FirestoreStore) Close() error { return s.client.Close() }

func (s *FirestoreStore) GetConfig(ctx context.Context) (Config, error) {
	s.mu.Lock()
	if s.haveCache && time.Since(s.cachedAt) < fsConfigCacheTTL {
		c := s.cachedCfg
		s.mu.Unlock()
		return c, nil
	}
	s.mu.Unlock()

	snap, err := s.client.Collection(fsColMeta).Doc(fsDocConfig).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}
	var c Config
	if err := snap.DataTo(&c); err != nil {
		return Config{}, err
	}
	s.cache(c)
	return c, nil
}

func (s *FirestoreStore) SetConfig(ctx context.Context, c Config) error {
	if _, err := s.client.Collection(fsColMeta).Doc(fsDocConfig).Set(ctx, c); err != nil {
		return err
	}
	s.cache(c)
	return nil
}

func (s *FirestoreStore) cache(c Config) {
	s.mu.Lock()
	s.cachedCfg, s.cachedAt, s.haveCache = c, time.Now(), true
	s.mu.Unlock()
}

func (s *FirestoreStore) Reserve(ctx context.Context, sub, day string, cfg Config) (Decision, error) {
	d := Decision{UserLimit: cfg.PerUserDailyLimit, GlobalLimit: cfg.GlobalDailyLimit}
	globalRef := s.client.Collection(fsColGlobal).Doc(day)
	userRef := s.client.Collection(fsColUser).Doc(day + "__" + sub)
	banRef := s.client.Collection(fsColBans).Doc(sub)

	err := s.client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		// All reads must precede writes inside a Firestore transaction.
		gUsed, err := txCount(tx, globalRef)
		if err != nil {
			return err
		}
		uUsed, err := txCount(tx, userRef)
		if err != nil {
			return err
		}
		banned, err := txBanned(tx, banRef)
		if err != nil {
			return err
		}

		d.GlobalUsed, d.UserUsed = gUsed, uUsed
		switch {
		case !cfg.Enabled:
			d.Reason = ReasonDisabled
		case banned:
			d.Reason = ReasonBanned
		case cfg.GlobalDailyLimit > 0 && gUsed >= cfg.GlobalDailyLimit:
			d.Reason = ReasonGlobal
		case cfg.PerUserDailyLimit > 0 && uUsed >= cfg.PerUserDailyLimit:
			d.Reason = ReasonUser
		default:
			if err := tx.Set(globalRef, fsCounter{Count: gUsed + 1}); err != nil {
				return err
			}
			if err := tx.Set(userRef, fsCounter{Count: uUsed + 1}); err != nil {
				return err
			}
			d.GlobalUsed, d.UserUsed = gUsed+1, uUsed+1
			d.Allowed = true
		}
		return nil
	})
	if err != nil {
		return Decision{}, err
	}
	return d, nil
}

func (s *FirestoreStore) Refund(ctx context.Context, sub, day string) error {
	globalRef := s.client.Collection(fsColGlobal).Doc(day)
	userRef := s.client.Collection(fsColUser).Doc(day + "__" + sub)
	return s.client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		gUsed, err := txCount(tx, globalRef)
		if err != nil {
			return err
		}
		uUsed, err := txCount(tx, userRef)
		if err != nil {
			return err
		}
		if gUsed > 0 {
			if err := tx.Set(globalRef, fsCounter{Count: gUsed - 1}); err != nil {
				return err
			}
		}
		if uUsed > 0 {
			if err := tx.Set(userRef, fsCounter{Count: uUsed - 1}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *FirestoreStore) GetUserUsage(ctx context.Context, sub, day string) (int, error) {
	return docCount(ctx, s.client.Collection(fsColUser).Doc(day+"__"+sub))
}

func (s *FirestoreStore) GetGlobalUsage(ctx context.Context, day string) (int, error) {
	return docCount(ctx, s.client.Collection(fsColGlobal).Doc(day))
}

func (s *FirestoreStore) IsBanned(ctx context.Context, sub string) (bool, error) {
	snap, err := s.client.Collection(fsColBans).Doc(sub).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var b fsBan
	if err := snap.DataTo(&b); err != nil {
		return false, err
	}
	return b.Banned, nil
}

func (s *FirestoreStore) SetBanned(ctx context.Context, sub string, banned bool) error {
	ref := s.client.Collection(fsColBans).Doc(sub)
	if !banned {
		_, err := ref.Delete(ctx)
		return err
	}
	_, err := ref.Set(ctx, fsBan{Banned: true})
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────

func docCount(ctx context.Context, ref *firestore.DocumentRef) (int, error) {
	snap, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var c fsCounter
	if err := snap.DataTo(&c); err != nil {
		return 0, err
	}
	return c.Count, nil
}

func txCount(tx *firestore.Transaction, ref *firestore.DocumentRef) (int, error) {
	snap, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var c fsCounter
	if err := snap.DataTo(&c); err != nil {
		return 0, err
	}
	return c.Count, nil
}

func txBanned(tx *firestore.Transaction, ref *firestore.DocumentRef) (bool, error) {
	snap, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var b fsBan
	if err := snap.DataTo(&b); err != nil {
		return false, err
	}
	return b.Banned, nil
}

// compile-time check: FirestoreStore implements Store.
var _ Store = (*FirestoreStore)(nil)
