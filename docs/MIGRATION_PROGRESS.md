# ERC-4337 v0.9 Migration Progress

## Day 1: Project Setup

- [x] Download v0.9 EntryPoint ABI from eth-infinitism/account-abstraction
- [x] Set up local test environment tracking
- [x] Create migration tracking document
- [x] Review current codebase for all v0.6 dependencies
- [x] Add git pre-push hook (lint + build + unit tests, exclude E2E)

## Days 2-3: UserOperation Struct Refactor

- [x] Create `pkg/userop/v06/object.go` (v0.6 struct for backward compat)
- [x] Create `pkg/userop/v06/parse.go` (v0.6 parsing)
- [x] Create `pkg/userop/v07/object.go` (v0.7 struct with Factory/Paymaster fields)
- [x] Create `pkg/userop/v07/parse.go` (v0.7 parsing)
- [x] Create `pkg/userop/v08/object.go` (v0.8 struct + EIP7702Auth)
- [x] Create `pkg/userop/v08/parse.go` (v0.8 parsing with EIP7702Auth support)
- [x] Create `pkg/userop/v09/object.go` (v0.9 struct + PaymasterSignature)
- [x] Create `pkg/userop/v09/parse.go` (v0.9 parsing with paymasterSignature support)
- [x] Create `pkg/userop/interface.go` (UserOperationInterface + version constants + factory)
- [x] Write unit tests for all new code (coverage: v06=88.5%, v07=85.9%, v08=78.4%, v09=88.0%)

## Day 4: Version Detection System

- [x] Create version constants: V0_6, V0_7, V0_8, V0_9
- [x] Add EntryPointVersion auto-detection from address
- [x] Add `Version` field to `internal/config/Values`
- [x] Update `GetValues()` for auto-detection
- [x] Support `ERC4337_BUNDLER_ENTRYPOINT_VERSION` env var override
- [x] Update default `SupportedEntryPoints` from v0.6 to v0.9
- [x] Verified full build passes

## Day 5-6: EntryPoint Bindings Regeneration

- [ ] Keep existing `pkg/entrypoint/bindings.go` (v0.6)
- [ ] Create `pkg/entrypoint/v09/bindings.go` from v0.9 ABI
- [ ] Create version-specific revert parsing
- [ ] Create version-specific event signatures

## Day 7: Gas Model Foundation

- [x] Update `pkg/gas/types.go` with new fields (PaymasterVerificationGasLimit, PaymasterPostOpGasLimit)
- [x] Update `pkg/gas/estimate.go` for version-aware estimation (interface support, ConvertToV06 helper, v0.9 error codes)
- [x] Update `pkg/gas/overhead.go` for version-aware overhead (UserOperationInterface, version-routed sanitization)
- [x] Implement 10% unused gas penalty (v0.7) — `CalcUnusedGasPenalty()`, `SetUnusedGasPenaltyPercent()`
- [x] Implement 40,000 gas penalty threshold (v0.8) — `SetGasPenaltyThreshold()`, integrated into penalty calc
- [x] Add `GetSender()` and `GetMaxFeePerGas()` to `UserOperationInterface` (all 4 version packages)
- [x] Update `CalcPreVerificationGasFunc` type to use `UserOperationInterface`
- [x] Add version-specific Overhead constructors: `NewDefaultOverheadV07()`, `V08()`, `V09()`
- [x] Write unit tests: types_test.go (4 tests), overhead_test.go (30+ tests), estimate_test.go (20+ tests), pvg_test.go (3 tests)
- [x] Coverage: non-RPC functions >90%, RPC-dependent functions (EstimateGas, Arbitrum/Optimism PVG) need RPC connection

## Week 2: Validation & Simulation (Days 8-14)

- [x] Update `pkg/modules/checks/initcode.go` — ValidateFactory + ValidateFactoryData
- [x] Update `pkg/modules/checks/paymaster.go` — ValidatePaymaster, ValidatePaymasterData, ValidatePaymasterVerificationGasLimit, ValidatePaymasterPostOpGasLimit
- [x] Update `pkg/modules/checks/verificationgas.go` — ValidateVerificationGasV07 with paymaster gas limits
- [x] Update `pkg/modules/checks/gas.go` — ValidateGasAvailableV07 for new prefund
- [x] Update `pkg/modules/checks/standalone.go` — Version-aware validation switching (v0.6 vs v0.7+)
- [x] Add V07 overloads to sender.go, callgas.go, gasfee.go, pendingops.go
- [x] Implement ERC-7562 validation rules (OP-013, OP-051, OP-054, STO-033, EREP-020, OP-070)
- [x] Add `delegateAndRevert()` support for off-chain simulation (v0.7+)
- [x] Remove on-chain `simulateValidation` calls for v0.7+ (replaced by trace-based + delegateAndRevert)
- [x] Add `Version` field to `UserOpHandlerCtx` with auto-detection from EntryPoint address
- [x] Extend `UserOperationInterface` with 10 new getter methods
- [x] Implement new interface methods in all version packages (v06, v07, v08, v09)
- [x] Update tracers for new versions
- [x] Create v0.9-specific simulation packages (`pkg/entrypoint/v07/simulation`, `v08/simulation`, `v09/simulation`) with `ProcessTraceResult`, `TraceSimulateValidation`, `TraceInput`/`TraceOutput`

### Day 8-10: PR-24 review fixes

- [x] Add division-by-zero guards in `pkg/entrypoint/transaction/gas.go` (`SuggestMeanGasTipCap`, `SuggestMeanGasFeeCap`, `SuggestMeanGasPrice`) — empty batch returns the supplied fallback
- [x] Add division-by-zero guards in `pkg/entrypoint/v09/transaction/gas.go` (v09-specific copies of the same three functions)
- [x] Add empty-batch regression tests in both `gas_test.go` files (6 new test cases)
- [x] Consolidate zero-address paymaster check in `pkg/modules/checks/paymaster.go` — single `ValidatePaymasterData` rule applies to v0.6/v0.7/v0.8/v0.9; remove duplicate from `ValidatePaymaster`
- [x] Add v0.6/v0.8/v0.9 zero-addr paymaster tests in `paymaster_v07_test.go`
- [x] Change `SimulateValidationV07` and `simulateWithTrace` signatures in `pkg/modules/checks/simulate.go` — accept `userop.UserOperationInterface` and a caller-supplied `simulation.EntityStakes`
- [x] Pass real stakes from `standalone.go` — built from `ctx.GetSenderDepositInfo()` / `GetFactoryDepositInfo()` / `GetPaymasterDepositInfo()` (no more hardcoded zero-value stakes)
- [x] Refactor `UserOpHandlerCtx.UserOp` to `userop.UserOperationInterface` (and `NewUserOpHandlerContext` first param) in `pkg/modules/context.go`
- [x] Update `pkg/modules/entities/reputation.go` to use `ctx.UserOp.GetSender()` instead of direct `.Sender` field access (5 sites)
- [x] Narrow `ctx.UserOp` to `*userop.UserOperation` in v0.6 branch of `standalone.go` (v0.7+ path already used interface getters)
- [x] Narrow at mempool boundary in `pkg/client/client.go` — `i.mempool.AddOp(epAddr, ctx.UserOp.(*userop.UserOperation))`
- [x] Update `pkg/modules/checks/erc7562.go` — `ValidateERC7562Rules` accepts `userop.UserOperationInterface`
- [x] `go test -count=1 ./...`, `go vet ./...`, and `go test -race` on `pkg/entrypoint/...` + `pkg/modules/...` all pass

### Days 13-14: Simulation & Execution (v07, v08, v09 packages)

**Approach**: Created version-specific simulation, methods, and transaction packages; kept v0.6 intact

- [x] Create `pkg/entrypoint/v07/methods/`, `v08/methods/` with `handleops.go`, `primitives.go` (PackedUserOp ABI types)
- [x] Create `pkg/entrypoint/v07/transaction/`, `v08/transaction/` with `handleops.go`, `gas.go`, `utils.go` (version-aware transaction building)
- [x] Create `pkg/entrypoint/v07/simulation/simulatevalidation.go`, `v08/simulation/simulatevalidation.go`, `v09/simulation/simulatevalidation.go` (off-chain `SimulateValidation` via `handleOps` `eth_call`, handles AA20-AA95 error codes via version-specific `reverts/` packages)
- [x] Existing `pkg/entrypoint/simulation/` and `pkg/entrypoint/execution/` (v0.6) remain untouched
- [x] Write unit tests for all new code (coverage: methods=100%, reverts=78%, simulation=~50%, transaction=52.9%)
- [x] `go build ./...`, `go vet ./...`, and `go test -count=1 ./pkg/entrypoint/...` all pass

### Days 15-17: EIP-7702 Support

- [x] Add `EIP7702Auth` field to UserOperation struct (already present in v0.8/v0.9)
- [x] Update `pkg/client/client.go`: Accept `eip7702Auth` parameter in `eth_sendUserOperation`
- [x] Update `pkg/modules/checks/standalone.go`: Validate EIP-7702 authorization for v0.8/v0.9
- [x] Update `pkg/modules/checks/auth.go`: Fix v08/v09 auth type handling, normalize yParity (0/1 and 27/28)
- [x] Update `pkg/entrypoint/filter/event_eip7702.go`: Handle `EIP7702AccountInitialized` event
- [x] Update `pkg/modules/entities/`: Reputation tracking for 7702 accounts
- [x] Add `Simple7702Account` contract type support with `MapContractTypeWithClient`
- [x] Update `pkg/entrypoint/simulation/`: Check delegation address correctness
- [x] Update `pkg/entrypoint/v08/simulation/` and `v09/simulation/`: Add `DelegateAddress` to `TraceInput`, pass delegate to `NewKnownEntity`
- [x] Write unit tests for all EIP-7702 code (auth tests: 14 tests, event tests: 1 test, simulation tests: 2 new tests per version, contracts tests: 4 tests, ERC-7562 tests: 7 tests)
- [x] `go build ./...`, `go test -count=1 ./pkg/modules/checks/...`, `go test -count=1 ./pkg/entrypoint/...` all pass

### Days 18-19: v0.8/v0.9 Features

**Approach**: Separate methods for ERC-712, paymaster signature validation, block number validity, v0.9 initCode handling, and getCurrentUserOpHash exposure.

- [x] Implement ERC-712 hash support:
  - Add `GetUserOpHashEIP712(domainSeparator, typeHash)` to `UserOperationInterface`
  - Implement in v06 (fallback to legacy), v07, v08, v09
  - Formula: `keccak256("\x19\x01" || domainSeparator || keccak256(typeHash || PackForSignature()))`
  - Tests: deterministic hash, different from legacy hash
- [x] Implement paymaster signature field:
  - Add `GetPaymasterSignature()` to interface; v09 returns `PaymasterSignature`, others return `[]byte{}`
  - Add `PaymasterSignature` field to `SponsorUserOperationResponse`
  - Update `payg/handler.go` to detect v09 ops and return signature separately
  - Add `ValidatePaymasterSignature()` structural check (presence when paymaster is set)
  - Update `standalone.go` to call `ValidatePaymasterSignature` for v0.9
  - Tests: v09 hash excludes paymasterSignature, response format, validation cases
- [x] Implement block number validity:
  - Create `pkg/modules/checks/validity.go` with `IsBlockNumberMode()`, `ValidateTimeRange()`
  - Detect block number mode via bit 47 of uint48 validAfter/validUntil
  - Block number mode: validate against current block with 1-block buffer
  - Timestamp mode: validate against Unix time with 30-second buffer
  - Integrate into `simulate.go` via `validateTimeRangeFromTrace()`
  - Tests: 17 test cases covering all modes, boundaries, zero values
- [x] Update initCode handling (v0.9):
  - Add `ValidateSenderV09()` that allows initCode with existing accounts
  - Update `standalone.go` to use `ValidateSenderV09` for v0.9
  - Add `filterIgnoredInitCodeEvent()` in `pkg/entrypoint/v09/filter/event.go`
  - Tests: existing account + factory allowed, new account without factory rejected
- [x] Add `getCurrentUserOpHash()` support:
  - Add `CurrentUserOpHash` field to `TraceOutput` in v09 execution
  - Populate from `BundlerExecutionReturn.CurrentUserOpHash` after trace
  - Execution tracer already captures STATICCALLs (implemented in Days 11-12)
  - Tests: struct field presence, empty default
- [x] `go build ./...`, `go test -count=1 ./...`, `go vet ./...` all pass
- [x] Write unit tests for all new code:
  - `validity_test.go`: 17 tests
  - `paymaster_test.go`: 6 new tests (paymaster signature)
  - `sender_test.go`: 4 new tests (v09 sender)
  - `object_test.go` (v09): 4 new tests (EIP-712, paymaster sig)
  - `handler_test.go` (payg): 3 new tests
  - `tracer_test.go` (v09 execution): 2 new tests (getCurrentUserOpHash)

### Days 20-21: Client & Bundler Integration

**Approach**: Use packed v0.6 struct as canonical mempool format. Version-aware ops are converted at the client boundary via `ConvertToV06Packed()` (preserves real paymaster gas limits, unlike `gas.ConvertToV06`). Mempool/batch/transaction packages all continue to operate on this single shape, with relay/builder dispatching to per-version `EstimateHandleOpsGas`/`HandleOps` based on the EntryPoint address.

- [x] Add `pkg/userop/convert.go` with `ConvertToV06Packed()` (preserves factory+factoryData → InitCode and paymaster+gasLimits+paymasterData → PaymasterAndData using on-chain PackedUserOperation layout)
- [x] Update `pkg/client/client.go`:
  - Version detection from EntryPoint address + request body fields
  - `SendUserOperation`: parses versioned op, converts to packed v0.6 before mempool insertion
  - `EstimateUserOperationGas`: passes versioned op + version into `getGasEstimate`, populates `PaymasterVerificationGasLimit`/`PaymasterPostOpGasLimit` for v0.7+
  - `GetUserOperationReceipt` / `GetUserOperationByHash`: dispatched by `GetUserOpReceiptWithEthClient` and `GetUserOpByHashWithEthClient` to the correct version filter package
  - `SponsorUserOperation`: hands versioned op directly to `payg.Handler`
- [x] Update `pkg/client/utils.go`:
  - `GetGasEstimateFunc` takes `userop.UserOperationInterface` + `userop.Version`
  - `HashLookupResult.UserOperation` is `map[string]any` to preserve per-version field naming in the JSON response (initCode for v0.6, factory/factoryData for v0.7+, etc.)
- [x] Update `pkg/handlers/payg/handler.go`:
  - Uses `ConvertToV06Packed()` (real paymaster gas limits) instead of `gas.ConvertToV06()` (dummy zero gas limits)
  - Factored into `convertOp`, `signOp`, `buildResponse`, `isV09` helpers
- [x] Update `pkg/modules/relay/relayer.go` and `pkg/modules/builder/client.go`:
  - Detect version from `ctx.EntryPoint`
  - Route to `pkg/entrypoint/transaction` (v0.6), `pkg/entrypoint/v07/transaction`, `v08/transaction`, or `v09/transaction`
- [x] Tests added: `convert_test.go`, `bundler_test.go`, `client_test.go`, `utils_test.go`, `rpc_test.go`, `relayer_test.go`, expanded `handler_test.go`
- [x] `go build ./...`, `go vet ./...`, `go test -count=1 ./...` all pass
- [x] Coverage targets met:
  - `pkg/client`: 71.3% (from 0.0%)
  - `pkg/bundler`: 78.4% (from 0.0%)
  - `pkg/modules/relay`: 82.6% (from 0.0%)
  - `pkg/modules/builder`: 71.2% (maintained)
  - `pkg/handlers/payg`: 70.6% (from 0.0%)
  - `pkg/userop`: 58.1% (extended with convert.go)

## Week 3: Features & Integration (Days 18-21)

- [x] Paymaster signature flow
- [x] Block number validity
- [x] Client & bundler integration

## Week 4: Testing & Polish (Days 22-30)

- [x] Unit tests (>70% coverage for new code)
- [ ] E2E tests
- [ ] Compliance tests
- [ ] Documentation updates
