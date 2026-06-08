package payg

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/handlers"
	"github.com/zarbanio/bundler/pkg/userop"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func TestSponsorUserOperationResponseV09Format(t *testing.T) {
	// Verify the response struct can hold paymasterSignature
	res := &handlers.SponsorUserOperationResponse{
		PaymasterAndData:     "0x1234",
		PaymasterSignature:   "0xabcd",
		PreVerificationGas:   "0x0",
		VerificationGasLimit: "0x0",
		CallGasLimit:         "0x0",
	}

	if res.PaymasterSignature != "0xabcd" {
		t.Fatalf("PaymasterSignature not set correctly")
	}
}

func TestPaymasterSignatureExcludedFromHash(t *testing.T) {
	op1 := &v09.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0x"),
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(100000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(1000000000),
		MaxPriorityFeePerGas:          big.NewInt(1000000000),
		Paymaster:                     common.HexToAddress("0x1234567890123456789012345678901234567890"),
		PaymasterVerificationGasLimit: big.NewInt(50000),
		PaymasterPostOpGasLimit:       big.NewInt(50000),
		PaymasterData:                 []byte{0xde, 0xad},
		Signature:                     []byte{0xab},
	}

	op2 := &v09.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0x"),
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(100000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(1000000000),
		MaxPriorityFeePerGas:          big.NewInt(1000000000),
		Paymaster:                     common.HexToAddress("0x1234567890123456789012345678901234567890"),
		PaymasterVerificationGasLimit: big.NewInt(50000),
		PaymasterPostOpGasLimit:       big.NewInt(50000),
		PaymasterData:                 []byte{0xde, 0xad},
		PaymasterSignature:            []byte{0xca, 0xfe},
		Signature:                     []byte{0xab},
	}

	ep := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	chainID := big.NewInt(1)

	hash1 := op1.GetUserOpHash(ep, chainID)
	hash2 := op2.GetUserOpHash(ep, chainID)

	// The paymasterSignature should NOT affect the userOpHash
	if hash1 != hash2 {
		t.Fatalf("userOpHash should exclude paymasterSignature: %s != %s", hash1.Hex(), hash2.Hex())
	}
}

func TestUserOperationInterfaceV09(t *testing.T) {
	var _ userop.UserOperationInterface = (*v09.UserOperation)(nil)
}
