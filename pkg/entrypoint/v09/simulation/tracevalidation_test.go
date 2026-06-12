package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint/methods"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

func validMockOpV09() *userop.UserOperation {
	return &userop.UserOperation{
		Sender:               common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                big.NewInt(0),
		InitCode:             common.Hex2Bytes("e19e9755942bb0bd0cccce25b1742596b8a8250b3bf2c3e7"),
		CallData:             common.Hex2Bytes("80c5c7d0"),
		CallGasLimit:         big.NewInt(30000),
		VerificationGasLimit: big.NewInt(100000),
		PreVerificationGas:   big.NewInt(50000),
		MaxFeePerGas:         big.NewInt(2000000000),
		MaxPriorityFeePerGas: big.NewInt(1000000000),
		PaymasterAndData:     []byte{},
		Signature:            common.Hex2Bytes("a925dcc5"),
	}
}

func validTraceResultV09(op *userop.UserOperation) *tracer.BundlerCollectorReturn {
	return &tracer.BundlerCollectorReturn{
		CallsFromEntryPoint: []tracer.CallFromEntryPointInfo{
			{
				TopLevelMethodSig:     common.Hex2Bytes("b61d27f6"),
				TopLevelTargetAddress: op.Sender,
				Opcodes:               tracer.Counts{},
				Access:                tracer.AccessMap{},
				ContractSize:          tracer.ContractSizeMap{},
				ExtCodeAccessInfo:     tracer.ExtCodeAccessInfoMap{},
				OOG:                   false,
			},
		},
		Calls:  []tracer.CallInfo{},
		Keccak: []string{},
		Logs:   []tracer.LogInfo{},
	}
}

func mockStakesV09() simutils.EntityStakes {
	return simutils.EntityStakes{
		common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"): {
			Deposit: big.NewInt(1000000),
			Staked:  true,
			Stake:   big.NewInt(1000000),
		},
	}
}

func TestProcessTraceResult_SuccessV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

func TestProcessTraceResult_OOGV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].OOG = true

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (OOG)")
	}
}

func TestProcessTraceResult_BannedOpcodeV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"GASPRICE": 1}

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (banned opcode)")
	}
}

func TestProcessTraceResult_UnstakedBannedOpcodeV09(t *testing.T) {
	op := validMockOpV09()
	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(0), Staked: false, Stake: big.NewInt(0)},
	}
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"BALANCE": 1}

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (unstaked banned opcode)")
	}
}

func TestProcessTraceResult_ExtCodeAccessEntryPointV09(t *testing.T) {
	op := validMockOpV09()
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026dCA1a")
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].ExtCodeAccessInfo = tracer.ExtCodeAccessInfoMap{
		ep: "EXTCODESIZE",
	}

	_, err := ProcessTraceResult(op, ep, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (EXTCODE access to EntryPoint)")
	}
}

func TestProcessTraceResult_BalanceOfCallV09(t *testing.T) {
	op := validMockOpV09()
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026dCA1a")
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.Calls = []tracer.CallInfo{
		{
			Type:   "CALL",
			From:   op.Sender,
			To:     ep,
			Method: methods.BalanceOfSelector,
			Value:  "0x0",
			Gas:    10000,
		},
		{
			Type:    "RETURN",
			GasUsed: 100,
			Data:    "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	_, err := ProcessTraceResult(op, ep, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (balanceOf on EntryPoint)")
	}
}

func TestProcessTraceResult_ForbiddenValueTransferV09(t *testing.T) {
	op := validMockOpV09()
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026dCA1a")
	recipient := common.HexToAddress("0x1111111111111111111111111111111111111111")
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.Calls = []tracer.CallInfo{
		{
			Type:   "CALL",
			From:   op.Sender,
			To:     recipient,
			Method: "0x00000000",
			Value:  "0x1",
			Gas:    10000,
		},
		{
			Type:    "RETURN",
			GasUsed: 100,
			Data:    "0x",
		},
	}

	_, err := ProcessTraceResult(op, ep, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (forbidden value transfer)")
	}
}

func TestProcessTraceResult_UnstakedPaymasterWithContextV09(t *testing.T) {
	paymaster := common.HexToAddress("0x2222222222222222222222222222222222222222")
	op := validMockOpV09()
	op.PaymasterAndData = paymaster.Bytes()

	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
		paymaster: {Deposit: big.NewInt(1000000), Staked: false, Stake: big.NewInt(0)},
	}

	res := validTraceResultV09(op)
	res.CallsFromEntryPoint = append(res.CallsFromEntryPoint, tracer.CallFromEntryPointInfo{
		TopLevelMethodSig:     common.Hex2Bytes("00000000"),
		TopLevelTargetAddress: paymaster,
		Opcodes:               tracer.Counts{},
		Access:                tracer.AccessMap{},
		ContractSize:          tracer.ContractSizeMap{},
		ExtCodeAccessInfo:     tracer.ExtCodeAccessInfoMap{},
		OOG:                   false,
	})

	res.Calls = []tracer.CallInfo{
		{
			Type:   "CALL",
			From:   common.HexToAddress("0x"),
			To:     paymaster,
			Method: methods.PackedValidatePaymasterUserOpSelector,
			Value:  "0x0",
			Gas:    50000,
		},
		{
			Type:    "RETURN",
			GasUsed: 1000,
			Data:    "0x0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (unstaked paymaster with context)")
	}
}

func TestProcessTraceResult_CollectsTouchedContractsV09(t *testing.T) {
	factory := common.HexToAddress("0x3333333333333333333333333333333333333333")
	deployed := common.HexToAddress("0x4444444444444444444444444444444444444444")
	op := validMockOpV09()
	op.InitCode = append(factory.Bytes(), common.Hex2Bytes("deadbeef")...)

	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
		factory:   {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
	}

	res := validTraceResultV09(op)
	res.CallsFromEntryPoint = append(res.CallsFromEntryPoint, tracer.CallFromEntryPointInfo{
		TopLevelMethodSig:     common.Hex2Bytes("00000000"),
		TopLevelTargetAddress: factory,
		Opcodes:               tracer.Counts{},
		Access:                tracer.AccessMap{},
		ContractSize: tracer.ContractSizeMap{
			deployed: {ContractSize: 100, Opcode: "EXTCODESIZE"},
		},
		ExtCodeAccessInfo:     tracer.ExtCodeAccessInfoMap{},
		OOG:                   false,
	})

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if len(out.TouchedContracts) == 0 {
		t.Fatal("ProcessTraceResult: got 0 touched contracts, want > 0")
	}
}

func TestProcessTraceResult_EIP7702DelegateV09(t *testing.T) {
	delegate := common.HexToAddress("0x5555555555555555555555555555555555555555")
	op := validMockOpV09()

	stakes := simutils.EntityStakes{
		op.Sender:  {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
		delegate: {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
	}

	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].EIP7702Auth = true
	// Add delegate trace so NewKnownEntity creates the delegate entity
	res.CallsFromEntryPoint = append(res.CallsFromEntryPoint, tracer.CallFromEntryPointInfo{
		TopLevelMethodSig:     common.Hex2Bytes("b61d27f6"),
		TopLevelTargetAddress: delegate,
		Opcodes:               tracer.Counts{},
		Access:                tracer.AccessMap{},
		ContractSize:          tracer.ContractSizeMap{},
		ExtCodeAccessInfo:     tracer.ExtCodeAccessInfoMap{},
		OOG:                   false,
	})

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, delegate)
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

func TestProcessTraceResult_EIP7702DelegateBannedOpcodeV09(t *testing.T) {
	delegate := common.HexToAddress("0x5555555555555555555555555555555555555555")
	op := validMockOpV09()

	stakes := simutils.EntityStakes{
		op.Sender:  {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
		delegate: {Deposit: big.NewInt(0), Staked: false, Stake: big.NewInt(0)},
	}

	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].EIP7702Auth = true
	res.CallsFromEntryPoint = append(res.CallsFromEntryPoint, tracer.CallFromEntryPointInfo{
		TopLevelMethodSig:     common.Hex2Bytes("b61d27f6"),
		TopLevelTargetAddress: delegate,
		Opcodes:               tracer.Counts{"BALANCE": 1},
		Access:                tracer.AccessMap{},
		ContractSize:          tracer.ContractSizeMap{},
		ExtCodeAccessInfo:     tracer.ExtCodeAccessInfoMap{},
		OOG:                   false,
	})

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, delegate)
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (unstaked delegate banned opcode)")
	}
}
