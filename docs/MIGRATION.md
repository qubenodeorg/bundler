# ERC-4337 v0.9 Migration Tracking

## Status: In Progress

---

## Completed Tasks

### Day 1 (Project Setup)
- [x] Downloaded v0.9 EntryPoint ABI → `abi/entrypoint.v09.json`
- [x] Set up `pkg/userop/v06/` and `pkg/userop/v09/` directories
- [x] Created migration tracking document
- [x] Reviewed current codebase for v0.6 dependencies (107 refs to `userop.UserOperation`, 15 calls to `userop.New()`)

### Days 2-3 (UserOperation Struct Refactor)
- [x] Created `pkg/userop/v06/` with v0.6 struct and ABI types
- [x] Created `pkg/userop/v09/` with v0.9 struct and ABI types
- [x] Created `pkg/userop/interface.go` with common interface
- [x] Updated `pkg/userop/parse.go` with `NewV09()` and `NewVersioned()`

---

## Codebase Review (Day 1)

### v0.6 Dependency Map

| Dependency | Files | Impact |
|------------|-------|--------|
| `userop.UserOperation` (struct) | 107 refs across 30+ files | Core type used everywhere |
| `userop.New()` (parse function) | 15 call sites | Entry point for op creation |
| `entrypoint.UserOperation` (bindings) | 1 file (generated) | ABI type for contract calls |
| `contract.UserOperation` (bindings) | 1 file (generated) | ABI type for contract calls |
| `entrypoint.UserOperation(*op)` cast | `handleops.go:48` | Direct struct cast |

### Struct Comparison

**Current `userop.UserOperation` (v0.6):**
- 11 fields including `InitCode []byte`, `PaymasterAndData []byte`
- `GetFactory()` extracts from InitCode[:20]
- `GetPaymaster()` extracts from PaymasterAndData[:20]
- `GetMaxGasAvailable()` has 3x multiplier for paymasters

**New `v09.UserOperation` (v0.9):**
- 16 fields including split `Factory`/`FactoryData` and `Paymaster`/`PaymasterData`/`PaymasterVerificationGasLimit`/`PaymasterPostOpGasLimit`
- Direct field access for factory and paymaster
- No 3x multiplier in gas calculations
- New fields: `EIP7702Auth`, `PaymasterSignature`

---

## Notes

- Migration follows Option B: existing `userop.UserOperation` and `userop.New()` kept intact
- New code uses `userop.NewV09()` for v0.9 UserOperations
- Version detection system (Day 4) will wire up auto-detection from EntryPoint address
