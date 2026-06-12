package entities

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// FlexInt decodes from a JSON number, a decimal string, or a 0x-prefixed hex
// string; debug_bundler_setReputation clients send any of the three.
type FlexInt int

func (n *FlexInt) UnmarshalJSON(data []byte) error {
	var num int
	if err := json.Unmarshal(data, &num); err == nil {
		*n = FlexInt(num)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("entities: cannot decode %s as int", data)
	}
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s, base = s[2:], 16
	}
	v, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return fmt.Errorf("entities: cannot decode %q as int: %w", s, err)
	}
	*n = FlexInt(v)
	return nil
}

type ReputationOverride struct {
	Address     common.Address `json:"address"`
	OpsSeen     FlexInt        `json:"opsSeen"`
	OpsIncluded FlexInt        `json:"opsIncluded"`
}

// ReputationConstants are a collection of values for determining the appropriate status of a UserOperation
// coming into the mempool.
type ReputationConstants struct {
	MinUnstakeDelay                int
	MinStakeValue                  int64
	SameSenderMempoolCount         int
	SameUnstakedEntityMempoolCount int
	ThrottledEntityMempoolCount    int
	ThrottledEntityLiveBlocks      int
	ThrottledEntityBundleCount     int
	MinInclusionRateDenominator    int
	ThrottlingSlack                int
	BanSlack                       int
}
