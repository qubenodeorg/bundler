package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ReputationStatus is the EIP-4337 reputation classification of an entity.
type ReputationStatus int

const (
	// StatusOK means the entity is allowed to submit new ops.
	StatusOK ReputationStatus = iota
	// StatusThrottled means the entity may keep only a bounded number of ops in
	// the pool.
	StatusThrottled
	// StatusBanned means no new ops from the entity are accepted.
	StatusBanned
)

// ReputationParams are the thresholds needed to classify an entity from its
// counters. They mirror the relevant fields of the bundler's
// entities.ReputationConstants without creating an import cycle.
type ReputationParams struct {
	MinInclusionRateDenominator int
	ThrottlingSlack             int
	BanSlack                    int
}

// ReputationEntry is a snapshot of an entity's counters.
type ReputationEntry struct {
	Address     common.Address
	OpsSeen     int
	OpsIncluded int
}

// ReputationDelta is a signed update to an entity's counters. OpsSeen and
// OpsIncluded may be negative; implementations clamp OpsSeen to >= 0.
type ReputationDelta struct {
	Address     common.Address
	OpsSeen     int
	OpsIncluded int
}

// ReputationStore tracks opsSeen/opsIncluded counters per entity.
//
// All counter updates for a single logical event (for example one UserOperation
// touching a sender, a factory, and a paymaster) must be applied through one
// Apply call so the update is atomic across entities.
type ReputationStore interface {
	// Status classifies an entity using its decayed counters.
	Status(ctx context.Context, entity common.Address, params ReputationParams) (ReputationStatus, error)
	// OpsCount returns the decayed opsSeen/opsIncluded counters for an entity.
	OpsCount(ctx context.Context, entity common.Address) (opsSeen int, opsIncluded int, err error)
	// Apply atomically adds the given signed deltas to each entity's counters.
	Apply(ctx context.Context, deltas ...ReputationDelta) error
	// Override replaces the counters of each entity with the given values.
	Override(ctx context.Context, entries []ReputationEntry) error
	// Dump returns every entity with a non-zero counter set.
	Dump(ctx context.Context) ([]ReputationEntry, error)
	// Clear removes all reputation counters.
	Clear(ctx context.Context) error
}

// ReputationEMAHours is the window over which reputation counters decay, as
// specified in EIP-4337 (24 hours).
const ReputationEMAHours = 24

// OpsCount is the persisted representation of an entity's reputation counters:
// the pair of counters plus the time they were last written. Decay is applied
// lazily on read/update based on how much wall-clock time has elapsed.
type OpsCount struct {
	Seen      int
	Included  int
	UpdatedAt time.Time
}

// Decayed applies EIP-4337 exponential decay to the counters and stamps the
// result as updated now. A single hourly step reduces each counter by
// count/EMAHours until both are below the decay floor (24).
func (c OpsCount) Decayed(now time.Time) OpsCount {
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
		return c
	}
	hours := int(now.Sub(c.UpdatedAt).Hours())
	for i := hours; i > 0; i-- {
		if c.Seen < 24 && c.Included < 24 {
			break
		}
		c.Seen -= c.Seen / ReputationEMAHours
		c.Included -= c.Included / ReputationEMAHours
	}
	c.UpdatedAt = now
	return c
}

// ApplyDelta decays then adds the signed delta, clamping seen to >= 0.
func (c OpsCount) ApplyDelta(d ReputationDelta, now time.Time) OpsCount {
	c = c.Decayed(now)
	c.Seen += d.OpsSeen
	if c.Seen < 0 {
		c.Seen = 0
	}
	c.Included += d.OpsIncluded
	if c.Included < 0 {
		c.Included = 0
	}
	c.UpdatedAt = now
	return c
}

// Encode serializes the counters in the legacy "seen:included:unix" format so
// pre-existing BadgerDB values remain readable.
func (c OpsCount) Encode() []byte {
	return []byte(strings.Join([]string{
		strconv.Itoa(c.Seen),
		strconv.Itoa(c.Included),
		strconv.FormatInt(c.UpdatedAt.Unix(), 10),
	}, ":"))
}

// DecodeOpsCount parses the "seen:included:unix" representation.
func DecodeOpsCount(b []byte) (OpsCount, error) {
	parts := strings.Split(string(b), ":")
	if len(parts) != 3 {
		return OpsCount{}, fmt.Errorf("store: invalid ops count %q", string(b))
	}
	seen, err := strconv.Atoi(parts[0])
	if err != nil {
		return OpsCount{}, fmt.Errorf("store: invalid opsSeen: %w", err)
	}
	included, err := strconv.Atoi(parts[1])
	if err != nil {
		return OpsCount{}, fmt.Errorf("store: invalid opsIncluded: %w", err)
	}
	unix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return OpsCount{}, fmt.Errorf("store: invalid lastUpdated: %w", err)
	}
	return OpsCount{Seen: seen, Included: included, UpdatedAt: time.Unix(unix, 0)}, nil
}

// ComputeStatus classifies an entity from its decayed counters.
func ComputeStatus(opsSeen, opsIncluded int, p ReputationParams) ReputationStatus {
	if opsSeen == 0 {
		return StatusOK
	}
	minExpectedIncluded := opsSeen / p.MinInclusionRateDenominator
	switch {
	case minExpectedIncluded <= opsIncluded+p.ThrottlingSlack:
		return StatusOK
	case minExpectedIncluded <= opsIncluded+p.BanSlack:
		return StatusThrottled
	default:
		return StatusBanned
	}
}

// AggregateDeltas sums deltas per address, preserving first-seen order.
func AggregateDeltas(deltas []ReputationDelta) []ReputationDelta {
	order := make([]common.Address, 0, len(deltas))
	byAddr := make(map[common.Address]ReputationDelta, len(deltas))
	for _, d := range deltas {
		if _, ok := byAddr[d.Address]; !ok {
			order = append(order, d.Address)
		}
		agg := byAddr[d.Address]
		agg.Address = d.Address
		agg.OpsSeen += d.OpsSeen
		agg.OpsIncluded += d.OpsIncluded
		byAddr[d.Address] = agg
	}
	out := make([]ReputationDelta, 0, len(order))
	for _, addr := range order {
		out = append(out, byAddr[addr])
	}
	return out
}
