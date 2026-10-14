package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zarbanio/bundler/pkg/store"
)

type locker struct{ s *Store }

var unlockScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
else
  return 0
end
`)

type redisLock struct {
	rdb    goredis.UniversalClient
	key    string
	token  string
	once   sync.Once
	unlock error
}

func (l *locker) lockKey(name string) string { return l.s.cfg.Key("lock", name) }

// TryLock attempts to acquire the named lock without waiting.
func (l *locker) TryLock(ctx context.Context, name string, ttl time.Duration) (store.Lock, bool, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	key := l.lockKey(name)
	ok, err := l.s.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	if !ok {
		return nil, false, nil
	}
	return &redisLock{rdb: l.s.rdb, key: key, token: token}, true, nil
}

// Lock blocks until the named lock is acquired or ctx is done.
func (l *locker) Lock(ctx context.Context, name string, ttl time.Duration) (store.Lock, error) {
	for {
		lk, ok, err := l.TryLock(ctx, name, ttl)
		if err != nil {
			return nil, err
		}
		if ok {
			return lk, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// Unlock releases the lock only if this holder still owns it, so an expired
// lock re-acquired by another replica is never deleted by mistake.
func (r *redisLock) Unlock(ctx context.Context) error {
	r.once.Do(func() {
		if _, err := unlockScript.Run(ctx, r.rdb, []string{r.key}, r.token).Result(); err != nil {
			r.unlock = fmt.Errorf("%w: %v", store.ErrUnavailable, err)
		}
	})
	return r.unlock
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store/redis: generate lock token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
