# Migration Guide

This guide covers upgrading a qubenode-bundler deployment from ERC-4337 **v0.6**
to **v0.7, v0.8, or v0.9**. The bundler supports all four versions from a single
binary, so migration is primarily a **configuration** change — point it at the
EntryPoint for the version you want and the bundler adapts its parsing,
validation, gas model, and hashing automatically.

- [Quick Start](#quick-start)
- [Choosing a Version](#choosing-a-version)
- [Configuration Changes](#configuration-changes)
- [Breaking Changes](#breaking-changes)
- [UserOperation Field Changes](#useroperation-field-changes)
- [Client / Integration Changes](#client--integration-changes)
- [Step-by-Step Migration](#step-by-step-migration)
- [Rollback](#rollback)

## Quick Start

To run against the v0.9 EntryPoint, set one variable:

```bash
ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS=0x433709009B8330FDa32311DF1C2AFA402eD8D009
# version auto-detected as v0.9
```

No code changes are required in the bundler. Wallets and SDKs that submit
UserOperations must send the new field layout (see
[UserOperation Field Changes](#useroperation-field-changes)).

## Choosing a Version

| Version | EntryPoint address | When to use |
|---------|--------------------|-------------|
| v0.6 | `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` | Existing v0.6 accounts; legacy mode |
| v0.7 | `0x0000000071727De22E5E9d8BAf0edAc6f37da032` | Split fields, off-chain validation |
| v0.8 | `0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108` | EIP-7702 delegated EOAs, ERC-712 hashing |
| v0.9 | `0x433709009B8330FDa32311DF1C2AFA402eD8D009` | Detached paymaster signatures |

> **Note:** v0.9 tracks the `v0.9.0` account-abstraction contracts and is
> experimental until upstream ships a stable v0.9 release.

## Configuration Changes

| Variable | Old | New | Notes |
|----------|-----|-----|-------|
| `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` | v0.6 address | target version address | Drives auto-detection |
| `ERC4337_BUNDLER_ENTRYPOINT_VERSION` | _(new)_ | `auto` (default) | Override to pin a version for non-canonical EntryPoints |

**Auto-detection** maps the first configured EntryPoint address to a version. If
you deploy a non-canonical EntryPoint (e.g. on a local testnet), set
`ERC4337_BUNDLER_ENTRYPOINT_VERSION` explicitly — otherwise an unknown address
falls back to `v0.6`. See
[CONFIGURATION.md](./docs/CONFIGURATION.md#entrypoint-version-detection).

## Breaking Changes

These changes are defined by the ERC-4337 spec between versions. The bundler
handles them internally; integrators need to be aware of them.

### v0.7

1. **UserOperation struct** — `initCode` split into `factory` + `factoryData`;
   `paymasterAndData` split into `paymaster`, `paymasterVerificationGasLimit`,
   `paymasterPostOpGasLimit`, and `paymasterData`.
2. **userOpHash** — no longer multiplies `verificationGasLimit` by 3 for
   paymasters; paymaster gas limits are accounted separately.
3. **Simulation** — `simulateValidation` is removed from the deployed contract;
   validation is simulated off-chain using the ERC-7562 tracer.
4. **Gas model** — 10% penalty on unused `callGasLimit`/paymaster gas.
5. **Error codes** — missing/invalid `userOpHash` returns `-32602` (was `-32601`
   in v0.6).

### v0.8

6. **EIP-7702** — an EOA can be the sender by delegating to account code. Signal
   with the `factory` marker `0x7702...` plus an `eip7702Auth` tuple.
7. **ERC-712 `userOpHash`** — the hash is now an EIP-712 typed-data hash
   (domain `ERC4337` / version `1`), replacing the legacy
   `keccak(hash || entryPoint || chainId)` scheme.
8. **Validation relaxations** — CREATE/CREATE2 allowed during deployment; all
   staked entities get storage access (STO-022 no longer an error).

### v0.9

9. **Paymaster signature** — new `paymasterSignature` field, **excluded** from
   the `userOpHash`, enabling parallel paymaster signing. It is optional.
10. **Block-number validity** — `validAfter`/`validUntil` may carry block
    numbers.
11. **initCode handling** — silently ignored if the account already exists.

## UserOperation Field Changes

| v0.6 field | v0.7+ replacement |
|------------|-------------------|
| `initCode` (address ‖ data) | `factory` (address) + `factoryData` (bytes) |
| `paymasterAndData` (address ‖ data) | `paymaster` + `paymasterVerificationGasLimit` + `paymasterPostOpGasLimit` + `paymasterData` |
| — | `eip7702Auth` (object, v0.8+) |
| — | `paymasterSignature` (bytes, v0.9) |

**v0.6 request body:**
```json
{
  "sender": "0x...",
  "nonce": "0x0",
  "initCode": "0x<factory><factoryData>",
  "callData": "0x...",
  "callGasLimit": "0x...",
  "verificationGasLimit": "0x...",
  "preVerificationGas": "0x...",
  "maxFeePerGas": "0x...",
  "maxPriorityFeePerGas": "0x...",
  "paymasterAndData": "0x",
  "signature": "0x..."
}
```

**v0.7+ request body (existing account, no paymaster):**
```json
{
  "sender": "0x...",
  "nonce": "0x0",
  "callData": "0x...",
  "callGasLimit": "0x...",
  "verificationGasLimit": "0x...",
  "preVerificationGas": "0x...",
  "maxFeePerGas": "0x...",
  "maxPriorityFeePerGas": "0x...",
  "signature": "0x..."
}
```

Optional groups (`factory*`, `paymaster*`, `eip7702Auth`, `paymasterSignature`)
are omitted entirely when unused. See [docs/API.md](./docs/API.md) for the full
field reference and examples.

## Client / Integration Changes

- **`eth_estimateUserOperationGas`** returns `paymasterVerificationGasLimit` and
  `paymasterPostOpGasLimit` for v0.7+ sponsored ops.
- **`pm_sponsorUserOperation`** returns the paymaster signature separately in a
  `paymasterSignature` field for v0.9 (appended into `paymasterAndData` for
  v0.6–v0.8). Copy it into the op's `paymasterSignature` field.
- **`eth_getUserOperationByHash` / `eth_getUserOperationReceipt`** echo the op in
  the version-appropriate field layout.
- Method names, the JSON-RPC envelope, and error codes are otherwise unchanged.

## Step-by-Step Migration

1. **Pick the target version** and its EntryPoint address from the table above.
2. **Deploy or confirm the EntryPoint** is available on your network at the
   canonical address.
3. **Update configuration:**
   ```bash
   ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS=<target EntryPoint address>
   # optionally, for non-canonical deployments:
   # ERC4337_BUNDLER_ENTRYPOINT_VERSION=v0.7
   ```
4. **Restart the bundler.** Confirm the resolved version in the startup logs.
5. **Update wallets / SDKs** to send the new field layout (see above).
6. **Smoke-test** `eth_sendUserOperation`, `eth_estimateUserOperationGas`, and
   `eth_getUserOperationReceipt` end-to-end on a testnet.
7. **For v0.8+**, exercise the EIP-7702 flow if you use delegated EOAs.
8. **For v0.9**, verify the detached `paymasterSignature` round-trips through your
   paymaster integration.

## Rollback

Because the binary is version-agnostic, rolling back is a configuration change:
restore the previous `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` (and
`ERC4337_BUNDLER_ENTRYPOINT_VERSION`, if set) and restart. In-flight ops in the
mempool are stored in the canonical packed form; drain the mempool before
switching versions to avoid mixing incompatible accounts. To reset the embedded
database:

```bash
make dev-reset-default-data-dir
```
