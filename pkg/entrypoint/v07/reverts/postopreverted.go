package reverts

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type PostOpRevertedRevert struct {
	ReturnData []byte
}

func postOpReverted() abi.Error {
	bytesT, _ := abi.NewType("bytes", "bytes", nil)
	return abi.NewError("PostOpReverted", abi.Arguments{
		{Name: "returnData", Type: bytesT},
	})
}

func NewPostOpReverted(err error) (*PostOpRevertedRevert, error) {
	rpcErr, ok := err.(rpc.DataError)
	if !ok {
		return nil, fmt.Errorf(
			"postOpReverted: cannot assert type: error is not of type rpc.DataError, err: %s",
			err,
		)
	}

	data, ok := rpcErr.ErrorData().(string)
	if !ok {
		return nil, fmt.Errorf(
			"postOpReverted: cannot assert type: data is not of type string, err: %s, data: %s",
			rpcErr.Error(),
			rpcErr.ErrorData(),
		)
	}

	postOp := postOpReverted()
	revert, err := postOp.Unpack(common.Hex2Bytes(data[2:]))
	if err != nil {
		return nil, fmt.Errorf("postOpReverted: %s", err)
	}

	args, ok := revert.([]any)
	if !ok {
		return nil, errors.New("postOpReverted: cannot assert type: args is not of type []any")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("postOpReverted: invalid args length: expected 1, got %d", len(args))
	}

	returnData, ok := args[0].([]byte)
	if !ok {
		return nil, errors.New("postOpReverted: cannot assert type: returnData is not of type []byte")
	}

	return &PostOpRevertedRevert{
		ReturnData: returnData,
	}, nil
}
