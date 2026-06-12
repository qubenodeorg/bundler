package checks

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateOpEntityRoles enforces STO-040: an address in the mempool may not be
// used as a sender in one UserOperation and as a different entity
// (paymaster/factory) in another.
func ValidateOpEntityRoles(ctx *modules.UserOpHandlerCtx) error {
	sender := ctx.UserOp.GetSender()

	// The new op's sender must not already appear as a non-sender entity in a
	// pending op. The mempool indexes pending ops by every entity role, so a
	// pending op with a different sender means our sender is its
	// factory/paymaster.
	for _, pending := range ctx.GetPendingSenderOps() {
		if pending.Sender != sender {
			return errors.NewRPCError(
				errors.BANNED_OPCODE,
				fmt.Sprintf(
					"%s is used as a different entity in another UserOperation currently in mempool",
					sender,
				),
				nil,
			)
		}
	}

	// The new op's factory/paymaster must not be the sender of a pending op.
	check := func(entity common.Address, pending []*userop.UserOperation) error {
		if entity == common.HexToAddress("0x") {
			return nil
		}
		for _, p := range pending {
			if p.Sender == entity {
				return errors.NewRPCError(
					errors.BANNED_OPCODE,
					fmt.Sprintf(
						"%s is used as a sender entity in another UserOperation currently in mempool",
						entity,
					),
					nil,
				)
			}
		}
		return nil
	}
	if err := check(ctx.UserOp.GetFactory(), ctx.GetPendingFactoryOps()); err != nil {
		return err
	}
	return check(ctx.UserOp.GetPaymaster(), ctx.GetPendingPaymasterOps())
}
