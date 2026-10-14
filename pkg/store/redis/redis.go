// Package redis implements store.Store on top of Redis or Valkey. Both speak
// the same wire protocol, so a single implementation serves either.
//
// Every key is namespaced by deployment and chain (see store.Config) and, for
// mempool keys, tagged with the EntryPoint ("{<ep>}") so that a Redis Cluster
// routes all keys touched by one atomic script to the same slot.
package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Store is a Redis/Valkey-backed store.Store.
type Store struct {
	cfg store.Config
	rdb goredis.UniversalClient
}

// New connects to the configured Redis/Valkey deployment. It does not verify
// connectivity; call Ping for that.
func New(cfg store.Config) (*Store, error) {
	rdb, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Store{cfg: cfg, rdb: rdb}, nil
}

func newClient(cfg store.Config) (goredis.UniversalClient, error) {
	var tlsCfg *tls.Config
	if cfg.TLS != nil {
		var err error
		tlsCfg, err = buildTLS(cfg.TLS)
		if err != nil {
			return nil, err
		}
	}

	switch {
	case cfg.URL != "":
		parsed, err := goredis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("store/redis: parse url: %w", err)
		}
		base := &goredis.Options{
			Addr:         parsed.Addr,
			Username:     parsed.Username,
			Password:     parsed.Password,
			DB:           parsed.DB,
			PoolSize:     cfg.PoolSize,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			MaxRetries:   cfg.MaxRetries,
		}
		if parsed.TLSConfig != nil {
			base.TLSConfig = parsed.TLSConfig
		}
		if tlsCfg != nil {
			base.TLSConfig = tlsCfg
		}
		return goredis.NewClient(base), nil
	case len(cfg.Addrs) == 0:
		return nil, errors.New("store/redis: no url or addrs configured")
	case cfg.MasterName != "":
		return goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.Addrs,
			Username:      cfg.Username,
			Password:      cfg.Password,
			DB:            cfg.DB,
			PoolSize:      cfg.PoolSize,
			DialTimeout:   cfg.DialTimeout,
			ReadTimeout:   cfg.ReadTimeout,
			WriteTimeout:  cfg.WriteTimeout,
			MaxRetries:    cfg.MaxRetries,
			TLSConfig:     tlsCfg,
		}), nil
	case len(cfg.Addrs) > 1:
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:        cfg.Addrs,
			Username:     cfg.Username,
			Password:     cfg.Password,
			PoolSize:     cfg.PoolSize,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			MaxRetries:   cfg.MaxRetries,
			TLSConfig:    tlsCfg,
		}), nil
	default:
		return goredis.NewClient(&goredis.Options{
			Addr:         cfg.Addrs[0],
			Username:     cfg.Username,
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			MaxRetries:   cfg.MaxRetries,
			TLSConfig:    tlsCfg,
		}), nil
	}
}

func buildTLS(cfg *store.TLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // opt-in only
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("store/redis: read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("store/redis: invalid CA file")
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("store/redis: load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

// Mempool returns the mempool view of the store.
func (s *Store) Mempool() store.MempoolStore { return &mempoolStore{s} }

// Reputation returns the reputation view of the store.
func (s *Store) Reputation() store.ReputationStore { return &reputationStore{s} }

// CodeHash returns the code-hash view of the store.
func (s *Store) CodeHash() store.CodeHashStore { return &codeHashStore{s} }

// Locker returns a Redis-backed distributed lock manager.
func (s *Store) Locker() store.Locker { return &locker{s} }

// Ping verifies the backend is reachable.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.rdb.Close() }

// --- key helpers -----------------------------------------------------------

func (s *Store) mempoolBase(ep common.Address) string {
	return s.cfg.Key("mempool", "{"+ep.String()+"}")
}

func (s *Store) opKey(ep common.Address, sender common.Address, nonce string) string {
	return s.mempoolBase(ep) + ":op:" + sender.String() + ":" + nonce
}

func (s *Store) idxKey(ep common.Address, sender common.Address, nonce string) string {
	return s.mempoolBase(ep) + ":idx:" + sender.String() + ":" + nonce
}

func (s *Store) orderKey(ep common.Address) string { return s.mempoolBase(ep) + ":order" }

func (s *Store) seqKey(ep common.Address) string { return s.mempoolBase(ep) + ":seq" }

func (s *Store) entityPrefix(ep common.Address) string { return s.mempoolBase(ep) + ":entity:" }

func (s *Store) entityKey(ep common.Address, entity common.Address) string {
	return s.entityPrefix(ep) + entity.String()
}

func (s *Store) reputationKey(addr common.Address) string {
	return s.cfg.Key("reputation", addr.String())
}

func (s *Store) reputationPattern() string { return s.cfg.Key("reputation") + ":*" }

func (s *Store) codeHashKey(h common.Hash) string {
	return s.cfg.Key("checks", "codeHashes", h.String())
}

func (s *Store) codeHashPattern() string { return s.cfg.Key("checks", "codeHashes") + ":*" }

func ttlSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64(d.Seconds())
}

// --- mempool ---------------------------------------------------------------

var addOpScript = goredis.NewScript(`
local idxKey = KEYS[1]
local opKey = KEYS[2]
local orderKey = KEYS[3]
local seqKey = KEYS[4]
local entityPrefix = ARGV[4]
local n = tonumber(ARGV[3])
local ttl = tonumber(ARGV[2])

local old = redis.call('SMEMBERS', idxKey)
for i = 1, #old do
  redis.call('ZREM', entityPrefix .. old[i], opKey)
end

local seq = redis.call('INCR', seqKey)
redis.call('SET', opKey, ARGV[1])
redis.call('ZADD', orderKey, seq, opKey)
redis.call('DEL', idxKey)
for i = 1, n do
  redis.call('ZADD', KEYS[4 + i], seq, opKey)
  redis.call('SADD', idxKey, ARGV[4 + i])
end
if ttl > 0 then
  redis.call('EXPIRE', opKey, ttl)
  redis.call('EXPIRE', idxKey, ttl)
end
return seq
`)

var removeOpScript = goredis.NewScript(`
local idxKey = KEYS[1]
local opKey = KEYS[2]
local orderKey = KEYS[3]
local entityPrefix = ARGV[1]
local old = redis.call('SMEMBERS', idxKey)
for i = 1, #old do
  redis.call('ZREM', entityPrefix .. old[i], opKey)
end
redis.call('DEL', idxKey)
redis.call('DEL', opKey)
redis.call('ZREM', orderKey, opKey)
return 1
`)

type mempoolStore struct{ s *Store }

func (m *mempoolStore) Add(ctx context.Context, ep common.Address, op *userop.UserOperation) error {
	data, err := op.MarshalJSON()
	if err != nil {
		return err
	}

	entities := []common.Address{op.Sender}
	keys := []string{
		m.s.idxKey(ep, op.Sender, op.Nonce.String()),
		m.s.opKey(ep, op.Sender, op.Nonce.String()),
		m.s.orderKey(ep),
		m.s.seqKey(ep),
		m.s.entityKey(ep, op.Sender),
	}
	args := []any{
		string(data),
		ttlSeconds(m.s.cfg.OpTTL),
		"1",
		m.s.entityPrefix(ep),
		op.Sender.String(),
	}
	for _, addr := range []common.Address{op.GetFactory(), op.GetPaymaster()} {
		if addr == (common.Address{}) {
			continue
		}
		entities = append(entities, addr)
		keys = append(keys, m.s.entityKey(ep, addr))
		args = append(args, addr.String())
	}
	args[2] = strconv.Itoa(len(entities))

	if err := addOpScript.Run(ctx, m.s.rdb, keys, args...).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

func (m *mempoolStore) Remove(ctx context.Context, ep common.Address, ops ...*userop.UserOperation) error {
	for _, op := range ops {
		err := removeOpScript.Run(ctx, m.s.rdb, []string{
			m.s.idxKey(ep, op.Sender, op.Nonce.String()),
			m.s.opKey(ep, op.Sender, op.Nonce.String()),
			m.s.orderKey(ep),
		}, m.s.entityPrefix(ep)).Err()
		if err != nil && !errors.Is(err, goredis.Nil) {
			return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
		}
	}
	return nil
}

func (m *mempoolStore) Ops(ctx context.Context, ep common.Address, entity common.Address) ([]*userop.UserOperation, error) {
	return m.load(ctx, m.s.entityKey(ep, entity))
}

func (m *mempoolStore) All(ctx context.Context, ep common.Address) ([]*userop.UserOperation, error) {
	return m.load(ctx, m.s.orderKey(ep))
}

// load fetches the ordered members of a zset and resolves them to ops,
// opportunistically pruning members whose value key has expired.
func (m *mempoolStore) load(ctx context.Context, zset string) ([]*userop.UserOperation, error) {
	members, err := m.s.rdb.ZRange(ctx, zset, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	if len(members) == 0 {
		return nil, nil
	}

	values, err := m.s.rdb.MGet(ctx, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}

	ops := make([]*userop.UserOperation, 0, len(members))
	stale := make([]string, 0)
	for i, v := range values {
		str, ok := v.(string)
		if !ok {
			stale = append(stale, members[i])
			continue
		}
		op, err := decodeOp([]byte(str))
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	if len(stale) > 0 {
		_ = m.s.rdb.ZRem(ctx, zset, toAny(stale)...).Err()
	}
	return ops, nil
}

func (m *mempoolStore) Clear(ctx context.Context) error {
	return m.s.scanDelete(ctx, m.s.cfg.Key("mempool")+"*")
}

// --- reputation ------------------------------------------------------------

var repOpsScript = goredis.NewScript(`
local now = tonumber(ARGV[1])
local out = {}
for i = 1, #KEYS do
  local seen = tonumber(redis.call('HGET', KEYS[i], 'opsSeen') or '0')
  local included = tonumber(redis.call('HGET', KEYS[i], 'opsIncluded') or '0')
  local updated = tonumber(redis.call('HGET', KEYS[i], 'updatedAt') or now)
  local hours = math.floor((now - updated) / 3600)
  for j = 1, hours do
    if seen < 24 and included < 24 then break end
    seen = seen - math.floor(seen / 24)
    included = included - math.floor(included / 24)
  end
  seen = seen + tonumber(ARGV[2 * i])
  included = included + tonumber(ARGV[2 * i + 1])
  if seen < 0 then seen = 0 end
  if included < 0 then included = 0 end
  redis.call('HSET', KEYS[i], 'opsSeen', seen, 'opsIncluded', included, 'updatedAt', now)
  out[#out + 1] = seen
  out[#out + 1] = included
end
return out
`)

type reputationStore struct{ s *Store }

func (r *reputationStore) decount(ctx context.Context, entity common.Address) (int, int, error) {
	res, err := repOpsScript.Run(ctx, r.s.rdb, []string{r.s.reputationKey(entity)},
		time.Now().Unix(), 0, 0).Slice()
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return intFromLua(res, 0), intFromLua(res, 1), nil
}

func (r *reputationStore) Status(ctx context.Context, entity common.Address, params store.ReputationParams) (store.ReputationStatus, error) {
	seen, included, err := r.decount(ctx, entity)
	if err != nil {
		return store.StatusOK, err
	}
	return store.ComputeStatus(seen, included, params), nil
}

func (r *reputationStore) OpsCount(ctx context.Context, entity common.Address) (int, int, error) {
	return r.decount(ctx, entity)
}

func (r *reputationStore) Apply(ctx context.Context, deltas ...store.ReputationDelta) error {
	agg := store.AggregateDeltas(deltas)
	if len(agg) == 0 {
		return nil
	}
	keys := make([]string, 0, len(agg))
	args := []any{time.Now().Unix()}
	for _, d := range agg {
		keys = append(keys, r.s.reputationKey(d.Address))
		args = append(args, d.OpsSeen, d.OpsIncluded)
	}
	if err := repOpsScript.Run(ctx, r.s.rdb, keys, args...).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

func (r *reputationStore) Override(ctx context.Context, entries []store.ReputationEntry) error {
	pipe := r.s.rdb.Pipeline()
	now := time.Now().Unix()
	for _, e := range entries {
		pipe.HSet(ctx, r.s.reputationKey(e.Address),
			"opsSeen", e.OpsSeen, "opsIncluded", e.OpsIncluded, "updatedAt", now)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

func (r *reputationStore) Dump(ctx context.Context) ([]store.ReputationEntry, error) {
	keys, err := r.s.scanKeys(ctx, r.s.reputationPattern())
	if err != nil {
		return nil, err
	}
	out := make([]store.ReputationEntry, 0, len(keys))
	pipe := r.s.rdb.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.HGetAll(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, goredis.Nil) {
		return nil, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	now := time.Now().Unix()
	for i, k := range keys {
		vals := cmds[i].Val()
		seen, _ := strconv.Atoi(vals["opsSeen"])
		included, _ := strconv.Atoi(vals["opsIncluded"])
		updated, _ := strconv.ParseInt(vals["updatedAt"], 10, 64)
		if updated == 0 {
			updated = now
		}
		c := store.OpsCount{Seen: seen, Included: included, UpdatedAt: time.Unix(updated, 0)}
		c = c.Decayed(time.Unix(now, 0))
		parts := strings.Split(k, ":")
		out = append(out, store.ReputationEntry{
			Address:     common.HexToAddress(parts[len(parts)-1]),
			OpsSeen:     c.Seen,
			OpsIncluded: c.Included,
		})
	}
	return out, nil
}

func (r *reputationStore) Clear(ctx context.Context) error {
	return r.s.scanDelete(ctx, r.s.reputationPattern())
}

// --- code hashes -----------------------------------------------------------

type codeHashStore struct{ s *Store }

func (c *codeHashStore) Save(ctx context.Context, userOpHash common.Hash, hashes []store.CodeHash) error {
	raw, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	if err := c.s.rdb.Set(ctx, c.s.codeHashKey(userOpHash), raw, 0).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

func (c *codeHashStore) Get(ctx context.Context, userOpHash common.Hash) ([]store.CodeHash, error) {
	raw, err := c.s.rdb.Get(ctx, c.s.codeHashKey(userOpHash)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	var hashes []store.CodeHash
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}

func (c *codeHashStore) Remove(ctx context.Context, userOpHashes ...common.Hash) error {
	keys := make([]string, 0, len(userOpHashes))
	for _, h := range userOpHashes {
		keys = append(keys, c.s.codeHashKey(h))
	}
	return c.s.deleteKeys(ctx, keys)
}

func (c *codeHashStore) Clear(ctx context.Context) error {
	return c.s.scanDelete(ctx, c.s.codeHashPattern())
}

// --- shared helpers --------------------------------------------------------

func (s *Store) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := s.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return keys, nil
}

func (s *Store) scanDelete(ctx context.Context, pattern string) error {
	keys, err := s.scanKeys(ctx, pattern)
	if err != nil {
		return err
	}
	return s.deleteKeys(ctx, keys)
}

// deleteKeys removes keys one at a time through a pipeline so it stays correct
// under Redis Cluster, where a multi-key DEL may span slots.
func (s *Store) deleteKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	pipe := s.rdb.Pipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

func decodeOp(value []byte) (*userop.UserOperation, error) {
	data := make(map[string]any)
	if err := json.Unmarshal(value, &data); err != nil {
		return nil, err
	}
	return userop.New(data)
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func intFromLua(res []any, i int) int {
	if i >= len(res) {
		return 0
	}
	if v, ok := res[i].(int64); ok {
		return int(v)
	}
	return 0
}
