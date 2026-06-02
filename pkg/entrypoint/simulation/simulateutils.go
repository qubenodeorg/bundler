package simulation

import (
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint"
)

// EntityStakes provides a mapping for encountered entity addresses and their stake info on the EntryPoint.
type EntityStakes map[common.Address]*entrypoint.IStakeManagerDepositInfo

var (
	// Only one create2 opcode is allowed if these two conditions are met:
	// 	1. op.initcode.length != 0
	// 	2. During account simulation (i.e. before markerOpCode)
	Create2OpCode = "CREATE2"

	// BannedOpCodeSet lists opcodes not allowed during simulation for depth > 1.
	BannedOpCodeSet = mapset.NewSet(
		"GASPRICE",
		"GASLIMIT",
		"DIFFICULTY",
		"TIMESTAMP",
		"BASEFEE",
		"BLOCKHASH",
		"NUMBER",
		"ORIGIN",
		"GAS",
		"CREATE",
		"COINBASE",
		"SELFDESTRUCT",
		"TSTORE",
		"TLOAD",
	)

	// BannedUnstakedOpCodeSet lists opcodes not allowed for unstaked entities.
	BannedUnstakedOpCodeSet = mapset.NewSet(
		"SELFBALANCE",
		"BALANCE",
	)

	RevertOpCode = "REVERT"
	ReturnOpCode = "RETURN"

	// Precompiled contract that performs secp256r1 signature verification. See
	// https://github.com/ethereum/RIPs/blob/master/RIPS/rip-7212.md
	Rip7212Precompile = common.HexToAddress("0x100")
)

// BannedOpCodes returns the set of banned opcodes.
func BannedOpCodes() mapset.Set[string] {
	return BannedOpCodeSet
}

// BannedUnstakedOpCodes returns the set of opcodes banned for unstaked entities.
func BannedUnstakedOpCodes() mapset.Set[string] {
	return BannedUnstakedOpCodeSet
}

// Backward-compatible aliases
var (
	create2OpCode       = Create2OpCode
	bannedOpCodes       = BannedOpCodeSet
	bannedUnstakedOpCodes = BannedUnstakedOpCodeSet
	revertOpCode        = RevertOpCode
	returnOpCode        = ReturnOpCode
	rip7212precompile   = Rip7212Precompile
)
