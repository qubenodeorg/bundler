# Module System

This document describes the modular middleware architecture of the qubenode-bundler.

## Overview

The bundler uses a composable middleware pattern that allows validation and processing logic to be chained together. This design makes it easy to add, remove, or customize behavior without modifying core code.

## Handler Types

The module system defines two handler function types:

### `UserOpHandlerFunc`

Processes a single `UserOperation` during the Client's `eth_sendUserOperation` flow.

```go
type UserOpHandlerFunc func(ctx *UserOpHandlerCtx) error
```

**Context:** [`UserOpHandlerCtx`](https://pkg.go.dev/github.com/qubenode/bundler/pkg/modules#UserOpHandlerCtx)
- `UserOp` - The UserOperation being processed
- `EntryPoint` - Target EntryPoint address
- `ChainID` - Current chain ID
- `GetSenderDepositInfo()` - EntryPoint deposit for sender
- `GetFactoryDepositInfo()` - EntryPoint deposit for factory
- `GetPaymasterDepositInfo()` - EntryPoint deposit for paymaster
- `GetPendingSenderOps()` - Pending ops from same sender
- `GetPendingFactoryOps()` - Pending ops from same factory
- `GetPendingPaymasterOps()` - Pending ops from same paymaster

### `BatchHandlerFunc`

Processes a batch of `UserOperations` during the Bundler's periodic execution.

```go
type BatchHandlerFunc func(ctx *BatchHandlerCtx) error
```

**Context:** [`BatchHandlerCtx`](https://pkg.go.dev/github.com/qubenode/bundler/pkg/modules#BatchHandlerCtx)
- `Batch` - Slice of UserOperations to process
- `PendingRemoval` - Ops marked for removal
- `EntryPoint` - Target EntryPoint address
- `ChainID` - Current chain ID
- `BaseFee` - Current base fee
- `Tip` - Suggested gas tip
- `GasPrice` - Legacy gas price
- `Data` - Arbitrary key-value storage for logging
- `MarkOpIndexForRemoval(index, reason)` - Remove an op from the batch

## Module Composition

Modules are composed using helper functions that chain handlers together:

```go
// Client modules
client.UseModules(
    rep.CheckStatus(),
    rep.ValidateOpLimit(),
    check.ValidateOpValues(),
    check.SimulateOp(),
    rep.IncOpsSeen(),
)

// Bundler modules
bundler.UseModules(
    exp.DropExpired(),
    gasprice.SortByGasPrice(),
    gasprice.FilterUnderpriced(),
    batch.SortByNonce(),
    batch.MaintainGasLimit(maxGas),
    check.PaymasterDeposit(),
    check.SimulateBatch(),
    relayer.SendUserOperation(),
    rep.IncOpsIncluded(),
    check.Clean(),
)
```

Execution stops at the first module that returns an error.

## Built-in Modules

### Client Modules

#### Reputation (`pkg/modules/entities/`)

Tracks reputation for senders, factories, paymasters, and aggregators.

| Module | Description |
|--------|-------------|
| `CheckStatus()` | Rejects ops from banned or throttled entities |
| `ValidateOpLimit()` | Enforces ops-per-block limits for throttled entities |
| `IncOpsSeen()` | Increments the ops-seen counter for reputation scoring |
| `IncOpsIncluded()` | Increments the ops-included counter (Bundler side) |

#### Validation (`pkg/modules/checks/`)

Standard ERC-4337 validation checks.

| Module | Description |
|--------|-------------|
| `ValidateOpValues()` | Sanity checks: sender, initCode, gas limits, fees, pending ops |
| `SimulateOp()` | Runs `simulateValidation` on EntryPoint |

### Bundler Modules

#### Expiration (`pkg/modules/expire/`)

| Module | Description |
|--------|-------------|
| `DropExpired()` | Removes UserOperations that have exceeded their TTL |

#### Gas Price (`pkg/modules/gasprice/`)

| Module | Description |
|--------|-------------|
| `SortByGasPrice()` | Sorts batch by gas price (highest first) |
| `FilterUnderpriced()` | Removes ops with gas price below market |
| `GetBaseFeeWithEthClient()` | Fetches current base fee |
| `GetGasTipWithEthClient()` | Fetches suggested gas tip |
| `GetLegacyGasPriceWithEthClient()` | Fetches legacy gas price |

#### Batch Ordering (`pkg/modules/batch/`)

| Module | Description |
|--------|-------------|
| `SortByNonce()` | Sorts by sender nonce to ensure valid execution order |
| `MaintainGasLimit()` | Ensures total batch gas stays within limit |

#### Validation (`pkg/modules/checks/`)

| Module | Description |
|--------|-------------|
| `PaymasterDeposit()` | Verifies paymaster has sufficient deposit |
| `SimulateBatch()` | Batch-level simulation (placeholder) |
| `CodeHashes()` | Verifies contract code hasn't changed since first simulation |
| `Clean()` | Removes temporary DB entries after processing |

#### Relay (`pkg/modules/relay/`)

| Module | Description |
|--------|-------------|
| `SendUserOperation()` | Submits batch via EOA transaction (private mode) |

#### Builder (`pkg/modules/builder/`)

| Module | Description |
|--------|-------------|
| `SendUserOperation()` | Submits batch via Flashbots/block builder API (searcher mode) |

## Writing Custom Modules

### Custom Client Module

```go
package mymodule

import "github.com/qubenode/bundler/pkg/modules"

func MyValidator() modules.UserOpHandlerFunc {
    return func(ctx *modules.UserOpHandlerCtx) error {
        // Access the UserOperation
        op := ctx.UserOp
        
        // Perform validation
        if op.Sender == bannedAddress {
            return errors.New("sender is banned")
        }
        
        return nil
    }
}
```

### Custom Bundler Module

```go
package mymodule

import "github.com/qubenode/bundler/pkg/modules"

func MyBatchFilter() modules.BatchHandlerFunc {
    return func(ctx *modules.BatchHandlerCtx) error {
        // Iterate backwards to safely remove items
        for i := len(ctx.Batch) - 1; i >= 0; i-- {
            op := ctx.Batch[i]
            
            // Check condition and mark for removal
            if shouldDrop(op) {
                ctx.MarkOpIndexForRemoval(i, "custom reason")
            }
        }
        
        // Add data for logging
        ctx.Data["custom_metric"] = len(ctx.Batch)
        
        return nil
    }
}
```

### Registering Custom Modules

```go
// Client
c := client.New(...)
c.UseModules(
    mymodule.MyValidator(),
    // ... other modules
)

// Bundler
b := bundler.New(...)
b.UseModules(
    mymodule.MyBatchFilter(),
    // ... other modules
)
```

## Module Execution Order

The order of modules matters. Follow these guidelines:

### Client Module Order

1. **Reputation checks first** - Reject banned ops early to save resources
2. **Validation before simulation** - Catch obvious errors before expensive simulation
3. **Simulation before acceptance** - Ensure ops will pass on-chain
4. **Side effects last** - Increment counters only after validation passes

### Bundler Module Order

1. **Expiration first** - Remove stale ops immediately
2. **Sorting before filtering** - Ensure correct ordering for gas/nonce
3. **Validation before submission** - Verify paymaster deposits, code hashes
4. **Submission before cleanup** - Only clean up after successful relay
5. **Cleanup last** - Always clean temporary data

## Context Data

The `BatchHandlerCtx.Data` map is used for logging. Values added here appear in bundler run logs:

```go
ctx.Data["txn_hash"] = txn.Hash().String()
ctx.Data["batch_size"] = len(ctx.Batch)
```

## Error Handling

- Returning an error from a module stops the entire chain
- Use `MarkOpIndexForRemoval` to drop individual ops without failing the batch
- Log errors through the context's logger when available
