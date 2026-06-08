package gas

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/zarbanio/bundler/pkg/userop"
	"github.com/zarbanio/bundler/pkg/userop/v06"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v08"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

type mockOp struct {
	toMapErr error
	paymaster common.Address
	sender    common.Address
	maxFee    *big.Int
}

func (m *mockOp) GetSender() common.Address                         { return m.sender }
func (m *mockOp) GetNonce() *big.Int                                { return big.NewInt(0) }
func (m *mockOp) GetPaymaster() common.Address                      { return m.paymaster }
func (m *mockOp) GetFactory() common.Address                        { return common.Address{} }
func (m *mockOp) GetFactoryData() []byte                            { return nil }
func (m *mockOp) GetCallData() []byte                               { return nil }
func (m *mockOp) GetCallGasLimit() *big.Int                         { return big.NewInt(0) }
func (m *mockOp) GetVerificationGasLimit() *big.Int                 { return big.NewInt(0) }
func (m *mockOp) GetPreVerificationGas() *big.Int                   { return big.NewInt(0) }
func (m *mockOp) GetMaxFeePerGas() *big.Int                         { return m.maxFee }
func (m *mockOp) GetMaxPriorityFeePerGas() *big.Int                 { return big.NewInt(0) }
func (m *mockOp) GetSignature() []byte                              { return nil }
func (m *mockOp) GetEIP7702Auth() interface{}                       { return nil }
func (m *mockOp) GetPaymasterVerificationGasLimit() *big.Int        { return big.NewInt(0) }
func (m *mockOp) GetPaymasterPostOpGasLimit() *big.Int              { return big.NewInt(0) }
func (m *mockOp) GetPaymasterData() []byte                          { return nil }
func (m *mockOp) GetMaxGasAvailable() *big.Int                      { return big.NewInt(0) }
func (m *mockOp) GetMaxPrefund() *big.Int                           { return big.NewInt(0) }
func (m *mockOp) GetDynamicGasPrice(basefee *big.Int) *big.Int      { return big.NewInt(0) }
func (m *mockOp) Pack() []byte                                      { return nil }
func (m *mockOp) PackForSignature() []byte                          { return nil }
func (m *mockOp) GetUserOpHash(ep common.Address, chainID *big.Int) common.Hash { return common.Hash{} }
func (m *mockOp) GetUserOpHashEIP712(domainSeparator [32]byte, typeHash [32]byte) common.Hash { return common.Hash{} }
func (m *mockOp) GetPaymasterSignature() []byte                     { return nil }
func (m *mockOp) MarshalJSON() ([]byte, error)                      { return nil, m.toMapErr }
func (m *mockOp) ToMap() (map[string]any, error) {
	if m.toMapErr != nil {
		return nil, m.toMapErr
	}
	return map[string]any{}, nil
}

func TestIsPrefundNotPaid(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"AA21 didn't pay prefund", errors.New("AA21 didn't pay prefund"), true},
		{"AA31 paymaster deposit too low", errors.New("AA31 paymaster deposit too low"), true},
		{"other error", errors.New("AA40 over verificationGasLimit"), false},
		{"nil error", nil, false},
		{"empty error", errors.New(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrefundNotPaid(tt.err); got != tt.want {
				t.Errorf("isPrefundNotPaid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidationOOG(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"AA40 over verificationGasLimit", errors.New("AA40 over verificationGasLimit"), true},
		{"AA41 too little verificationGas", errors.New("AA41 too little verificationGas"), true},
		{"AA51 prefund below actualGasCost", errors.New("AA51 prefund below actualGasCost"), true},
		{"AA13 initCode failed or OOG", errors.New("AA13 initCode failed or OOG"), true},
		{"AA23 reverted", errors.New("AA23 reverted"), true},
		{"AA33 reverted", errors.New("AA33 reverted"), true},
		{"return data out of bounds", errors.New("return data out of bounds"), true},
		{"validation OOG", errors.New("validation OOG"), true},
		{"AA21 didn't pay prefund", errors.New("AA21 didn't pay prefund"), false},
		{"execution OOG", errors.New("execution OOG"), false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidationOOG(tt.err); got != tt.want {
				t.Errorf("isValidationOOG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExecutionOOG(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"execution OOG", errors.New("execution OOG"), true},
		{"execution reverted", errors.New("execution reverted"), false},
		{"validation OOG", errors.New("validation OOG"), false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExecutionOOG(tt.err); got != tt.want {
				t.Errorf("isExecutionOOG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExecutionReverted(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"execution reverted", errors.New("execution reverted"), true},
		{"execution OOG", errors.New("execution OOG"), false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExecutionReverted(tt.err); got != tt.want {
				t.Errorf("isExecutionReverted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func v07OpData() map[string]any {
	return map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x1",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
}

func TestConvertToV06FromV07(t *testing.T) {
	op, err := v07.New(v07OpData())
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if v06Op.Sender != op.Sender {
		t.Errorf("Sender: got %s, want %s", v06Op.Sender.Hex(), op.Sender.Hex())
	}
	if v06Op.Nonce.Cmp(op.Nonce) != 0 {
		t.Errorf("Nonce: got %d, want %d", v06Op.Nonce, op.Nonce)
	}
	if v06Op.CallGasLimit.Cmp(op.CallGasLimit) != 0 {
		t.Errorf("CallGasLimit: got %d, want %d", v06Op.CallGasLimit, op.CallGasLimit)
	}
	if v06Op.VerificationGasLimit.Cmp(op.VerificationGasLimit) != 0 {
		t.Errorf("VerificationGasLimit: got %d, want %d", v06Op.VerificationGasLimit, op.VerificationGasLimit)
	}
	if v06Op.MaxFeePerGas.Cmp(op.MaxFeePerGas) != 0 {
		t.Errorf("MaxFeePerGas: got %d, want %d", v06Op.MaxFeePerGas, op.MaxFeePerGas)
	}
}

func TestConvertToV06FromV07WithFactory(t *testing.T) {
	data := v07OpData()
	data["factory"] = "0xe19e9755942bb0bd0cccce25b2591746b8a8250b"
	data["factoryData"] = "0x3bf2c3e7"

	op, err := v07.New(data)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if len(v06Op.InitCode) == 0 {
		t.Fatal("expected non-empty InitCode after factory conversion")
	}

	factory := common.BytesToAddress(v06Op.InitCode[:20])
	if !strings.EqualFold(factory.Hex(), "0xe19e9755942bb0bd0cccce25b2591746b8a8250b") {
		t.Errorf("expected factory address in InitCode, got %s", factory.Hex())
	}
}

func TestConvertToV06FromV07WithPaymaster(t *testing.T) {
	data := v07OpData()
	data["paymaster"] = "0x8b388a082f370d8ac2e2b3997e9151168bd09ff5"
	data["paymasterData"] = "0xdeadbeef"

	op, err := v07.New(data)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if len(v06Op.PaymasterAndData) == 0 {
		t.Fatal("expected non-empty PaymasterAndData after paymaster conversion")
	}

	pm := common.BytesToAddress(v06Op.PaymasterAndData[:20])
	if !strings.EqualFold(pm.Hex(), "0x8b388a082f370d8ac2e2b3997e9151168bd09ff5") {
		t.Errorf("expected paymaster address in PaymasterAndData, got %s", pm.Hex())
	}
}

func TestConvertToV06FromV08(t *testing.T) {
	data := v07OpData()
	op, err := v08.New(data)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if v06Op.Sender != op.Sender {
		t.Errorf("Sender mismatch: got %s, want %s", v06Op.Sender.Hex(), op.Sender.Hex())
	}
}

func TestConvertToV06FromV09(t *testing.T) {
	data := v07OpData()
	data["paymasterSignature"] = "0xdead"
	op, err := v09.New(data)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if v06Op.Sender != op.Sender {
		t.Errorf("Sender mismatch: got %s, want %s", v06Op.Sender.Hex(), op.Sender.Hex())
	}
}

func TestConvertToV06FromV06Passthrough(t *testing.T) {
	opData := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x1",
		"initCode":             "0x",
		"callData":             "0x80c5c7d0",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x",
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v06.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if v06Op.Sender != op.Sender {
		t.Errorf("Sender mismatch: got %s, want %s", v06Op.Sender.Hex(), op.Sender.Hex())
	}
}

func TestConvertToV06HandlesNilVersionedFields(t *testing.T) {
	op, err := v07.New(v07OpData())
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	if v06Op.GetPaymaster() != common.HexToAddress("0x") {
		t.Error("expected zero paymaster address after conversion")
	}
	if v06Op.GetFactory() != common.HexToAddress("0x") {
		t.Error("expected zero factory address after conversion")
	}
}

func TestConvertToV06ToMapError(t *testing.T) {
	mock := &mockOp{toMapErr: errors.New("toMap failed")}
	_, err := ConvertToV06(mock)
	if err == nil || err.Error() != "toMap failed" {
		t.Errorf("expected toMap failed error, got %v", err)
	}
}

func TestInterfaceMethods(t *testing.T) {
	var op userop.UserOperationInterface

	v06op := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x1",
		"initCode":             "0x",
		"callData":             "0x80c5c7d0",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x",
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}

	op, err := v06.New(v06op)
	if err != nil {
		t.Fatal(err)
	}

	if op.GetSender() == (common.Address{}) {
		t.Error("GetSender should return non-zero address")
	}
	if op.GetMaxFeePerGas().Cmp(big.NewInt(0)) <= 0 {
		t.Error("GetMaxFeePerGas should return positive value")
	}
	if op.GetPaymaster() != common.HexToAddress("0x") {
		t.Error("GetPaymaster should return zero for no paymaster")
	}
}

func TestConvertToV06ErrorHandling(t *testing.T) {
	op, err := v07.New(v07OpData())
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v06Op == nil {
		t.Error("expected non-nil result")
	}
}

func TestConvertToV06PassthroughV06WithInitCode(t *testing.T) {
	initCode := "0xe19e9755942bb0bd0cccce25b2591746b8a8250b3bf2c3e7"
	opData := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x1",
		"initCode":             initCode,
		"callData":             "0x80c5c7d0",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x",
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v06.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	expected, _ := hexutil.Decode(initCode)
	if len(v06Op.InitCode) != len(expected) {
		t.Errorf("InitCode length: got %d, want %d", len(v06Op.InitCode), len(expected))
	}
}

func TestConvertToV06PassthroughV06WithPaymasterAndData(t *testing.T) {
	pmAndData := "0x8b388a082f370d8ac2e2b3997e9151168bd09ff5deadbeef"
	opData := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x1",
		"initCode":             "0x",
		"callData":             "0x80c5c7d0",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     pmAndData,
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v06.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	v06Op, err := ConvertToV06(op)
	if err != nil {
		t.Fatal(err)
	}

	expected, _ := hexutil.Decode(pmAndData)
	if len(v06Op.PaymasterAndData) != len(expected) {
		t.Errorf("PaymasterAndData length: got %d, want %d", len(v06Op.PaymasterAndData), len(expected))
	}
}
