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
	epv08 "github.com/zarbanio/bundler/pkg/entrypoint/v08"
	v08reverts "github.com/zarbanio/bundler/pkg/entrypoint/v08/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
	uov08 "github.com/zarbanio/bundler/pkg/userop/v08"
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

func toPackedOp(op userop.UserOperationInterface) (*epv08.PackedUserOperation, error) {
	if v08op, ok := op.(*uov08.UserOperation); ok {
		return &epv08.PackedUserOperation{
			Sender:             v08op.Sender,
			Nonce:              v08op.Nonce,
			InitCode:           v08op.PackInitCode(),
			CallData:           v08op.CallData,
			AccountGasLimits:   v08op.AccountGasLimits(),
			PreVerificationGas: v08op.PreVerificationGas,
			GasFees:            v08op.GasFees(),
			PaymasterAndData:   v08op.PackPaymasterAndData(),
			Signature:          v08op.Signature,
		}, nil
	}
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v08op, err := uov08.New(data)
	if err != nil {
		return nil, err
	}
	return &epv08.PackedUserOperation{
		Sender:             v08op.Sender,
		Nonce:              v08op.Nonce,
		InitCode:           v08op.PackInitCode(),
		CallData:           v08op.CallData,
		AccountGasLimits:   v08op.AccountGasLimits(),
		PreVerificationGas: v08op.PreVerificationGas,
		GasFees:            v08op.GasFees(),
		PaymasterAndData:   v08op.PackPaymasterAndData(),
		Signature:          v08op.Signature,
	}, nil
}

func SimulateHandleOp(in *SimulateInput) error {
	ep, err := epv08.NewV08(in.EntryPoint, ethclient.NewClient(in.Rpc))
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
	tx, err := ep.HandleOps(auth, []epv08.PackedUserOperation{*packedOp}, beneficiary)
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

	if fo, foErr := v08reverts.NewFailedOpWithRevert(err); foErr == nil {
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return err
}
