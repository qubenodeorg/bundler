package v09

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// EntryPointSimulations is the v0.9 off-chain simulation variant of the
// EntryPoint (eth-infinitism EntryPointSimulations.sol). It is never deployed
// on-chain: its deployedBytecode replaces the EntryPoint's code via state
// override during eth_call/debug_traceCall. Unlike handleOps, its
// simulateHandleOp() does not revert with AA24 on signature failure, which is
// what allows gas estimation with dummy signatures (and with gas fields that
// diverge from the values the user signed over).
//
//go:embed entrypoint_simulations.json
var entryPointSimulationsArtifact []byte

type simulationsArtifact struct {
	ABI              json.RawMessage `json:"abi"`
	DeployedBytecode string          `json:"deployedBytecode"`
}

var (
	simsOnce     sync.Once
	simsABI      abi.ABI
	simsBytecode hexutil.Bytes
	simsErr      error
)

func loadSimulations() {
	var art simulationsArtifact
	if simsErr = json.Unmarshal(entryPointSimulationsArtifact, &art); simsErr != nil {
		simsErr = fmt.Errorf("entrypoint simulations artifact: %w", simsErr)
		return
	}
	simsABI, simsErr = abi.JSON(bytes.NewReader(art.ABI))
	if simsErr != nil {
		simsErr = fmt.Errorf("entrypoint simulations abi: %w", simsErr)
		return
	}
	simsBytecode, simsErr = hexutil.Decode(art.DeployedBytecode)
	if simsErr != nil {
		simsErr = fmt.Errorf("entrypoint simulations bytecode: %w", simsErr)
	}
}

// SimulationsDeployedBytecode returns the EntryPointSimulations runtime code
// for use as a state override at the EntryPoint address.
func SimulationsDeployedBytecode() (hexutil.Bytes, error) {
	simsOnce.Do(loadSimulations)
	return simsBytecode, simsErr
}

// PackSimulateHandleOp encodes a call to
// EntryPointSimulations.simulateHandleOp(op, target, targetCallData).
func PackSimulateHandleOp(op PackedUserOperation, target common.Address, targetCallData []byte) ([]byte, error) {
	simsOnce.Do(loadSimulations)
	if simsErr != nil {
		return nil, simsErr
	}
	if targetCallData == nil {
		targetCallData = []byte{}
	}
	return simsABI.Pack("simulateHandleOp", op, target, targetCallData)
}
