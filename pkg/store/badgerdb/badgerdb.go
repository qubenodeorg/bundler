// Package badgerdb implements store.Store on top of the embedded BadgerDB. It
// is the default backend for single-instance deployments and preserves the key
// layout used by earlier bundler releases so existing data keeps loading.
package badgerdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/internal/opqueue"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Store is a BadgerDB-backed store.Store.
type Store struct {
	cfg    store.Config
	db     *badger.DB
	queue  *opqueue.Queues
	gcDone chan struct{}
}

// Open opens (or creates) a BadgerDB directory and loads any persisted pending
// ops into the arrival index.
func Open(cfg store.Config) (*Store, error) {
	db, err := badger.Open(badger.DefaultOptions(cfg.DataDirectory))
	if err != nil {
		return nil, fmt.Errorf("store/badger: open %q: %w", cfg.DataDirectory, err)
	}

	s := &Store{
		cfg:    cfg,
		db:     db,
		queue:  opqueue.New(),
		gcDone: make(chan struct{}),
	}
	if err := s.loadMempool(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.startGC()
	return s, nil
}

// Mempool returns the mempool view of the store.
func (s *Store) Mempool() store.MempoolStore { return &mempoolStore{s} }

// Reputation returns the reputation view of the store.
func (s *Store) Reputation() store.ReputationStore { return &reputationStore{s} }

// CodeHash returns the code-hash view of the store.
func (s *Store) CodeHash() store.CodeHashStore { return &codeHashStore{s} }

// Locker returns a no-op locker: BadgerDB is node-local and single-writer.
func (s *Store) Locker() store.Locker { return store.NoopLocker{} }

// Ping confirms the database is open and usable.
func (s *Store) Ping(context.Context) error {
	if err := s.db.View(func(txn *badger.Txn) error { return nil }); err != nil {
		return fmt.Errorf("%w: %v", store.ErrUnavailable, err)
	}
	return nil
}

// Close stops value-log garbage collection and closes the database.
func (s *Store) Close() error {
	close(s.gcDone)
	return s.db.Close()
}

// startGC periodically reclaims space from the append-only value log.
func (s *Store) startGC() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.gcDone:
				return
			case <-ticker.C:
			again:
				err := s.db.RunValueLogGC(0.7)
				if err == nil {
					goto again
				}
			}
		}
	}()
}

func (s *Store) mempoolKey(ep common.Address, sender common.Address, nonce string) []byte {
	return []byte(s.cfg.Key("mempool", ep.String(), sender.String(), nonce))
}

func (s *Store) mempoolPrefix() string {
	return s.cfg.Key("mempool") + ":"
}

// loadMempool rebuilds the in-process arrival index from persisted ops.
func (s *Store) loadMempool() error {
	prefix := []byte(s.mempoolPrefix())
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			rest := strings.TrimPrefix(string(item.Key()), string(prefix))
			ep := common.HexToAddress(strings.SplitN(rest, ":", 2)[0])

			if err := item.Value(func(v []byte) error {
				op, err := decodeOp(v)
				if err != nil {
					return err
				}
				s.queue.Add(ep, op)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func decodeOp(value []byte) (*userop.UserOperation, error) {
	data := make(map[string]any)
	if err := json.Unmarshal(value, &data); err != nil {
		return nil, err
	}
	return userop.New(data)
}

type mempoolStore struct{ s *Store }

func (m *mempoolStore) Add(_ context.Context, ep common.Address, op *userop.UserOperation) error {
	data, err := op.MarshalJSON()
	if err != nil {
		return err
	}
	if err := m.s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(m.s.mempoolKey(ep, op.Sender, op.Nonce.String()), data)
	}); err != nil {
		return err
	}
	m.s.queue.Add(ep, op)
	return nil
}

func (m *mempoolStore) Remove(_ context.Context, ep common.Address, ops ...*userop.UserOperation) error {
	if err := m.s.db.Update(func(txn *badger.Txn) error {
		for _, op := range ops {
			if err := txn.Delete(m.s.mempoolKey(ep, op.Sender, op.Nonce.String())); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	m.s.queue.Remove(ep, ops...)
	return nil
}

func (m *mempoolStore) Ops(_ context.Context, ep common.Address, entity common.Address) ([]*userop.UserOperation, error) {
	return m.s.queue.Ops(ep, entity), nil
}

func (m *mempoolStore) All(_ context.Context, ep common.Address) ([]*userop.UserOperation, error) {
	return m.s.queue.All(ep), nil
}

func (m *mempoolStore) Clear(context.Context) error {
	if err := m.s.db.DropPrefix([]byte(m.s.mempoolPrefix())); err != nil {
		return err
	}
	m.s.queue.Reset()
	return nil
}

func (s *Store) reputationKey(addr common.Address) []byte {
	return []byte(s.cfg.Key("entity", "opsCount", addr.String()))
}

func (s *Store) reputationPrefix() string {
	return s.cfg.Key("entity", "opsCount") + ":"
}

func readOpsCount(txn *badger.Txn, key []byte) (store.OpsCount, error) {
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return store.OpsCount{}, nil
	}
	if err != nil {
		return store.OpsCount{}, err
	}
	var raw []byte
	if err := item.Value(func(v []byte) error {
		raw = append([]byte(nil), v...)
		return nil
	}); err != nil {
		return store.OpsCount{}, err
	}
	return store.DecodeOpsCount(raw)
}

type reputationStore struct{ s *Store }

func (r *reputationStore) Status(_ context.Context, entity common.Address, params store.ReputationParams) (store.ReputationStatus, error) {
	var status store.ReputationStatus
	now := time.Now()
	err := r.s.db.Update(func(txn *badger.Txn) error {
		key := r.s.reputationKey(entity)
		c, err := readOpsCount(txn, key)
		if err != nil {
			return err
		}
		c = c.Decayed(now)
		status = store.ComputeStatus(c.Seen, c.Included, params)
		return txn.Set(key, c.Encode())
	})
	return status, err
}

func (r *reputationStore) OpsCount(_ context.Context, entity common.Address) (int, int, error) {
	var seen, included int
	now := time.Now()
	err := r.s.db.Update(func(txn *badger.Txn) error {
		key := r.s.reputationKey(entity)
		c, err := readOpsCount(txn, key)
		if err != nil {
			return err
		}
		c = c.Decayed(now)
		seen, included = c.Seen, c.Included
		return txn.Set(key, c.Encode())
	})
	return seen, included, err
}

func (r *reputationStore) Apply(_ context.Context, deltas ...store.ReputationDelta) error {
	now := time.Now()
	return r.s.db.Update(func(txn *badger.Txn) error {
		for _, d := range store.AggregateDeltas(deltas) {
			key := r.s.reputationKey(d.Address)
			c, err := readOpsCount(txn, key)
			if err != nil {
				return err
			}
			if err := txn.Set(key, c.ApplyDelta(d, now).Encode()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *reputationStore) Override(_ context.Context, entries []store.ReputationEntry) error {
	now := time.Now()
	return r.s.db.Update(func(txn *badger.Txn) error {
		for _, e := range entries {
			c := store.OpsCount{Seen: e.OpsSeen, Included: e.OpsIncluded, UpdatedAt: now}
			if err := txn.Set(r.s.reputationKey(e.Address), c.Encode()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *reputationStore) Dump(_ context.Context) ([]store.ReputationEntry, error) {
	out := []store.ReputationEntry{}
	prefix := []byte(r.s.reputationPrefix())
	now := time.Now()
	err := r.s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := string(it.Item().Key())
			parts := strings.Split(key, ":")
			addr := common.HexToAddress(parts[len(parts)-1])
			c, err := readOpsCount(txn, []byte(key))
			if err != nil {
				return err
			}
			c = c.Decayed(now)
			out = append(out, store.ReputationEntry{Address: addr, OpsSeen: c.Seen, OpsIncluded: c.Included})
		}
		return nil
	})
	return out, err
}

func (r *reputationStore) Clear(context.Context) error {
	return r.s.db.DropPrefix([]byte(r.s.reputationPrefix()))
}

func (s *Store) codeHashKey(h common.Hash) []byte {
	return []byte(s.cfg.Key("checks", "codeHashes", h.String()))
}

type codeHashStore struct{ s *Store }

func (c *codeHashStore) Save(_ context.Context, userOpHash common.Hash, hashes []store.CodeHash) error {
	raw, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	return c.s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(c.s.codeHashKey(userOpHash), raw)
	})
}

func (c *codeHashStore) Get(_ context.Context, userOpHash common.Hash) ([]store.CodeHash, error) {
	var hashes []store.CodeHash
	err := c.s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(c.s.codeHashKey(userOpHash))
		if err == badger.ErrKeyNotFound {
			return store.ErrNotFound
		}
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			return json.Unmarshal(v, &hashes)
		})
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

func (c *codeHashStore) Remove(_ context.Context, userOpHashes ...common.Hash) error {
	return c.s.db.Update(func(txn *badger.Txn) error {
		for _, h := range userOpHashes {
			if err := txn.Delete(c.s.codeHashKey(h)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *codeHashStore) Clear(context.Context) error {
	return c.s.db.DropPrefix([]byte(c.s.cfg.Key("checks", "codeHashes") + ":"))
}
