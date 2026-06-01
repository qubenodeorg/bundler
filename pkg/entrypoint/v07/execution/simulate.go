package execution

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	epv07 "github.com/zarbanio/bundler/pkg/entrypoint/v07"
	v07reverts "github.com/zarbanio/bundler/pkg/entrypoint/v07/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
	uov07 "github.com/zarbanio/bundler/pkg/userop/v07"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	Sos        state.OverrideSet
	ChainID    *big.Int
	Target     common.Address
	Data       []byte
}

func toPackedOp(op userop.UserOperationInterface) (*epv07.PackedUserOperation, error) {
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v07op, err := uov07.New(data)
	if err != nil {
		return nil, err
	}
	return &epv07.PackedUserOperation{
		Sender:             v07op.Sender,
		Nonce:              v07op.Nonce,
		InitCode:           v07op.PackInitCode(),
		CallData:           v07op.CallData,
		AccountGasLimits:   v07op.AccountGasLimits(),
		PreVerificationGas: v07op.PreVerificationGas,
		GasFees:            v07op.GasFees(),
		PaymasterAndData:   v07op.PackPaymasterAndData(),
		Signature:          v07op.Signature,
	}, nil
}

func SimulateHandleOp(in *SimulateInput) error {
	ep, err := epv07.NewV07(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(utils.DummyPk, in.ChainID)
	if err != nil {
		return err
	}
	auth.GasLimit = math.MaxUint64
	auth.NoSend = true

	packedOp, err := toPackedOp(in.Op)
	if err != nil {
		return err
	}

	beneficiary := common.HexToAddress("0x")
	tx, err := ep.HandleOps(auth, []epv07.PackedUserOperation{*packedOp}, beneficiary)
	if err != nil {
		return err
	}

	req := utils.EthCallReq{
		From: common.HexToAddress("0x"),
		To:   in.EntryPoint,
		Data: tx.Data(),
	}
	err = in.Rpc.CallContext(context.Background(), nil, "eth_call", &req, "latest", in.Sos)

	if err == nil {
		return nil
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := v07reverts.NewFailedOpWithRevert(err); foErr == nil {
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return err
}
