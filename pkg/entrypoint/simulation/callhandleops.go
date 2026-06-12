package simulation

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/entrypoint/methods"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	"github.com/zarbanio/bundler/pkg/userop"
)

var callHandleOpsFrom = crypto.PubkeyToAddress(utils.DummyPk.PublicKey)

// CallHandleOps performs an eth_call of EntryPoint.handleOps([op]) with the
// op's exact gas values. Some handleOps-only failures — most notably AA51
// "prefund below actualGasCost", where the prefund implied by the op's gas
// limits cannot cover the full execution — never surface in simulateValidation
// because it stops after the validation phase. The call is stateless and the
// op's signature is untouched, so a revert here is a faithful prediction of an
// on-chain revert. Inner callData reverts do NOT revert handleOps and are not
// reported by this check.
func CallHandleOps(
	rpcClient *rpc.Client,
	entryPoint common.Address,
	op *userop.UserOperation,
) error {
	ho, err := methods.HandleOpsMethod.Inputs.Pack(
		[]entrypoint.UserOperation{entrypoint.UserOperation(op.Core())},
		callHandleOpsFrom,
	)
	if err != nil {
		return err
	}

	req := utils.EthCallReq{
		From: callHandleOpsFrom,
		To:   entryPoint,
		Data: append(methods.HandleOpsMethod.ID, ho...),
	}
	err = rpcClient.CallContext(context.Background(), nil, "eth_call", &req, "latest")
	if err == nil {
		return nil
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return fmt.Errorf("handleOps would revert: %s", fo.Reason)
	}
	return err
}
