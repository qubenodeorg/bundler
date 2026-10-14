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
//
// Every op is scored with a strictly increasing arrival counter that never
// shrinks when ops are removed, and that same score is reused across the
// arrival set and every entity set, so ordering is stable and consistent.
type Queues struct {
	mu               sync.RWMutex
	next             map[common.Address]uint64
	setsByEntryPoint map[common.Address]*set
}

// New returns an empty Queues.
func New() *Queues {
	return &Queues{
		next:             make(map[common.Address]uint64),
		setsByEntryPoint: make(map[common.Address]*set),
	}
}

func (q *Queues) getEntryPointSet(entryPoint common.Address) *set {
	s, ok := q.setsByEntryPoint[entryPoint]
	if !ok {
		s = &set{all: sortedset.New(), entities: make(map[common.Address]*sortedset.SortedSet)}
		q.setsByEntryPoint[entryPoint] = s
	}
	return s
}

// nextScore returns the next monotonic arrival score for the EntryPoint.
// Scores start at 1 so 0 can denote "no persisted score" during reload.
func (q *Queues) nextScore(entryPoint common.Address) uint64 {
	v := q.next[entryPoint]
	if v == 0 {
		v = 1
	}
	q.next[entryPoint] = v + 1
	return v
}

// Key is the stable identifier of an op within a queue.
func Key(entryPoint common.Address, sender common.Address, nonce string) string {
	return entryPoint.Hex() + "|" + sender.Hex() + "|" + nonce
}

// Next reserves and returns the next arrival score without inserting anything.
// Callers that persist state use it to record the score before inserting.
func (q *Queues) Next(entryPoint common.Address) uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.nextScore(entryPoint)
}

// Add inserts or replaces an op at the next arrival score.
func (q *Queues) Add(entryPoint common.Address, op *userop.UserOperation) uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	score := q.nextScore(entryPoint)
	q.insertLocked(entryPoint, op, score)
	return score
}

// Insert indexes an op at an explicit arrival score, advancing the counter past
// it. It is used when reloading persisted state so arrival order survives a
// restart.
func (q *Queues) Insert(entryPoint common.Address, op *userop.UserOperation, score uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.next[entryPoint] <= score {
		q.next[entryPoint] = score + 1
	}
	q.insertLocked(entryPoint, op, score)
}

// insertLocked adds op at score. On replacement it drops stale factory/paymaster
// index entries (the sender index entry is keyed by the same op and is simply
// overwritten).
func (q *Queues) insertLocked(entryPoint common.Address, op *userop.UserOperation, score uint64) {
	s := q.getEntryPointSet(entryPoint)
	key := Key(entryPoint, op.Sender, op.Nonce.String())

	if existing := s.all.GetByKey(key); existing != nil {
		if old, ok := existing.Value.(*userop.UserOperation); ok {
			s.entity(old.GetFactory()).Remove(key)
			s.entity(old.GetPaymaster()).Remove(key)
		}
	}
	sc := sortedset.SCORE(score)
	s.all.AddOrUpdate(key, sc, op)
	s.entity(op.Sender).AddOrUpdate(key, sc, op)
	if factory := op.GetFactory(); factory != (common.Address{}) {
		s.entity(factory).AddOrUpdate(key, sc, op)
	}
	if paymaster := op.GetPaymaster(); paymaster != (common.Address{}) {
		s.entity(paymaster).AddOrUpdate(key, sc, op)
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

// Reset discards every index and the arrival counters.
func (q *Queues) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next = make(map[common.Address]uint64)
	q.setsByEntryPoint = make(map[common.Address]*set)
}
