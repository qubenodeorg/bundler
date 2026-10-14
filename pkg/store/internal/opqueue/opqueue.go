// Package opqueue provides an in-process, arrival-ordered index of pending
// UserOperations. It is used by the embedded backends (memory and BadgerDB) to
// answer entity/arrival queries without scanning storage. Network backends
// implement the same semantics with native ordered structures.
package opqueue

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/wangjia184/sortedset"
	"github.com/zarbanio/bundler/pkg/userop"
)

type set struct {
	all      *sortedset.SortedSet
	entities map[common.Address]*sortedset.SortedSet
}

func (s *set) entity(addr common.Address) *sortedset.SortedSet {
	if _, ok := s.entities[addr]; !ok {
		s.entities[addr] = sortedset.New()
	}
	return s.entities[addr]
}

// Queues is a concurrency-safe collection of per-EntryPoint op indexes.
type Queues struct {
	mu               sync.RWMutex
	setsByEntryPoint map[common.Address]*set
}

// New returns an empty Queues.
func New() *Queues {
	return &Queues{setsByEntryPoint: make(map[common.Address]*set)}
}

func (q *Queues) getEntryPointSet(entryPoint common.Address) *set {
	s, ok := q.setsByEntryPoint[entryPoint]
	if !ok {
		s = &set{all: sortedset.New(), entities: make(map[common.Address]*sortedset.SortedSet)}
		q.setsByEntryPoint[entryPoint] = s
	}
	return s
}

// Key is the stable identifier of an op within a queue.
func Key(entryPoint common.Address, sender common.Address, nonce string) string {
	return entryPoint.Hex() + "|" + sender.Hex() + "|" + nonce
}

// Add inserts or replaces an op. On replacement it also drops stale
// factory/paymaster index entries (the sender index entry is keyed by the same
// op and is simply overwritten).
func (q *Queues) Add(entryPoint common.Address, op *userop.UserOperation) {
	q.mu.Lock()
	defer q.mu.Unlock()

	s := q.getEntryPointSet(entryPoint)
	key := Key(entryPoint, op.Sender, op.Nonce.String())

	if existing := s.all.GetByKey(key); existing != nil {
		if old, ok := existing.Value.(*userop.UserOperation); ok {
			s.entity(old.GetFactory()).Remove(key)
			s.entity(old.GetPaymaster()).Remove(key)
		}
	}
	s.all.AddOrUpdate(key, sortedset.SCORE(s.all.GetCount()), op)
	s.entity(op.Sender).AddOrUpdate(key, sortedset.SCORE(op.Nonce.Int64()), op)
	if factory := op.GetFactory(); factory != (common.Address{}) {
		fs := s.entity(factory)
		fs.AddOrUpdate(key, sortedset.SCORE(fs.GetCount()), op)
	}
	if paymaster := op.GetPaymaster(); paymaster != (common.Address{}) {
		ps := s.entity(paymaster)
		ps.AddOrUpdate(key, sortedset.SCORE(ps.GetCount()), op)
	}
}

// Ops returns every op in which entity appears in any role.
func (q *Queues) Ops(entryPoint common.Address, entity common.Address) []*userop.UserOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	entries, ok := q.setsByEntryPoint[entryPoint]
	if !ok {
		return nil
	}
	es, ok := entries.entities[entity]
	if !ok || es.GetCount() == 0 {
		return nil
	}
	nodes := es.GetByRankRange(-1, -es.GetCount(), false)
	batch := make([]*userop.UserOperation, 0, len(nodes))
	for _, n := range nodes {
		batch = append(batch, n.Value.(*userop.UserOperation))
	}
	return batch
}

// All returns every op for the EntryPoint in arrival order.
func (q *Queues) All(entryPoint common.Address) []*userop.UserOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	entries, ok := q.setsByEntryPoint[entryPoint]
	if !ok {
		return nil
	}
	nodes := entries.all.GetByRankRange(1, -1, false)
	batch := make([]*userop.UserOperation, 0, len(nodes))
	for _, n := range nodes {
		batch = append(batch, n.Value.(*userop.UserOperation))
	}
	return batch
}

// Remove deletes the given ops from every index.
func (q *Queues) Remove(entryPoint common.Address, ops ...*userop.UserOperation) {
	q.mu.Lock()
	defer q.mu.Unlock()

	entries, ok := q.setsByEntryPoint[entryPoint]
	if !ok {
		return
	}
	for _, op := range ops {
		key := Key(entryPoint, op.Sender, op.Nonce.String())
		entries.all.Remove(key)
		entries.entity(op.Sender).Remove(key)
		entries.entity(op.GetFactory()).Remove(key)
		entries.entity(op.GetPaymaster()).Remove(key)
	}
}

// Reset discards every index.
func (q *Queues) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.setsByEntryPoint = make(map[common.Address]*set)
}
