package methods

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	HandleOpsMethod = abi.NewMethod(
		"handleOps",
		"handleOps",
		abi.Function,
		"",
		false,
		false,
		abi.Arguments{
			{Name: "ops", Type: packedUserOpArr},
			{Name: "beneficiary", Type: address},
		},
		nil,
	)
	HandleOpsSelector = hexutil.Encode(HandleOpsMethod.ID)
)
