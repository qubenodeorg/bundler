package execution

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	epv09 "github.com/zarbanio/bundler/pkg/entrypoint/v09"
	v09reverts "github.com/zarbanio/bundler/pkg/entrypoint/v09/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
	uov09 "github.com/zarbanio/bundler/pkg/userop/v09"
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

func toPackedOp(op userop.UserOperationInterface) (*epv09.PackedUserOperation, error) {
	if v09op, ok := op.(*uov09.UserOperation); ok {
		return &epv09.PackedUserOperation{
			Sender:             v09op.Sender,
			Nonce:              v09op.Nonce,
			InitCode:           v09op.PackInitCode(),
			CallData:           v09op.CallData,
			AccountGasLimits:   v09op.AccountGasLimits(),
			PreVerificationGas: v09op.PreVerificationGas,
			GasFees:            v09op.GasFees(),
			PaymasterAndData:   v09op.PackPaymasterAndData(),
			Signature:          v09op.Signature,
		}, nil
	}
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v09op, err := uov09.New(data)
	if err != nil {
		return nil, err
	}
	return &epv09.PackedUserOperation{
		Sender:             v09op.Sender,
		Nonce:              v09op.Nonce,
		InitCode:           v09op.PackInitCode(),
		CallData:           v09op.CallData,
		AccountGasLimits:   v09op.AccountGasLimits(),
		PreVerificationGas: v09op.PreVerificationGas,
		GasFees:            v09op.GasFees(),
		PaymasterAndData:   v09op.PackPaymasterAndData(),
		Signature:          v09op.Signature,
	}, nil
}

func SimulateHandleOp(in *SimulateInput) error {
	ep, err := epv09.NewV09(in.EntryPoint, ethclient.NewClient(in.Rpc))
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
	tx, err := ep.HandleOps(auth, []epv09.PackedUserOperation{*packedOp}, beneficiary)
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

	if fo, foErr := v09reverts.NewFailedOpWithRevert(err); foErr == nil {
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return err
}
