# Architecture

This document describes the high-level architecture of the qubenode-bundler, a Go implementation of an [ERC-4337 v0.6](https://eips.ethereum.org/EIPS/eip-4337) Bundler.

## Overview

The bundler is an off-chain actor that packages `UserOperations` (account abstraction transactions) and submits them to the Ethereum `EntryPoint` contract. It operates as an HTTP JSON-RPC server that accepts `UserOperations`, validates them, stores them in a local mempool, and periodically batches them for on-chain execution.

## System Architecture

```mermaid
flowchart TB
    subgraph External["External Systems"]
        W[Wallet/DApp]
        EP[EntryPoint Contract]
        ETH[Ethereum Node]
    end

    subgraph Bundler["Bundler Server"]
        API[JSON-RPC API<br/>port 4337]
        C[Client Module]
        M[Mempool<br/>BadgerDB]
        B[Bundler Module]
        MOD[Module Chain]
        SIG[Signer EOA]
    end

    W -->|eth_sendUserOperation| API
    API --> C
    C -->|Validate| MOD
    MOD -->|Accept| M
    M -->|Dump Pending| B
    B -->|Process Batch| MOD
    MOD -->|Build TX| SIG
    SIG -->|Submit| ETH
    ETH -->|Mine| EP
```

## Two-Mode Architecture

The bundler supports two distinct operating modes:

### Private Mode

In private mode, the bundler operates with a local mempool and sends transactions directly via an EOA (Externally Owned Account). This mode is compatible with all EVM networks but is vulnerable to frontrunning.

```mermaid
flowchart LR
    subgraph Private["Private Mode"]
        direction TB
        P1[Local Mempool]
        P2[EOA Relayer]
        P3[Regular TX Submission]
    end

    subgraph Searcher["Searcher Mode"]
        direction TB
        S1[P2P Mempool]
        S2[Block Builder API]
        S3[MEV Bundle Submission]
    end

    style Private fill:#e1f5fe
    style Searcher fill:#fff3e0
```

### Searcher Mode

In searcher mode, the bundler integrates with Flashbots or block builder APIs to submit bundles via MEV protection. This requires a compatible chain (Ethereum mainnet/Goerli/Sepolia) and builder URLs.

## Component Breakdown

### 1. HTTP Server (`pkg/jsonrpc/`)

Gin-based HTTP server exposing JSON-RPC endpoints on port 4337.

### 2. Client (`pkg/client/`)

Processes incoming `UserOperations` through validation middleware:
- Sanity checks (gas limits, fees, sender)
- EntryPoint simulation (`simulateValidation`)
- Reputation scoring
- Mempool insertion

### 3. Mempool (`pkg/mempool/`)

Persistent pending transaction pool backed by BadgerDB:
- Survives process restarts
- Organized by EntryPoint and sender address
- FIFO ordering for batching

### 4. Bundler (`pkg/bundler/`)

Timer-driven batch processor (runs every 1 second):
- Dumps pending ops from mempool
- Fetches current gas prices
- Executes batch middleware chain
- Removes processed/dropped ops

### 5. Signer (`pkg/signer/`)

EOA key management and transaction signing.

### 6. Modules (`pkg/modules/`)

Composable middleware for both Client and Bundler:
- **Client modules**: Validate individual ops
- **Bundler modules**: Process batches before submission

## Package Structure

```mermaid
flowchart TB
    subgraph cmd["cmd/"]
        ROOT[root.go]
        START[start.go]
    end

    subgraph internal["internal/"]
        CFG[config/]
        LOG[logger/]
        O11Y[o11y/]
        START_PRIV[start/private.go]
        START_SEARCH[start/searcher.go]
    end

    subgraph pkg["pkg/"]
        CLIENT[client/]
        BUNDLER_PKG[bundler/]
        MEMPOOL[mempool/]
        MODULES[modules/]
        USEROP[userop/]
        ENTRY[entrypoint/]
        GAS[gas/]
        SIGNER[signer/]
        JSONRPC[jsonrpc/]
        TRACER[tracer/]
    end

    START --> START_PRIV
    START --> START_SEARCH
    START_PRIV --> CLIENT
    START_PRIV --> BUNDLER_PKG
    START_PRIV --> MEMPOOL
    CLIENT --> MODULES
    CLIENT --> USEROP
    CLIENT --> ENTRY
    BUNDLER_PKG --> MODULES
    BUNDLER_PKG --> MEMPOOL
    MEMPOOL --> USEROP
    MODULES --> USEROP
    MODULES --> ENTRY
    CLIENT --> GAS
    BUNDLER_PKG --> GAS
    JSONRPC --> CLIENT
```

## Module System

The architecture uses two types of middleware handlers:

| Handler Type | Function Signature | Used By | Purpose |
|-------------|-------------------|---------|---------|
| `UserOpHandlerFunc` | `func(ctx *UserOpHandlerCtx) error` | Client | Validate single UserOperation |
| `BatchHandlerFunc` | `func(ctx *BatchHandlerCtx) error` | Bundler | Process batch of UserOperations |

Modules are composed into chains using `ComposeUserOpHandlerFunc` and `ComposeBatchHandlerFunc`.

### Client Module Chain (Private Mode)

```
1. rep.CheckStatus()       - Check entity reputation
2. rep.ValidateOpLimit()   - Validate ops seen limit
3. check.ValidateOpValues() - Sanity checks (gas, fees, sender)
4. check.SimulateOp()      - EntryPoint simulation
5. rep.IncOpsSeen()        - Increment ops seen counter
```

### Bundler Module Chain (Private Mode)

```
1. exp.DropExpired()              - Remove expired ops
2. gasprice.SortByGasPrice()      - Sort by gas price
3. gasprice.FilterUnderpriced()   - Filter underpriced ops
4. batch.SortByNonce()            - Sort by nonce
5. batch.MaintainGasLimit()       - Enforce batch gas limit
6. check.PaymasterDeposit()       - Verify paymaster deposits
7. check.SimulateBatch()          - Batch simulation (placeholder)
8. relayer.SendUserOperation()    - Submit via EOA
9. rep.IncOpsIncluded()           - Increment ops included
10. check.Clean()                 - Clean up DB entries
```

### Searcher Mode Differences

Searcher mode replaces the relayer with a builder module and adds code hash verification:

```diff
- relayer.SendUserOperation()
+ builder.SendUserOperation()

+ check.CodeHashes()              - Verify contract code hasn't changed
```

## Data Flow Summary

```mermaid
sequenceDiagram
    participant W as Wallet
    participant API as JSON-RPC API
    participant C as Client
    participant M as Mempool
    participant B as Bundler
    participant E as Ethereum

    W->>API: eth_sendUserOperation
    API->>C: SendUserOperation()
    C->>C: Validate + Simulate
    C->>M: AddOp()
    API-->>W: userOpHash

    loop Every 1 second
        B->>M: Dump()
        B->>B: Sort + Filter + Validate
        B->>E: Submit batch
        B->>M: RemoveOps()
    end

    W->>API: eth_getUserOperationReceipt
    API->>E: Query logs
    API-->>W: Receipt
```

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `go-ethereum` | Ethereum client, ABI bindings, crypto |
| `gin` | HTTP server framework |
| `cobra` + `viper` | CLI and configuration |
| `badger/v3` | Embedded persistent database |
| `flashbotsrpc` | Block builder API integration |
| `opentelemetry` | Observability (traces and metrics) |
| `zerolog` | Structured logging |

## Network Support

The bundler supports multiple EVM networks with chain-specific configurations:

| Network | Chain ID | Special Handling |
|---------|----------|-----------------|
| Ethereum Mainnet | 1 | Standard |
| Goerli | 5 | Standard |
| Sepolia | 11155111 | Standard |
| Optimism | 10 | OP Stack gas oracle |
| Arbitrum One | 42161 | Arbitrum gas buffer |
| Base | 8453 | OP Stack gas oracle |
| Lyra | 957 | OP Stack gas oracle |
| Ancient8 Sepolia | 28122024 | OP Stack gas oracle |

For L2 networks, the bundler adjusts PreVerificationGas calculations using network-specific gas oracles.

## EntryPoint Support

Default supported EntryPoint: `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789`

This is configurable via the `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` environment variable.

## Observability

Optional OpenTelemetry integration provides:
- Distributed tracing via OTLP/gRPC
- Metrics collection (mempool size gauge)
- Request logging with trace context

Enable by setting `ERC4337_BUNDLER_OTEL_SERVICE_NAME` and `ERC4337_BUNDLER_OTEL_COLLECTOR_URL`.
