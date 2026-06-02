package checks

import (
	"math/big"

	"github.com/zarbanio/bundler/pkg/userop"
)

// partialNilOp returns nil only for specified getters, and can override
// specific getter return values for testing.
type partialNilOp struct {
	userop.UserOperationInterface
	nilNonce         bool
	nilCallGasLimit  bool
	nilVG            bool
	nilPVG           bool
	nilMaxFee        bool
	nilMaxPriority   bool
	nilPaymasterVGL  bool
	nilPaymasterPOGL bool
	nilMaxGasAvail   bool
	// Override values for testing (when non-nil, returned instead of delegating)
	overridePaymasterVGL  *big.Int
	overridePaymasterPOGL *big.Int
	overrideMaxGasAvail   *big.Int
	overridePaymasterData []byte
}

func (p *partialNilOp) GetNonce() *big.Int {
	if p.nilNonce {
		return nil
	}
	return p.UserOperationInterface.GetNonce()
}

func (p *partialNilOp) GetCallGasLimit() *big.Int {
	if p.nilCallGasLimit {
		return nil
	}
	return p.UserOperationInterface.GetCallGasLimit()
}

func (p *partialNilOp) GetVerificationGasLimit() *big.Int {
	if p.nilVG {
		return nil
	}
	return p.UserOperationInterface.GetVerificationGasLimit()
}

func (p *partialNilOp) GetPreVerificationGas() *big.Int {
	if p.nilPVG {
		return nil
	}
	return p.UserOperationInterface.GetPreVerificationGas()
}

func (p *partialNilOp) GetMaxFeePerGas() *big.Int {
	if p.nilMaxFee {
		return nil
	}
	return p.UserOperationInterface.GetMaxFeePerGas()
}

func (p *partialNilOp) GetMaxPriorityFeePerGas() *big.Int {
	if p.nilMaxPriority {
		return nil
	}
	return p.UserOperationInterface.GetMaxPriorityFeePerGas()
}

func (p *partialNilOp) GetPaymasterVerificationGasLimit() *big.Int {
	if p.nilPaymasterVGL {
		return nil
	}
	if p.overridePaymasterVGL != nil {
		return p.overridePaymasterVGL
	}
	return p.UserOperationInterface.GetPaymasterVerificationGasLimit()
}

func (p *partialNilOp) GetPaymasterPostOpGasLimit() *big.Int {
	if p.nilPaymasterPOGL {
		return nil
	}
	if p.overridePaymasterPOGL != nil {
		return p.overridePaymasterPOGL
	}
	return p.UserOperationInterface.GetPaymasterPostOpGasLimit()
}

func (p *partialNilOp) GetMaxGasAvailable() *big.Int {
	if p.nilMaxGasAvail {
		return nil
	}
	if p.overrideMaxGasAvail != nil {
		return p.overrideMaxGasAvail
	}
	return p.UserOperationInterface.GetMaxGasAvailable()
}

func (p *partialNilOp) GetPaymasterData() []byte {
	if p.overridePaymasterData != nil {
		return p.overridePaymasterData
	}
	return p.UserOperationInterface.GetPaymasterData()
}
