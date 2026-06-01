package reverts

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type DelegateAndRevertRevert struct {
	Success bool
	Ret     []byte
}

func delegateAndRevert() abi.Error {
	boolT, _ := abi.NewType("bool", "bool", nil)
	bytesT, _ := abi.NewType("bytes", "bytes", nil)
	return abi.NewError("DelegateAndRevert", abi.Arguments{
		{Name: "success", Type: boolT},
		{Name: "ret", Type: bytesT},
	})
}

func NewDelegateAndRevert(err error) (*DelegateAndRevertRevert, error) {
	rpcErr, ok := err.(rpc.DataError)
	if !ok {
		return nil, fmt.Errorf(
			"delegateAndRevert: cannot assert type: error is not of type rpc.DataError, err: %s",
			err,
		)
	}

	data, ok := rpcErr.ErrorData().(string)
	if !ok {
		return nil, fmt.Errorf(
			"delegateAndRevert: cannot assert type: data is not of type string, err: %s, data: %s",
			rpcErr.Error(),
			rpcErr.ErrorData(),
		)
	}

	dar := delegateAndRevert()
	revert, err := dar.Unpack(common.Hex2Bytes(data[2:]))
	if err != nil {
		return nil, fmt.Errorf("delegateAndRevert: %s", err)
	}

	args, ok := revert.([]any)
	if !ok {
		return nil, errors.New("delegateAndRevert: cannot assert type: args is not of type []any")
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("delegateAndRevert: invalid args length: expected 2, got %d", len(args))
	}

	success, ok := args[0].(bool)
	if !ok {
		return nil, errors.New("delegateAndRevert: cannot assert type: success is not of type bool")
	}

	ret, ok := args[1].([]byte)
	if !ok {
		return nil, errors.New("delegateAndRevert: cannot assert type: ret is not of type []byte")
	}

	return &DelegateAndRevertRevert{
		Success: success,
		Ret:     ret,
	}, nil
}
