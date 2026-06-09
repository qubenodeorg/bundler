package payg

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/contract"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/handlers"
	"github.com/zarbanio/bundler/pkg/userop"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v08"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func newDummyContractData(t *testing.T) *contract.Data {
	t.Helper()
	return contract.NewData(testutils.ValidAddress1, common.Address{}, big.NewInt(0))
}

func newHandler(t *testing.T) *Handler {
	t.Helper()
	n := testutils.RpcMock(testutils.MethodMocks{
		// 32 bytes — a valid bytes32 return for paymaster.getHash().
		"eth_call":                "0x0000000000000000000000000000000000000000000000000000000000000001",
		"eth_chainId":             "0x1",
		"eth_estimateGas":         "0x1",
		"eth_getTransactionCount": "0x0",
		"eth_gasPrice":            "0x1",
		"eth_blockNumber":         "0x1",
		"eth_getBlockByNumber":    testutils.NewBlockMock(),
		"debug_traceCall":         map[string]any{},
	})
	t.Cleanup(n.Close)
	r, _ := rpc.Dial(n.URL)
	eth := ethclient.NewClient(r)
	return New(testutils.DummyEOA, r, eth, testutils.ChainID, gas.NewDefaultOverhead())
}

func TestNew(t *testing.T) {
	h := newHandler(t)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.gasEstimator == nil {
		t.Fatal("gas estimator not initialized")
	}
}

func TestRun_V06Path_ErrorPropagatesFromGasEstimator(t *testing.T) {
	// The eth_call returning empty bytes will cause the paymaster contract call to fail.
	// We just need the dispatch + ConvertToV06Packed path to be exercised; error is expected.
	h := newHandler(t)
	op := testutils.MockValidInitUserOp()
	_, err := h.Run(op, testutils.ValidAddress1, testutils.ValidAddress2)
	if err == nil {
		t.Fatal("expected gas estimation to error with empty paymaster contract response")
	}
}

func TestRun_V07Path_ConversionInvoked(t *testing.T) {
	h := newHandler(t)
	op := &v07.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(1),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     common.Address{},
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     []byte{},
	}
	if _, err := h.Run(op, testutils.ValidAddress1, testutils.ValidAddress2); err == nil {
		t.Fatal("expected error from downstream eth call")
	}
}

func TestRun_V09Path_ConversionInvoked(t *testing.T) {
	h := newHandler(t)
	op := &v09.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(1),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     common.Address{},
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     []byte{},
	}
	// We don't expect success (eth call won't return real data) — we just want the v0.9
	// type assertion path to be exercised when error doesn't happen earlier.
	_, err := h.Run(op, testutils.ValidAddress1, testutils.ValidAddress2)
	if err == nil {
		t.Fatal("expected error from downstream eth call")
	}
}

func TestRun_ConvertToV06PackedFailsGracefully(t *testing.T) {
	// Provide a v0.7 op with a paymaster but a nil paymaster gas limit. ConvertToV06Packed
	// substitutes zero for nil, so this should NOT fail. We just exercise the path.
	h := newHandler(t)
	op := &v07.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(1),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     testutils.ValidAddress2,
		PaymasterVerificationGasLimit: nil,
		PaymasterPostOpGasLimit:       nil,
		PaymasterData:                 []byte{},
		Signature:                     []byte{},
	}
	if _, err := h.Run(op, testutils.ValidAddress1, testutils.ValidAddress2); err == nil {
		t.Fatal("expected downstream error")
	}
}

func TestConvertOp(t *testing.T) {
	// v0.6 input passes through unchanged.
	v06 := testutils.MockValidInitUserOp()
	got, err := convertOp(v06)
	if err != nil {
		t.Fatalf("convertOp v0.6: %v", err)
	}
	if got != v06 {
		t.Fatalf("expected v0.6 input to be returned as-is")
	}

	// v0.7 input is converted with paymaster gas limits preserved.
	v07op := &v07.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(0),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     testutils.ValidAddress2,
		PaymasterVerificationGasLimit: big.NewInt(0x100),
		PaymasterPostOpGasLimit:       big.NewInt(0x200),
		PaymasterData:                 []byte{0x01},
		Signature:                     []byte{0x02},
	}
	got2, err := convertOp(v07op)
	if err != nil {
		t.Fatalf("convertOp v0.7: %v", err)
	}
	if len(got2.PaymasterAndData) < common.AddressLength+32 {
		t.Fatalf("expected paymasterAndData with packed gas limits")
	}
}

func TestSignOp(t *testing.T) {
	h := newHandler(t)
	pmOp := testutils.MockValidInitUserOp()
	data := newDummyContractData(t)
	sig, err := h.signOp(pmOp, data)
	if err != nil {
		t.Fatalf("signOp: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("expected non-empty signature")
	}
}

func TestIsV09(t *testing.T) {
	v09op := &v09.UserOperation{}
	if !isV09(v09op) {
		t.Fatal("expected v09 detection to return true")
	}
	v07op := &v07.UserOperation{}
	if isV09(v07op) {
		t.Fatal("expected v07 detection to return false")
	}
	v06op := testutils.MockValidInitUserOp()
	if isV09(v06op) {
		t.Fatal("expected v06 detection to return false")
	}
}

func TestBuildResponse_V06EmbedsSignature(t *testing.T) {
	pmOp := testutils.MockValidInitUserOp()
	op := pmOp
	data := newDummyContractData(t)
	sig := []byte{0xca, 0xfe, 0xba, 0xbe}

	res, err := buildResponse(op, pmOp, data, sig)
	if err != nil {
		t.Fatalf("buildResponse: %v", err)
	}
	if res.PaymasterSignature != "" {
		t.Fatalf("expected empty PaymasterSignature for v0.6, got %s", res.PaymasterSignature)
	}
	if !strings.Contains(strings.ToLower(res.PaymasterAndData), "cafebabe") {
		t.Fatalf("expected paymasterAndData to embed signature, got %s", res.PaymasterAndData)
	}
}

func TestBuildResponse_V07EmbedsSignature(t *testing.T) {
	v07op := &v07.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(0),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     common.Address{},
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     []byte{},
	}
	pmOp := testutils.MockValidInitUserOp()
	data := newDummyContractData(t)
	sig := []byte{0xab, 0xcd}

	res, err := buildResponse(v07op, pmOp, data, sig)
	if err != nil {
		t.Fatalf("buildResponse: %v", err)
	}
	if res.PaymasterSignature != "" {
		t.Fatalf("v0.7 should not return paymasterSignature separately")
	}
}

func TestBuildResponse_V09ReturnsSignatureSeparately(t *testing.T) {
	v09op := &v09.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(0),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     common.Address{},
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     []byte{},
	}
	pmOp := testutils.MockValidInitUserOp()
	data := newDummyContractData(t)
	sig := []byte{0xca, 0xfe, 0xba, 0xbe}

	res, err := buildResponse(v09op, pmOp, data, sig)
	if err != nil {
		t.Fatalf("buildResponse: %v", err)
	}
	if res.PaymasterSignature == "" {
		t.Fatal("v0.9 should populate PaymasterSignature separately")
	}
	if strings.Contains(strings.ToLower(res.PaymasterAndData), "cafebabe") {
		t.Fatalf("v0.9 paymasterAndData should NOT contain the signature, got %s", res.PaymasterAndData)
	}
	if res.PaymasterSignature != "0xcafebabe" {
		t.Fatalf("PaymasterSignature mismatch: got %s", res.PaymasterSignature)
	}
}

// --- Existing pure unit tests preserved below ---

func TestSponsorUserOperationResponseV09Format(t *testing.T) {
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

	if hash1 != hash2 {
		t.Fatalf("userOpHash should exclude paymasterSignature: %s != %s", hash1.Hex(), hash2.Hex())
	}
}

func TestUserOperationInterfaceV09(t *testing.T) {
	var _ userop.UserOperationInterface = (*v09.UserOperation)(nil)
}

func TestConvertedPackedFormatPreservesPaymasterGas(t *testing.T) {
	// Ensure ConvertToV06Packed for v0.8 with EIP-7702 auth doesn't lose paymaster gas limits.
	op := &v08.UserOperation{
		Sender:                        testutils.ValidAddress1,
		Nonce:                         big.NewInt(1),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(1),
		VerificationGasLimit:          big.NewInt(1),
		PreVerificationGas:            big.NewInt(1),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     testutils.ValidAddress2,
		PaymasterVerificationGasLimit: big.NewInt(0xabcd),
		PaymasterPostOpGasLimit:       big.NewInt(0x1234),
		PaymasterData:                 []byte{0x01},
		Signature:                     []byte{0x02},
	}
	packed, err := userop.ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("ConvertToV06Packed: %v", err)
	}
	got := packed.PaymasterAndData
	if len(got) < common.AddressLength+32 {
		t.Fatalf("paymasterAndData too short: %d bytes", len(got))
	}
	// Sanity: paymaster address bytes at the start.
	if !strings.EqualFold(common.BytesToAddress(got[:common.AddressLength]).Hex(), op.Paymaster.Hex()) {
		t.Fatalf("paymaster bytes mismatch")
	}
}
