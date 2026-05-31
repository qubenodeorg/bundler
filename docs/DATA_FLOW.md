# Data Flow

This document details the lifecycle of a `UserOperation` through the bundler system, including request flows, batch processing, and error handling paths.

## UserOperation Lifecycle

```mermaid
flowchart TB
    subgraph Ingestion["1. Ingestion"]
        A[Wallet/DApp] -->|JSON-RPC| B[HTTP Server]
    end

    subgraph Validation["2. Validation"]
        B --> C[Parse UserOp]
        C --> D[Check EntryPoint]
        D --> E[Client Module Chain]
        E -->|Pass| F[Add to Mempool]
        E -->|Fail| G[Return Error]
    end

    subgraph Batching["3. Batching"]
        F --> H[Bundler Timer]
        H -->|Every 1s| I[Dump Mempool]
        I --> J[Sort & Filter]
        J --> K[Bundler Module Chain]
        K -->|Pass| L[Build Batch]
        K -->|Drop| M[Remove from Mempool]
    end

    subgraph Submission["4. Submission"]
        L --> N[Estimate Gas]
        N --> O[Sign Transaction]
        O --> P[Submit to Network]
        P -->|Wait| Q[Confirm Inclusion]
    end

    subgraph Cleanup["5. Cleanup"]
        Q --> R[Remove from Mempool]
        M --> R
        R --> S[Clean DB Entries]
    end

    style Validation fill:#e8f5e9
    style Batching fill:#fff3e0
    style Submission fill:#e3f2fd
    style Cleanup fill:#fce4ec
```

## Request Flow: `eth_sendUserOperation`

```mermaid
sequenceDiagram
    actor W as Wallet/DApp
    participant API as JSON-RPC Handler
    participant C as Client
    participant R as Reputation Module
    participant V as Validation Module
    participant S as Simulation Module
    participant M as Mempool

    W->>API: POST / {"method":"eth_sendUserOperation","params":[userOp, entryPoint]}
    API->>C: SendUserOperation(op, ep)
    
    C->>C: Parse entryPoint address
    C->>C: Parse userOp fields
    C->>C: Compute userOpHash
    
    C->>R: CheckStatus()
    R-->>C: OK / Banned / Throttled
    
    alt Entity Banned
        C-->>API: Error: BANNED
        API-->>W: JSON-RPC Error
    end
    
    C->>R: ValidateOpLimit()
    R-->>C: OK / Too Many Ops
    
    C->>V: ValidateOpValues()
    V->>V: Check sender, initCode<br/>Check gas limits, fees<br/>Check pending ops
    V-->>C: OK / Validation Error
    
    alt Validation Failed
        C-->>API: Error: INVALID_FIELDS
        API-->>W: JSON-RPC Error
    end
    
    C->>S: SimulateOp()
    S->>S: Call simulateValidation<br/>on EntryPoint
    S-->>C: OK / Simulation Error
    
    alt Simulation Failed
        C-->>API: Error: REJECTED_BY_EP_OR_ACCOUNT
        API-->>W: JSON-RPC Error
    end
    
    C->>R: IncOpsSeen()
    R-->>C: OK
    
    C->>M: AddOp(entryPoint, userOp)
    M-->>C: OK
    
    C-->>API: userOpHash
    API-->>W: {"result": "0x..."}
```

## Request Flow: `eth_estimateUserOperationGas`

```mermaid
sequenceDiagram
    actor W as Wallet/DApp
    participant API as JSON-RPC Handler
    participant C as Client
    participant G as Gas Estimator
    participant ETH as Ethereum Node

    W->>API: POST / {"method":"eth_estimateUserOperationGas","params":[userOp, entryPoint, stateOverride]}
    API->>C: EstimateUserOperationGas(op, ep, os)
    
    C->>C: Parse entryPoint address
    C->>C: Parse userOp fields
    
    alt maxFeePerGas == 0
        C->>ETH: eth_maxPriorityFeePerGas
        ETH-->>C: Suggested fees
        C->>C: Override userOp gas prices
    end
    
    C->>G: Estimate gas limits
    G->>ETH: debug_traceCall with tracer
    ETH-->>G: Tracing results
    G-->>C: verificationGasLimit, callGasLimit
    
    C->>C: Calculate PreVerificationGas
    
    C-->>API: GasEstimates
    API-->>W: {"result":{"preVerificationGas":"...","verificationGasLimit":"...","callGasLimit":"..."}}
```

## Batch Processing Flow

```mermaid
flowchart TB
    subgraph Timer["Timer Trigger"]
        T[1 Second Timer] -->|Tick| B[Bundler.Run]
    end

    subgraph Collect["Collect Phase"]
        B -->|For each EntryPoint| D[Dump Mempool]
        D -->|FIFO List| BS[Batch Slice]
    end

    subgraph GasFetch["Gas Fetch Phase"]
        BS --> BF[Get BaseFee]
        BS --> GT[Get GasTip]
        BS --> GP[Get LegacyGasPrice]
    end

    subgraph Process["Processing Phase"]
        BF --> CTX[Create BatchHandlerCtx]
        GT --> CTX
        GP --> CTX
        CTX --> MOD[Execute Module Chain]
    end

    subgraph Modules["Module Chain Execution"]
        MOD --> MOD1[DropExpired]
        MOD1 --> MOD2[SortByGasPrice]
        MOD2 --> MOD3[FilterUnderpriced]
        MOD3 --> MOD4[SortByNonce]
        MOD4 --> MOD5[MaintainGasLimit]
        MOD5 --> MOD6[PaymasterDeposit]
        MOD6 --> MOD7[SimulateBatch]
        MOD7 --> MOD8[SendUserOperation]
        MOD8 --> MOD9[IncOpsIncluded]
        MOD9 --> MOD10[Clean]
    end

    subgraph Cleanup2["Cleanup Phase"]
        MOD10 --> RM[Remove Processed Ops]
        MOD10 --> DR[Remove Dropped Ops]
    end

    style Timer fill:#e8f5e9
    style Collect fill:#fff3e0
    style Process fill:#e3f2fd
    style Modules fill:#f3e5f5
    style Cleanup2 fill:#fce4ec
```

## Error Handling and Removal Paths

### Client-Side Validation Errors

| Error Code | Trigger | Module | Action |
|-----------|---------|--------|--------|
| `INVALID_FIELDS` | Bad gas limits, fees, nonce | `ValidateOpValues` | Reject, don't add to mempool |
| `REJECTED_BY_EP_OR_ACCOUNT` | Simulation reverts | `SimulateOp` | Reject, don't add to mempool |
| `INVALID_SIGNATURE` | Signature check fails | `SimulateOp` | Reject, don't add to mempool |
| `BANNED_OPCODE` | Banned opcode detected | `SimulateOp` | Reject, don't add to mempool |
| `SHORT_DEADLINE` | Expires within 30s | `SimulateOp` | Reject, don't add to mempool |

### Bundler-Side Removal Reasons

| Reason | Trigger | Module | Action |
|--------|---------|--------|--------|
| `expired` | TTL exceeded | `DropExpired` | Mark for removal |
| `underpriced` | Gas price too low | `FilterUnderpriced` | Mark for removal |
| `insufficient paymaster deposit` | Paymaster can't cover gas | `PaymasterDeposit` | Mark for removal |
| `code hash changed` | Contract code updated | `CodeHashes` | Mark for removal |
| Simulation revert | Batch simulation fails | `SimulateBatch` | Mark for removal |

```mermaid
flowchart TB
    subgraph ClientError["Client Errors"]
        CE1[Invalid Fields] --> CE_R[Return Error to Caller]
        CE2[Simulation Failed] --> CE_R
        CE3[Banned Opcode] --> CE_R
    end

    subgraph BundlerDrop["Bundler Drops"]
        BD1[Expired] --> BD_R[Mark for Removal]
        BD2[Underpriced] --> BD_R
        BD3[Bad Deposit] --> BD_R
        BD4[Code Changed] --> BD_R
        BD_R --> BD_DB[Remove from DB]
    end

    subgraph Retry["Retry Behavior"]
        R1[Gas Estimation Revert] --> R2[Drop One Op]
        R2 --> R3[Re-estimate]
        R3 -->|Success| R4[Submit]
        R3 -->|Fail| R2
    end

    style ClientError fill:#ffebee
    style BundlerDrop fill:#fff3e0
    style Retry fill:#e8f5e9
```

## Reputation System Flow

```mermaid
flowchart TB
    subgraph Entities["Tracked Entities"]
        S[Sender]
        F[Factory]
        P[Paymaster]
    end

    subgraph Staking["EntryPoint Staking"]
        S -->|Check| SD[Deposit Info]
        F -->|Check| FD[Deposit Info]
        P -->|Check| PD[Deposit Info]
    end

    subgraph Scoring["Reputation Scoring"]
        SD -->|Influence| R[Reputation Score]
        FD -->|Influence| R
        PD -->|Influence| R
    end

    subgraph Actions["Actions"]
        R -->|Banned| A1[Reject All Ops]
        R -->|Throttled| A2[Limit Ops per Block]
        R -->|OK| A3[Accept Normally]
    end

    style Entities fill:#e3f2fd
    style Staking fill:#e8f5e9
    style Scoring fill:#fff3e0
    style Actions fill:#fce4ec
```

## Mempool State Transitions

```mermaid
stateDiagram-v2
    [*] --> Pending: AddOp
    Pending --> Pending: ReplaceOp (same nonce)
    Pending --> Processing: Bundler Dump
    Processing --> Included: On-chain Success
    Processing --> Failed: On-chain Revert
    Processing --> Dropped: Validation Failure
    Included --> [*]: RemoveOps
    Failed --> Pending: Retry
    Failed --> [*]: Max Retries
    Dropped --> [*]: RemoveOps
```

## Gas Price Update Flow

The bundler fetches gas prices at the start of each batch processing cycle:

```mermaid
sequenceDiagram
    participant B as Bundler
    participant ETH as Ethereum Node

    B->>ETH: eth_getBlockByNumber("latest")
    ETH-->>B: Block with baseFeePerGas
    B->>B: Extract BaseFee

    B->>ETH: eth_maxPriorityFeePerGas
    ETH-->>B: Suggested tip
    B->>B: Extract GasTip

    B->>ETH: eth_gasPrice
    ETH-->>B: Legacy gas price
    B->>B: Extract LegacyGasPrice

    B->>B: Create BatchHandlerCtx<br/>with gas values
```

## Receipt Query Flow

```mermaid
sequenceDiagram
    actor W as Wallet/DApp
    participant API as JSON-RPC Handler
    participant C as Client
    participant ETH as Ethereum Node

    W->>API: eth_getUserOperationReceipt
    API->>C: GetUserOperationReceipt(hash)
    
    C->>ETH: Query UserOperationEvent logs
    ETH-->>C: Matching events
    
    alt Found
        C->>ETH: Get transaction receipt
        ETH-->>C: TX receipt with logs
        C-->>API: Formatted receipt
        API-->>W: Receipt JSON
    else Not Found
        C-->>API: nil
        API-->>W: {"result": null}
    end
```
