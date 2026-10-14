// Package memory implements an ephemeral, process-local store. It is intended
// for tests and single-process development; nothing is shared between
// replicas and everything is lost on restart.
package memory

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/internal/opqueue"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Store is an in-memory implementation of store.Store.
type Store struct {
	cfg store.Config

	mu    sync.RWMutex
	queue *opqueue.Queues
	rep   map[common.Address]store.OpsCount
	codes map[common.Hash][]store.CodeHash
}

// New returns an empty in-memory store.
func New(cfg store.Config) *Store {
	return &Store{
		cfg:   cfg,
		queue: opqueue.New(),
		rep:   make(map[common.Address]store.OpsCount),
		codes: make(map[common.Hash][]store.CodeHash),
	}
}

// Mempool returns the mempool view of the store.
func (s *Store) Mempool() store.MempoolStore { return &mempoolStore{s} }

// Reputation returns the reputation view of the store.
func (s *Store) Reputation() store.ReputationStore { return &reputationStore{s} }

// CodeHash returns the code-hash view of the store.
func (s *Store) CodeHash() store.CodeHashStore { return &codeHashStore{s} }

// Locker returns a no-op locker: this backend is never shared across replicas.
func (s *Store) Locker() store.Locker { return store.NoopLocker{} }

// Ping always succeeds: the store lives in the process.
func (s *Store) Ping(context.Context) error { return nil }

// Close is a no-op.
func (s *Store) Close() error { return nil }

type mempoolStore struct{ s *Store }

func (m *mempoolStore) Add(_ context.Context, ep common.Address, op *userop.UserOperation) error {
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	m.s.queue.Add(ep, op)
	return nil
}

func (m *mempoolStore) Remove(_ context.Context, ep common.Address, ops ...*userop.UserOperation) error {
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	m.s.queue.Remove(ep, ops...)
	return nil
}

func (m *mempoolStore) Ops(_ context.Context, ep common.Address, entity common.Address) ([]*userop.UserOperation, error) {
	m.s.mu.RLock()
	defer m.s.mu.RUnlock()
	return m.s.queue.Ops(ep, entity), nil
}

func (m *mempoolStore) All(_ context.Context, ep common.Address) ([]*userop.UserOperation, error) {
	m.s.mu.RLock()
	defer m.s.mu.RUnlock()
	return m.s.queue.All(ep), nil
}

func (m *mempoolStore) Clear(context.Context) error {
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	m.s.queue.Reset()
	return nil
}

type reputationStore struct{ s *Store }

// getLocked returns the decayed counters without persisting them: reads must
// not create or mutate entries, so an entity that is only checked keeps no
// state and repeated reads do not reset the decay clock.
func (r *reputationStore) getLocked(addr common.Address, now time.Time) store.OpsCount {
	return r.s.rep[addr].Decayed(now)
}

func (r *reputationStore) Status(_ context.Context, entity common.Address, params store.ReputationParams) (store.ReputationStatus, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c := r.getLocked(entity, time.Now())
	return store.ComputeStatus(c.Seen, c.Included, params), nil
}

func (r *reputationStore) OpsCount(_ context.Context, entity common.Address) (int, int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c := r.getLocked(entity, time.Now())
	return c.Seen, c.Included, nil
}

func (r *reputationStore) Apply(_ context.Context, deltas ...store.ReputationDelta) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	now := time.Now()
	for _, d := range store.AggregateDeltas(deltas) {
		r.s.rep[d.Address] = r.s.rep[d.Address].ApplyDelta(d, now)
	}
	return nil
}

func (r *reputationStore) Override(_ context.Context, entries []store.ReputationEntry) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	now := time.Now()
	for _, e := range entries {
		r.s.rep[e.Address] = store.OpsCount{Seen: e.OpsSeen, Included: e.OpsIncluded, UpdatedAt: now}
	}
	return nil
}

func (r *reputationStore) Dump(_ context.Context) ([]store.ReputationEntry, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := make([]store.ReputationEntry, 0, len(r.s.rep))
	for addr := range r.s.rep {
		c := r.getLocked(addr, time.Now())
		out = append(out, store.ReputationEntry{Address: addr, OpsSeen: c.Seen, OpsIncluded: c.Included})
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Address[:], out[j].Address[:]) < 0
	})
	return out, nil
}

func (r *reputationStore) Clear(context.Context) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.rep = make(map[common.Address]store.OpsCount)
	return nil
}

type codeHashStore struct{ s *Store }

func (c *codeHashStore) Save(_ context.Context, userOpHash common.Hash, hashes []store.CodeHash) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.codes[userOpHash] = append([]store.CodeHash(nil), hashes...)
	return nil
}

func (c *codeHashStore) Get(_ context.Context, userOpHash common.Hash) ([]store.CodeHash, error) {
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	hashes, ok := c.s.codes[userOpHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return append([]store.CodeHash(nil), hashes...), nil
}

func (c *codeHashStore) Remove(_ context.Context, userOpHashes ...common.Hash) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	for _, h := range userOpHashes {
		delete(c.s.codes, h)
	}
	return nil
}

func (c *codeHashStore) Clear(context.Context) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.codes = make(map[common.Hash][]store.CodeHash)
	return nil
}
