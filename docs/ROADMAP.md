# ERC-4337 v0.9 Migration Roadmap

## Executive Summary

**Objective**: Migrate qubenode-bundler from ERC-4337 v0.6 to v0.9 with backward compatibility
**Timeline**: 30 days
**Scope**: Ethereum mainnet + Arbitrum One
**Strategy**: Big bang migration with configurable version support
**Developer**: Solo

---

## Current State

- **Version**: ERC-4337 v0.6
- **EntryPoint**: `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789`
- **Supported Chains**: Ethereum, Optimism, Arbitrum, Base, Lyra, Ancient8
- **Architecture**: Modular middleware (Client → Mempool → Bundler)

## Target State

- **Version**: ERC-4337 v0.9
- **EntryPoint**: `0x433709009B8330FDa32311DF1C2AFA402eD8D009`
- **Supported Chains**: Ethereum mainnet, Arbitrum One (v0.9 compatible)
- **New Features**: EIP-7702, paymaster signatures, block number validity, ERC-712 hashes
- **Backward Compatibility**: Configurable v0.6/v0.9 support

---

## High-Level Changes

### Breaking Changes (v0.7)

1. **UserOperation Struct**
   - `InitCode []byte` → `Factory common.Address` + `FactoryData []byte`
   - `PaymasterAndData []byte` → `Paymaster common.Address` + `PaymasterData []byte` + `PaymasterVerificationGasLimit` + `PaymasterPostOpGasLimit`
   - Hash calculation changes (no more 3x verificationGas multiplier)

2. **EntryPoint Contract**
   - Simulation methods removed from deployed contract (off-chain only)
   - New error codes (AA20-AA95)
   - `ReturnInfo` structure updated
   - `delegateAndRevert()` helper added

3. **Gas Model**
   - 10% unused gas penalty
   - Paymaster gas limits separated
   - Prefund calculation updated

4. **Validation Rules (ERC-7562)**
   - OP-013: Ban unassigned opcodes
   - OP-051: Allow EXTCODESIZE ISZERO for depositTo()
   - OP-054: Forbid EntryPoint calls except depositTo()
   - STO-033: Staked entity storage read
   - EREP-020: Staked factory accountability
   - OP-070: Transient storage rules

### New Features (v0.8)

5. **EIP-7702 Support**
   - Native authorization in EntryPoint
   - `eip7702Auth` parameter
   - Delegation address in userOpHash
   - Simple7702Account contract

6. **ERC-712 Hash Support**
   - Typed data signing compatible
   - Better hardware wallet support

7. **Gas Penalty Threshold**
   - No penalty if unused gas < 40,000

8. **Validation Relaxations**
   - CREATE/CREATE2 allowed during deployment
   - All staked entities get storage access

### New Features (v0.9)

9. **Paymaster Signature**
   - `paymasterSignature` field (excluded from userOpHash)
   - Parallel signing support

10. **Block Number Validity**
    - `validAfter`/`validUntil` can specify block numbers

11. **InitCode Handling**
    - Silently ignored if account exists
    - `IgnoredInitCode` event

12. **Execution Hash Query**
    - `getCurrentUserOpHash()` during execution

---

## Implementation Plan

### Week 1: Foundation (Days 1-7)

#### Day 1: Project Setup
- [ ] Download v0.9 EntryPoint ABI and contract source from eth-infinitism/account-abstraction
- [ ] Set up local test environment with v0.9 contracts
- [ ] Create migration tracking document
- [ ] Review current codebase for all v0.6 dependencies

#### Days 2-3: UserOperation Struct Refactor

**Approach**: Create versioned structs to preserve backward compatibility

- [ ] Create `pkg/userop/v06/object.go`:
  - Move current v0.6 `UserOperation` struct (keep as-is for backward compat)
  - Keep `InitCode []byte` and `PaymasterAndData []byte`
  - Keep v0.6 hash calculation (`verificationGasLimit * 3` for paymasters)
- [ ] Create `pkg/userop/v09/object.go`:
  - New v0.9 `UserOperation` struct with:
    - `Factory common.Address` + `FactoryData []byte`
    - `Paymaster common.Address` + `PaymasterData []byte`
    - `PaymasterVerificationGasLimit *big.Int`
    - `PaymasterPostOpGasLimit *big.Int`
  - New `EIP7702Auth` field
  - New `PaymasterSignature` field
  - v0.9 hash calculation (no 3x multiplier)
- [ ] Create `pkg/userop/interface.go`:
  - Define `UserOperationInterface` with common methods
  - Version-aware factory function: `NewUserOperation(version, data)`
- [ ] Update `pkg/userop/parse.go`:
  - Version-aware parsing based on detected EntryPoint version
  - Parse v0.6 JSON-RPC requests into v06 struct
  - Parse v0.9 JSON-RPC requests into v09 struct
  - Support both field naming conventions during transition
- [ ] Create version-specific ABI types:
  - `pkg/userop/v06/types.go`: v0.6 `UserOpPrimitives`, `UserOpType`
  - `pkg/userop/v09/types.go`: v0.9 `UserOpPrimitives`, `UserOpType`

**Deliverable**: Versioned data model compiles, both structs usable

#### Day 4: Version Detection System

**Approach**: Address-based detection (EntryPoint contracts don't expose version())

- [ ] Add `EntryPointVersion` auto-detection:
  - **Primary**: Map known EntryPoint addresses to versions:
    - `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` → v0.6
    - `0x0000000071727De22E5E9d8BAf0edAc6f37da032` → v0.7
    - `0x4337084d9e255ff0702461cf8895ce9e3b5ff108` → v0.8
    - `0x433709009B8330FDa32311DF1C2AFA402eD8D009` → v0.9
  - **Fallback**: Bytecode selector analysis (check for v0.9-specific method selectors like `getCurrentUserOpHash()`)
  - Support explicit `ERC4337_BUNDLER_ENTRYPOINT_VERSION` env var as override
- [ ] Create version constants: `V0_6`, `V0_7`, `V0_8`, `V0_9`
- [ ] Add `Version` field to `internal/config/Values`
- [ ] Update `GetValues()` to auto-detect from `SupportedEntryPoints` addresses
- [ ] Add version-aware interfaces where needed

**Deliverable**: Version detection works

#### Days 5-6: EntryPoint Bindings Regeneration

**Approach**: Keep v0.6 bindings, add v0.9 in separate package

- [ ] Keep existing `pkg/entrypoint/bindings.go` (v0.6 bindings untouched)
- [ ] Create `pkg/entrypoint/v09/bindings.go` from v0.9 ABI using `abigen`
- [ ] Update `Makefile` target `generate-entrypoint-pkg` to generate both versions
- [ ] Create version-specific revert parsing:
  - Keep `pkg/entrypoint/reverts/` for v0.6 error codes
  - Create `pkg/entrypoint/v09/reverts/` for new error codes (AA20-AA95)
- [ ] Create version-specific result parsing:
  - `pkg/entrypoint/v09/reverts/validationresult.go` for new `ReturnInfo` structure
  - `pkg/entrypoint/v09/reverts/executionresult.go` for updated fields
  - `pkg/entrypoint/v09/reverts/failedop.go` for new error formats
- [ ] Create version-specific event signatures:
  - Keep existing events in `pkg/entrypoint/filter/` (v0.6)
  - Add `EIP7702AccountInitialized` and `IgnoredInitCode` to `pkg/entrypoint/v09/filter/`
  - Update `UserOperationEvent` signature in v09 package

**Deliverable**: EntryPoint bindings compile

#### Day 7: Gas Model Foundation
- [ ] Update `pkg/gas/types.go`:
  - Add `PaymasterVerificationGasLimit *big.Int`
  - Add `PaymasterPostOpGasLimit *big.Int`
  - Keep `VerificationGas` for backward compatibility
- [ ] Update `pkg/gas/estimate.go`:
  - Binary search logic for new gas fields
  - Error code matching for v0.9 errors
- [ ] Update `pkg/gas/overhead.go`:
  - `CalcCallDataCost()` for new struct size
  - `CalcPerUserOpCost()` recalibration
- [ ] Implement 10% unused gas penalty (v0.7)
- [ ] Implement 40,000 gas penalty threshold (v0.8)

**Deliverable**: Gas calculations compile

---

### Week 2: Validation & Simulation (Days 8-14)

#### Days 8-10: Validation Rules Update
- [ ] Update `pkg/modules/checks/initcode.go`:
  - `ValidateInitCode()` → `ValidateFactory()` and `ValidateFactoryData()`
  - Remove nonce==0 requirement for initCode (v0.9)
- [ ] Update `pkg/modules/checks/paymaster.go`:
  - `ValidatePaymasterAndData()` → `ValidatePaymaster()` + `ValidatePaymasterData()`
  - Add `ValidatePaymasterVerificationGasLimit()`
  - Add `ValidatePaymasterPostOpGasLimit()`
  - Handle zero-address paymaster rejection (v0.8)
- [ ] Update `pkg/modules/checks/verificationgas.go`:
  - Validate paymaster gas limits
  - Update max verification gas checks
- [ ] Update `pkg/modules/checks/gas.go`:
  - `ValidateGasAvailable()` for new prefund calculation
- [ ] Update `pkg/modules/checks/standalone.go`:
  - Call new validation functions
  - Version-aware validation switching
- [ ] Implement ERC-7562 validation rules:
  - OP-013: Ban unassigned opcodes
  - OP-051: Allow EXTCODESIZE ISZERO
  - OP-054: Forbid EntryPoint calls except depositTo()
  - STO-033: Staked entity storage read
  - EREP-020: Staked factory accountability
  - UREP-*: Unstaked reputation rules
  - ALT-*: Alternative mempool rules
  - OP-070: Transient storage rules (TSTORE/TLOAD)
- [ ] Remove on-chain `simulateValidation` calls (v0.7)
- [ ] Add `delegateAndRevert()` support for networks without state overrides

**Deliverable**: Validation module compiles

#### Days 11-12: Tracer Updates
- [x] Update `pkg/tracer/BundlerCollectorTracer.js`:
  - Handle v0.7 EntryPoint internal function calls
  - Handle v0.8 EntryPoint internal function calls
  - Handle v0.9 EntryPoint internal function calls
  - Track new validation flow
  - Support TSTORE/TLOAD opcodes
- [x] Update `pkg/tracer/BundlerExecutionTracer.js`:
  - New execution trace format
  - Capture `getCurrentUserOpHash()` calls
- [x] Update `pkg/tracer/types.go`:
  - New trace return structs
  - EIP-7702 trace data
- [x] Update `pkg/tracer/load.go`:
  - Version-aware tracer loading
- [x] Test tracers against v0.7 EntryPoint locally
- [x] Test tracers against v0.8 EntryPoint locally
- [x] Test tracers against v0.9 EntryPoint locally

**Deliverable**: Tracers work with v0.7, v0.8, v0.9

#### Days 13-14: Simulation & Execution

**Approach**: Create v0.9-specific simulation packages, keep v0.6 intact

- [ ] Create `pkg/entrypoint/v09/simulation/simulatevalidation.go`:
  - Off-chain simulation only (simulateValidation removed from on-chain in v0.7)
  - Handle new error codes (AA20-AA95)
  - Accept `userop/v09.UserOperation` struct
- [ ] Create `pkg/entrypoint/v09/simulation/tracevalidation.go`:
  - New trace format for v0.9 EntryPoint internals
- [ ] Create `pkg/entrypoint/v09/execution/simulate.go`:
  - Pass `userop/v09.UserOperation` struct
  - Handle `executeUserOp()` (v0.7 feature)
- [ ] Create `pkg/entrypoint/v09/execution/trace.go`:
  - Execution tracer integration for v0.9
- [ ] Create `pkg/entrypoint/v09/methods/handleops.go`:
  - v0.9 UserOperation tuple type
  - `toAbiType()` mapping for v09 fields
- [ ] Create `pkg/entrypoint/v09/transaction/handleops.go`:
  - Version-aware transaction building
  - Handle both v0.6 and v0.9 UserOperation structs
- [ ] Create `pkg/entrypoint/v09/transaction/gas.go`:
  - Gas estimation for v0.9 struct
- [ ] Keep existing `pkg/entrypoint/simulation/` and `pkg/entrypoint/execution/` (v0.6) untouched

**Deliverable**: Simulation and execution compile for both versions

---

### Week 3: Features & Integration (Days 15-21)

#### Days 15-17: EIP-7702 Support
- [ ] Add `EIP7702Auth` field to UserOperation struct
- [ ] Update `pkg/client/client.go`:
  - Accept `eip7702Auth` parameter in `eth_sendUserOperation`
  - Include delegation address in userOpHash calculation
- [ ] Update `pkg/modules/checks/standalone.go`:
  - Validate EIP-7702 authorization
  - Add ERC-7562 AUTH validation category
- [ ] Update `pkg/entrypoint/filter/event.go`:
  - Handle `EIP7702AccountInitialized` event
- [ ] Update `pkg/modules/entities/`:
  - Reputation tracking for 7702 accounts
- [ ] Add `Simple7702Account` contract type support
- [ ] Update `pkg/entrypoint/simulation/`:
  - Check delegation address correctness

**Deliverable**: EIP-7702 flow works

#### Days 18-19: v0.8/v0.9 Features
- [ ] Implement ERC-712 hash support:
  - Update `GetUserOpHash()` for ERC-712 typed data
  - Support both legacy and ERC-712 formats
- [ ] Implement paymaster signature field:
  - Add `paymasterSignature` to UserOperation
  - Exclude from userOpHash calculation
  - Update `pm_sponsorUserOperation()` to return signature
  - Validate paymaster signature during simulation
- [ ] Implement block number validity:
  - Detect block number mode (high bit set)
  - Validate against current block number
- [ ] Update initCode handling (v0.9):
  - Allow initCode with existing accounts
  - Handle `IgnoredInitCode` event
- [ ] Add `getCurrentUserOpHash()` support:
  - Update execution tracer
  - Document usage

**Deliverable**: All v0.9 features implemented

#### Days 20-21: Client & Bundler Integration
- [ ] Update `pkg/client/client.go`:
  - Version-aware `SendUserOperation()`
  - Version-aware `EstimateUserOperationGas()`
  - New GasEstimates response format
  - Update `GetUserOperationReceipt()` for new events
  - Update `GetUserOperationByHash()` for new struct
- [ ] Update `pkg/client/rpc.go`:
  - Version-aware RPC adapters
- [ ] Update `pkg/handlers/payg/handler.go`:
  - Generate paymaster signatures
  - Return `paymasterSignature` in response
- [ ] Update `pkg/bundler/bundler.go`:
  - Version-aware batch building
  - Handle new gas fields in batch processing
- [ ] Update `pkg/modules/relay/relayer.go`:
  - Version-aware transaction building
  - Handle v0.9 UserOperation ABI
- [ ] Update `pkg/modules/builder/builder.go`:
  - Version-aware bundle building

**Deliverable**: Client and bundler compile

---

### Week 4: Testing & Polish (Days 22-30)

#### Days 22-24: Unit Testing
- [ ] Write tests for new UserOperation struct
- [ ] Write tests for version detection system
- [ ] Write tests for new gas calculations
- [ ] Write tests for new validation rules
- [ ] Write tests for EIP-7702 flow
- [ ] Write tests for paymaster signature flow
- [ ] Write tests for block number validity
- [ ] Update existing tests for new struct
- [ ] Run `go test ./...` and fix failures
- [ ] Target >80% test coverage

**Deliverable**: All unit tests pass

#### Days 25-27: E2E Testing
- [ ] Update E2E test suite:
  - Deploy v0.9 EntryPoint locally
  - Update test contracts to v0.9
  - Update test transactions for new struct
- [ ] Test on Ethereum (local):
  - `eth_sendUserOperation`
  - `eth_estimateUserOperationGas`
  - `eth_getUserOperationReceipt`
  - EIP-7702 flow
  - Paymaster signature flow
- [ ] Test on Arbitrum Sepolia:
  - Deploy test contracts
  - Test with Arbitrum gas calculations
  - Verify L2-specific behavior
- [ ] Fix E2E test failures

**Deliverable**: E2E tests pass

#### Days 28-29: Compliance Testing
- [ ] Update compliance test references:
  - Use bundler-spec-tests v0.9
  - Update contract references
- [ ] Run compliance tests:
  - Validation rules
  - Gas estimation
  - Bundle building
- [ ] Fix compliance test failures
- [ ] Update CI workflows:
  - `.github/workflows/compliance.yml`
  - `.github/workflows/e2e.yml`
  - Use v0.9 contracts and EntryPoint

**Deliverable**: Compliance tests pass

#### Day 30: Documentation & Release
- [ ] Update `docs/API.md`:
  - New field names in UserOperation
  - New `eip7702Auth` parameter
  - New `paymasterSignature` field
  - Version-aware endpoints
- [ ] Update `docs/CONFIGURATION.md`:
  - New env vars: `ERC4337_BUNDLER_ENTRYPOINT_VERSION`
  - Version detection behavior
  - Migration guide
- [ ] Update `docs/ARCHITECTURE.md`:
  - Version-aware architecture
  - EIP-7702 flow diagrams
  - Paymaster signature flow
- [ ] Update `docs/MODULES.md`:
  - Version-aware modules
  - New validation rules
- [ ] Update `README.md`:
  - v0.9 support announcement
  - Migration instructions
  - Breaking changes notice
- [ ] Create `MIGRATION.md`:
  - Step-by-step migration guide for users
  - Breaking changes list
  - Configuration changes
- [ ] Final code review and cleanup
- [ ] Create PR with detailed description

**Deliverable**: PR ready for review

---

## Configuration Changes

### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_ENTRYPOINT_VERSION` | `auto` | EntryPoint version: `auto`, `v0.6`, `v0.7`, `v0.8`, `v0.9` |

### Auto-Detection Logic

```
if version == "auto":
    if entryPoint == "0x5FF137D4...":
        version = "v0.6"
    elif entryPoint == "0x0000000071...":
        version = "v0.7"
    elif entryPoint == "0x4337084d9e...":
        version = "v0.8"
    elif entryPoint == "0x433709009B...":
        version = "v0.9"
    else:
        # Try to detect via supportsInterface or version() call
        version = detectViaContractCall()
```

### Updated Defaults

| Variable | Old Default | New Default | Notes |
|----------|-------------|-------------|-------|
| `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` | `0x5FF137D4...` | `0x433709009B...` | v0.9 EntryPoint |

---

## Files Requiring Modification

### Critical (P0) - New Files

| File | Changes |
|------|---------|
| `pkg/userop/v06/object.go` | Move current v0.6 struct (keep intact) |
| `pkg/userop/v09/object.go` | **NEW**: v0.9 struct with Factory, Paymaster fields |
| `pkg/userop/interface.go` | **NEW**: Version-aware interface and factory |
| `pkg/userop/parse.go` | Version-aware parsing (both v0.6 and v0.9) |
| `pkg/entrypoint/v09/bindings.go` | **NEW**: v0.9 EntryPoint bindings |
| `pkg/entrypoint/v09/reverts/*.go` | **NEW**: v0.9 error codes (AA20-AA95) |
| `pkg/gas/estimate.go` | Version-aware gas estimation |
| `pkg/gas/overhead.go` | Version-aware struct size calculations |
| `pkg/gas/types.go` | New GasEstimates fields (both versions) |
| `pkg/modules/checks/standalone.go` | Version-aware validation switching |
| `pkg/modules/checks/initcode.go` | Version-aware factory validation |
| `pkg/modules/checks/paymaster.go` | Version-aware paymaster validation |
| `pkg/modules/checks/verificationgas.go` | Version-aware gas limit checks |
| `pkg/modules/checks/gas.go` | Version-aware prefund calculation |

### Critical (P0) - Keep As-Is

| File | Status |
|------|--------|
| `pkg/userop/object.go` | **KEEP**: v0.6 struct (backward compat) |
| `pkg/entrypoint/bindings.go` | **KEEP**: v0.6 bindings (backward compat) |
| `pkg/entrypoint/reverts/*.go` | **KEEP**: v0.6 error codes (backward compat) |

### High (P1) - New Files

| File | Changes |
|------|---------|
| `pkg/tracer/v09/*.js` | **NEW**: v0.9 tracer implementations |
| `pkg/tracer/types.go` | Add v0.9 trace return structs |
| `pkg/entrypoint/v09/simulation/*.go` | **NEW**: v0.9 off-chain simulation |
| `pkg/entrypoint/v09/execution/*.go` | **NEW**: v0.9 execution simulation |
| `pkg/entrypoint/v09/methods/*.go` | **NEW**: v0.9 ABI method definitions |
| `pkg/entrypoint/v09/transaction/*.go` | **NEW**: v0.9 transaction building |
| `pkg/entrypoint/v09/filter/*.go` | **NEW**: v0.9 event filtering |
| `pkg/estimator/gas.go` | Version-aware gas estimation logic |
| `internal/config/values.go` | Version detection config |

### Medium (P2)

| File | Changes |
|------|---------|
| `pkg/client/client.go` | Version-aware API handlers |
| `pkg/client/rpc.go` | Version adapters |
| `pkg/handlers/payg/handler.go` | Paymaster signature support |
| `pkg/bundler/bundler.go` | Version-aware batch building |
| `pkg/modules/relay/relayer.go` | Version-aware transaction relay |
| `pkg/modules/builder/builder.go` | Version-aware bundle building |
| `pkg/mempool/instance.go` | Version-aware struct serialization |

### Low (P3)

| File | Changes |
|------|---------|
| `docs/*.md` | Documentation updates |
| `e2e/*` | E2E test updates |
| `.github/workflows/*.yml` | CI updates |
| `e2e/*` | E2E test updates |
| `.github/workflows/*.yml` | CI updates |
| `README.md` | Version info |

---

## Arbitrum-Specific Considerations

### Gas Calculations
- Arbitrum uses same v0.9 EntryPoint (singleton via CREATE2)
- PVG buffer factor remains 16x for Arbitrum
- Update `pkg/arbitrum/` gas calculations for v0.9 struct size
- Test on Arbitrum Sepolia (chain ID: 421614)

### Known Issues
- Arbitrum `debug_traceCall` may have different behavior
- State override support varies
- L1 data cost affects gas estimation

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Struct refactor breaks compilation | High | High | Compile after each change |
| Tracer JS bugs | Medium | High | Test early and often |
| Gas calculation errors | Medium | High | Compare with reference bundler |
| EIP-7702 complexity | Medium | Medium | Defer if behind schedule |
| Arbitrum testing issues | Medium | Medium | Use Sepolia first |
| 30-day timeline too tight | Medium | High | Cut scope if needed |
| Backward compatibility bugs | Low | High | Extensive testing |

---

## Scope Cuts (If Behind Schedule)

1. **Alternative mempool support** - Can be deferred to post-migration
2. **P2P mempool (searcher mode)** - Focus on private mode
3. **Debug endpoints** - Update later
4. **OpenTelemetry** - Low priority
5. **All L2s except Arbitrum** - Already scoped out
6. **Full ERC-712 support** - Basic support first

---

## Success Criteria

- [ ] Compiles successfully (`go build ./...`)
- [ ] Unit tests pass (>80% coverage)
- [ ] E2E tests pass on Ethereum
- [ ] E2E tests pass on Arbitrum Sepolia
- [ ] Compliance tests pass (bundler-spec-tests v0.9)
- [ ] Version detection works correctly
- [ ] Backward compatibility works (v0.6 mode)
- [ ] EIP-7702 support functional
- [ ] Paymaster signature flow works
- [ ] Gas estimation within 10% of actual
- [ ] Documentation updated
- [ ] Migration guide written
- [ ] No v0.6-specific code in v0.9 path

---

## Dependencies

- [x] v0.9 EntryPoint ABI (from account-abstraction repo)
- [ ] Local test node (Geth with v0.9)
- [ ] Arbitrum Sepolia testnet access
- [ ] Reference bundler for comparison testing
- [ ] v0.9 test contracts (SimpleAccount, Simple7702Account)

---

## Resources

- [ERC-4337 Specification](https://eips.ethereum.org/EIPS/eip-4337)
- [ERC-7562 Validation Rules](https://eips.ethereum.org/EIPS/eip-7562)
- [Account Abstraction Repo](https://github.com/eth-infinitism/account-abstraction)
- [Bundler Reference Implementation](https://github.com/eth-infinitism/bundler)
- [EIP-7702 Documentation](https://eips.ethereum.org/EIPS/eip-7702)
- [Paymaster Signature Docs](https://docs.erc4337.io/paymasters/paymaster-signature.html)
- [Arbitrum Docs](https://docs.arbitrum.io/)

---

## Notes

- **Auto-detection**: Version is auto-detected from EntryPoint address. Override with env var.
- **Backward compatibility**: v0.6 mode kept for transition period. Marked deprecated.
- **Ethereum + Arbitrum**: Focus on these two chains. Other chains can be added later.
- **Paymaster**: Generic paymaster signature support. No specific service integration.
- **Timeline**: 30 days is aggressive but achievable with focused effort.

---

## Checklist

### Pre-Migration
- [ ] Read this roadmap
- [ ] Set up local test environment
- [ ] Download v0.9 contracts
- [ ] Review current codebase

### During Migration
- [ ] Follow week-by-week plan
- [ ] Compile after major changes
- [ ] Write tests alongside code
- [ ] Update documentation as you go

### Post-Migration
- [ ] Run all tests
- [ ] Deploy to testnet
- [ ] Monitor for issues
- [ ] Gather feedback
- [ ] Plan v0.6 deprecation timeline
