package simulation

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

// validateCreateRules enforces the contract-creation rules:
//   - OP-031: CREATE2 is allowed exactly once in the deployment phase, by the
//     factory, and only when the op carries initcode.
//   - OP-032: when the op has a factory (even unstaked), the sender contract
//     itself may use CREATE.
//   - EREP-060: a staked factory, or the sender when its factory is staked,
//     may use both CREATE and CREATE2 (but the sender only directly, not via
//     a utility contract).
//   - EREP-061: a staked factory may also use a utility contract that calls
//     CREATE, so factory usage is checked at the entity (subtree) level.
func validateCreateRules(op *userop.UserOperation, entities knownEntity) error {
	hasFactory := len(op.InitCode) > 0
	factoryIsStaked := entities["factory"].IsStaked

	for title, entity := range entities {
		createCount := entity.Info.Opcodes["CREATE"]
		create2Count := entity.Info.Opcodes[create2OpCode]
		if createCount == 0 && create2Count == 0 {
			continue
		}

		switch title {
		case "factory":
			// EREP-060/EREP-061: staked factories may create freely.
			if entity.IsStaked {
				continue
			}
			// The sender's constructor runs inside the factory's subtree, so
			// creations executed by the sender itself follow the account
			// rules: CREATE is fine with any factory (OP-032), CREATE2 would
			// need a staked one (EREP-060).
			factoryCreate, factoryCreate2 := createCount, create2Count
			if entity.Info.CreateOpcodes != nil {
				factoryCreate, factoryCreate2 = 0, 0
				for addr, counts := range entity.Info.CreateOpcodes {
					if addr == op.Sender {
						if counts[create2OpCode] > 0 {
							return fmt.Errorf("account uses banned opcode: %s", create2OpCode)
						}
						continue
					}
					factoryCreate += counts["CREATE"]
					factoryCreate2 += counts[create2OpCode]
				}
			}
			if factoryCreate > 0 {
				return fmt.Errorf("factory uses banned opcode: CREATE")
			}
			if factoryCreate2 > 1 || (factoryCreate2 > 0 && !hasFactory) {
				return fmt.Errorf("factory with too many %s", create2OpCode)
			}
		case "account":
			ownCreate, ownCreate2 := createCount, create2Count
			var nestedCreate, nestedCreate2 float64
			if entity.Info.CreateOpcodes != nil {
				ownCreate, ownCreate2 = 0, 0
				for addr, counts := range entity.Info.CreateOpcodes {
					if addr == op.Sender {
						ownCreate += counts["CREATE"]
						ownCreate2 += counts[create2OpCode]
					} else {
						nestedCreate += counts["CREATE"]
						nestedCreate2 += counts[create2OpCode]
					}
				}
			}
			// A utility contract creating on the account's behalf is always
			// banned, regardless of stake.
			if nestedCreate > 0 || (ownCreate > 0 && !hasFactory) {
				return fmt.Errorf("account uses banned opcode: CREATE")
			}
			if nestedCreate2 > 0 || (ownCreate2 > 0 && (!hasFactory || !factoryIsStaked)) {
				return fmt.Errorf("account uses banned opcode: %s", create2OpCode)
			}
		default:
			opName := "CREATE"
			if create2Count > 0 {
				opName = create2OpCode
			}
			return fmt.Errorf("%s uses banned opcode: %s", title, opName)
		}
	}

	return nil
}

// ValidateEntityRules enforces the per-entity ERC-7562 rules that follow the
// banned opcode checks: the contract-creation rules (OP-031/OP-032 and the
// staked-factory EREP-060/EREP-061 exceptions) and the storage slot access
// rules. It is shared by the v0.6–v0.9 trace validation paths and
// returns any alt-mempool ids collected by the storage validator.
func ValidateEntityRules(
	version userop.Version,
	op *userop.UserOperation,
	entryPoint common.Address,
	isRIP7212Supported bool,
	alt *altmempools.Directory,
	stakes EntityStakes,
	res *tracer.BundlerCollectorReturn,
	entities knownEntity,
) ([]string, error) {
	if err := validateCreateRules(op, entities); err != nil {
		return nil, err
	}

	altMempoolIds := []string{}
	slotsByEntity := NewStorageSlotsByEntity(stakes, res.Keccak)
	for title, entity := range entities {
		v := &storageSlotsValidator{
			Op:                    op,
			EntryPoint:            entryPoint,
			IsRIP7212Supported:    isRIP7212Supported,
			AltMempools:           alt,
			SenderSlots:           slotsByEntity[op.Sender],
			FactoryIsStaked:       entities["factory"].IsStaked,
			LegacySTO022:          version == userop.V0_6 || version == userop.V0_7,
			EntityName:            title,
			EntityAddr:            entity.Address,
			EntityAccessMap:       entity.Info.Access,
			EntityContractSizeMap: entity.Info.ContractSize,
			EntitySlots:           slotsByEntity[entity.Address],
			EntityIsStaked:        entity.IsStaked,
		}
		ids, err := v.Process()
		if err != nil {
			return nil, err
		}
		altMempoolIds = append(altMempoolIds, ids...)
	}

	return altMempoolIds, nil
}
