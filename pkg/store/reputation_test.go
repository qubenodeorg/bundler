package store

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestComputeStatus(t *testing.T) {
	params := ReputationParams{MinInclusionRateDenominator: 100, ThrottlingSlack: 10, BanSlack: 20}
	cases := []struct {
		name           string
		seen, included int
		want           ReputationStatus
	}{
		{"fresh", 0, 0, StatusOK},
		{"healthy", 100, 1, StatusOK},
		{"within throttling slack", 1500, 0, StatusThrottled},
		{"beyond ban slack", 2500, 0, StatusBanned},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeStatus(tc.seen, tc.included, params); got != tc.want {
				t.Fatalf("ComputeStatus(%d,%d) = %v, want %v", tc.seen, tc.included, got, tc.want)
			}
		})
	}
}

// TestComputeStatusZeroDenominator guards against a divide-by-zero panic from a
// misconfigured (unset) inclusion-rate denominator.
func TestComputeStatusZeroDenominator(t *testing.T) {
	if got := ComputeStatus(100, 0, ReputationParams{}); got != StatusBanned {
		t.Fatalf("ComputeStatus with zero denominator = %v, want Banned", got)
	}
	if got := ComputeStatus(0, 0, ReputationParams{}); got != StatusOK {
		t.Fatalf("ComputeStatus(0,0) = %v, want OK", got)
	}
}

func TestOpsCountDecay(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	c := OpsCount{Seen: 2400, Included: 240, UpdatedAt: base}

	decayed := c.Decayed(base.Add(time.Hour))
	if want := 2400 - 2400/ReputationEMAHours; decayed.Seen != want {
		t.Fatalf("Seen after 1h = %d, want %d", decayed.Seen, want)
	}
	if want := 240 - 240/ReputationEMAHours; decayed.Included != want {
		t.Fatalf("Included after 1h = %d, want %d", decayed.Included, want)
	}
	if !decayed.UpdatedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("UpdatedAt = %v, want %v", decayed.UpdatedAt, base.Add(time.Hour))
	}

	// Counters below the floor do not decay further.
	small := OpsCount{Seen: 10, Included: 5, UpdatedAt: base}
	if got := small.Decayed(base.Add(100 * time.Hour)); got.Seen != 10 || got.Included != 5 {
		t.Fatalf("small counters decayed to %+v, want unchanged", got)
	}
}

// TestOpsCountDecayPreservesFraction verifies only whole elapsed hours are
// consumed, so sub-hour time is carried forward rather than discarded.
func TestOpsCountDecayPreservesFraction(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	c := OpsCount{Seen: 2400, UpdatedAt: base}

	// 1h30m elapsed: exactly one whole hour is consumed.
	d := c.Decayed(base.Add(90 * time.Minute))
	if !d.UpdatedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("UpdatedAt = %v, want %v", d.UpdatedAt, base.Add(time.Hour))
	}

	// The preserved 30m combines with the next 30m to consume one more hour.
	d2 := d.Decayed(base.Add(2 * time.Hour))
	if !d2.UpdatedAt.Equal(base.Add(2 * time.Hour)) {
		t.Fatalf("UpdatedAt = %v, want %v", d2.UpdatedAt, base.Add(2*time.Hour))
	}
}

// TestApplyDeltaKeepsDecayClock verifies ApplyDelta does not reset the decay
// clock to now, so unused fractional time is not lost on every increment.
func TestApplyDeltaKeepsDecayClock(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	c := OpsCount{Seen: 100, UpdatedAt: base}
	got := c.ApplyDelta(ReputationDelta{OpsSeen: 1}, base.Add(30*time.Minute))
	if got.Seen != 101 {
		t.Fatalf("Seen = %d, want 101", got.Seen)
	}
	if !got.UpdatedAt.Equal(base) {
		t.Fatalf("UpdatedAt = %v, want unchanged %v", got.UpdatedAt, base)
	}
}

func TestEncodeDecodeOpsCount(t *testing.T) {
	at := time.Unix(1_700_000_000, 0)
	c := OpsCount{Seen: 5, Included: 2, UpdatedAt: at}
	got, err := DecodeOpsCount(c.Encode())
	if err != nil {
		t.Fatalf("DecodeOpsCount: %v", err)
	}
	if got.Seen != 5 || got.Included != 2 || !got.UpdatedAt.Equal(at) {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
}

func TestAggregateDeltas(t *testing.T) {
	a := common.HexToAddress("0xaa")
	b := common.HexToAddress("0xbb")
	got := AggregateDeltas([]ReputationDelta{
		{Address: a, OpsSeen: 1},
		{Address: b, OpsSeen: 2},
		{Address: a, OpsSeen: 3, OpsIncluded: 1},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Address != a || got[0].OpsSeen != 4 || got[0].OpsIncluded != 1 {
		t.Fatalf("got[0] = %+v, want a with (4,1)", got[0])
	}
	if got[1].Address != b || got[1].OpsSeen != 2 {
		t.Fatalf("got[1] = %+v, want b with (2,0)", got[1])
	}
}
