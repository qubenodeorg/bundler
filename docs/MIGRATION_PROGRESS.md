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

- [ ] Update validation rules for v0.7/v0.8/v0.9
- [ ] Update tracers for new versions
- [ ] Create v0.9-specific simulation packages

## Week 3: Features & Integration (Days 15-21)

- [ ] EIP-7702 support
- [ ] Paymaster signature flow
- [ ] Block number validity
- [ ] Client & bundler integration

## Week 4: Testing & Polish (Days 22-30)

- [ ] Unit tests (>80% coverage target)
- [ ] E2E tests
- [ ] Compliance tests
- [ ] Documentation updates
