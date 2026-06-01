package reverts

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type FailedOpWithRevertRevert struct {
	OpIndex int
	Reason  string
	Inner   []byte
}

func failedOpWithRevert() abi.Error {
	uint256, _ := abi.NewType("uint256", "uint256", nil)
	stringT, _ := abi.NewType("string", "string", nil)
	bytesT, _ := abi.NewType("bytes", "bytes", nil)
	return abi.NewError("FailedOpWithRevert", abi.Arguments{
		{Name: "opIndex", Type: uint256},
		{Name: "reason", Type: stringT},
		{Name: "inner", Type: bytesT},
	})
}

func NewFailedOpWithRevert(err error) (*FailedOpWithRevertRevert, error) {
	rpcErr, ok := err.(rpc.DataError)
	if !ok {
		return nil, fmt.Errorf(
			"failedOpWithRevert: cannot assert type: error is not of type rpc.DataError, err: %s",
			err,
		)
	}

	data, ok := rpcErr.ErrorData().(string)
	if !ok {
		return nil, fmt.Errorf(
			"failedOpWithRevert: cannot assert type: data is not of type string, err: %s, data: %s",
			rpcErr.Error(),
			rpcErr.ErrorData(),
		)
	}

	failedOp := failedOpWithRevert()
	revert, err := failedOp.Unpack(common.Hex2Bytes(data[2:]))
	if err != nil {
		return nil, fmt.Errorf("failedOpWithRevert: %s", err)
	}

	args, ok := revert.([]any)
	if !ok {
		return nil, errors.New("failedOpWithRevert: cannot assert type: args is not of type []any")
	}
	if len(args) != 3 {
		return nil, fmt.Errorf("failedOpWithRevert: invalid args length: expected 3, got %d", len(args))
	}

	opIndex, ok := args[0].(*big.Int)
	if !ok {
		return nil, errors.New("failedOpWithRevert: cannot assert type: opIndex is not of type *big.Int")
	}

	reason, ok := args[1].(string)
	if !ok {
		return nil, errors.New("failedOpWithRevert: cannot assert type: reason is not of type string")
	}

	inner, ok := args[2].([]byte)
	if !ok {
		return nil, errors.New("failedOpWithRevert: cannot assert type: inner is not of type []byte")
	}

	return &FailedOpWithRevertRevert{
		OpIndex: int(opIndex.Int64()),
		Reason:  reason,
		Inner:   inner,
	}, nil
}
