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
- [ ] Update `pkg/userop/object.go`:
  - Replace `InitCode []byte` with `Factory common.Address` + `FactoryData []byte`
  - Replace `PaymasterAndData []byte` with `Paymaster common.Address` + `PaymasterData []byte`
  - Add `PaymasterVerificationGasLimit *big.Int`
  - Add `PaymasterPostOpGasLimit *big.Int`
- [ ] Update `UserOpPrimitives` and `UserOpType` ABI types
- [ ] Update `Pack()` method for new struct
- [ ] Update `PackForSignature()` for v0.9 hash scheme
- [ ] Update `GetUserOpHash()` to match EntryPoint v0.9
- [ ] Update `GetMaxGasAvailable()` (remove 3x multiplier, add paymaster gas)
- [ ] Update `MarshalJSON()` and `ToMap()` for new fields
- [ ] Update helper methods: `GetPaymaster()`, `GetFactory()`, `GetFactoryData()`
- [ ] Update `pkg/userop/parse.go` for new field names and validation

**Deliverable**: Core data model compiles

#### Day 4: Version Detection System
- [ ] Add `EntryPointVersion` auto-detection:
  - Query EntryPoint for version via `version()` or `supportsInterface()`
  - Map known addresses to versions
  - Support explicit `ERC4337_BUNDLER_ENTRYPOINT_VERSION` env var as override
- [ ] Create version constants: `V0_6`, `V0_7`, `V0_8`, `V0_9`
- [ ] Add `Version` field to `internal/config/Values`
- [ ] Update `GetValues()` to auto-detect from `SupportedEntryPoints` addresses
- [ ] Add version-aware interfaces where needed

**Deliverable**: Version detection works

#### Days 5-6: EntryPoint Bindings Regeneration
- [ ] Regenerate `pkg/entrypoint/bindings.go` from v0.9 ABI using `abigen`
- [ ] Update `Makefile` target `generate-entrypoint-pkg`
- [ ] Update revert parsing for new error codes (AA20-AA95)
- [ ] Update `ValidationResult` parsing for new `ReturnInfo` structure
- [ ] Update `ExecutionResult` parsing
- [ ] Update `FailedOp` parsing
- [ ] Update event signatures:
  - `UserOperationEvent`
  - `AccountDeployed`
  - `EIP7702AccountInitialized`
  - `IgnoredInitCode`

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
- [ ] Update `pkg/tracer/BundlerCollectorTracer.js`:
  - Handle v0.9 EntryPoint internal function calls
  - Track new validation flow
  - Support TSTORE/TLOAD opcodes
- [ ] Update `pkg/tracer/BundlerExecutionTracer.js`:
  - New execution trace format
  - Capture `getCurrentUserOpHash()` calls
- [ ] Update `pkg/tracer/types.go`:
  - New trace return structs
  - EIP-7702 trace data
- [ ] Update `pkg/tracer/load.go`:
  - Version-aware tracer loading
- [ ] Test tracers against v0.9 EntryPoint locally

**Deliverable**: Tracers work with v0.9

#### Days 13-14: Simulation & Execution
- [ ] Update `pkg/entrypoint/simulation/simulatevalidation.go`:
  - Off-chain simulation only
  - Handle new error codes
  - Version-aware simulation
- [ ] Update `pkg/entrypoint/simulation/tracevalidation.go`:
  - New trace format handling
- [ ] Update `pkg/entrypoint/execution/simulate.go`:
  - Pass new UserOperation struct
  - Handle `executeUserOp()` (v0.7)
- [ ] Update `pkg/entrypoint/execution/trace.go`:
  - Execution tracer integration
- [ ] Update `pkg/entrypoint/methods/handleops.go`:
  - New UserOperation tuple type
  - Version-aware method building
- [ ] Update `pkg/entrypoint/transaction/handleops.go`:
  - `toAbiType()` mapping for new fields
  - Version-aware transaction building
- [ ] Update `pkg/entrypoint/transaction/gas.go`:
  - Gas estimation for v0.9 struct

**Deliverable**: Simulation and execution compile

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

### Critical (P0)

| File | Changes |
|------|---------|
| `pkg/userop/object.go` | New struct fields, hash calculation, JSON |
| `pkg/userop/parse.go` | Parse new fields, validation |
| `pkg/entrypoint/bindings.go` | Regenerate from v0.9 ABI |
| `pkg/entrypoint/reverts/*.go` | New error codes |
| `pkg/gas/estimate.go` | New gas fields, penalty logic |
| `pkg/gas/overhead.go` | Struct size, costs |
| `pkg/gas/types.go` | New GasEstimates fields |
| `pkg/modules/checks/standalone.go` | Version-aware validation |
| `pkg/modules/checks/initcode.go` | Factory validation |
| `pkg/modules/checks/paymaster.go` | Paymaster validation |
| `pkg/modules/checks/verificationgas.go` | Paymaster gas limits |
| `pkg/modules/checks/gas.go` | Prefund calculation |

### High (P1)

| File | Changes |
|------|---------|
| `pkg/tracer/*.js` | v0.9 internals, TSTORE/TLOAD |
| `pkg/tracer/types.go` | New trace structs |
| `pkg/entrypoint/simulation/*.go` | Off-chain simulation |
| `pkg/entrypoint/execution/*.go` | New struct, executeUserOp |
| `pkg/entrypoint/methods/*.go` | New tuple types |
| `pkg/entrypoint/transaction/*.go` | Version-aware building |
| `pkg/entrypoint/filter/*.go` | New event formats |
| `pkg/estimator/gas.go` | New gas estimation logic |
| `internal/config/values.go` | Version config |

### Medium (P2)

| File | Changes |
|------|---------|
| `pkg/client/client.go` | Version-aware API |
| `pkg/client/rpc.go` | Version adapters |
| `pkg/handlers/payg/handler.go` | Paymaster signatures |
| `pkg/bundler/bundler.go` | Version-aware batching |
| `pkg/modules/relay/relayer.go` | Version-aware relay |
| `pkg/modules/builder/builder.go` | Version-aware bundles |
| `pkg/mempool/instance.go` | New struct serialization |

### Low (P3)

| File | Changes |
|------|---------|
| `docs/*.md` | Documentation updates |
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
