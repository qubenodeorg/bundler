# API Reference

This document describes the JSON-RPC API exposed by the qubenode-bundler. The
endpoints follow the [ERC-4337 specification](https://eips.ethereum.org/EIPS/eip-4337)
and the bundler supports **v0.6, v0.7, v0.8, and v0.9** EntryPoints from a single
binary.

## Base URL

```
POST http://localhost:4337/
POST http://localhost:4337/rpc
```

## Content Type

```
Content-Type: application/json
```

## Version Awareness

The JSON-RPC method names and shapes are identical across all supported
EntryPoint versions — what changes is the **UserOperation field layout**. The
bundler picks the version from the `entryPoint` address passed in the request
(see [Version Detection](./CONFIGURATION.md#entrypoint-version-detection)):

| EntryPoint address | Version | UserOperation format |
|--------------------|---------|----------------------|
| `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` | v0.6 | `initCode`, `paymasterAndData` (packed) |
| `0x0000000071727De22E5E9d8BAf0edAc6f37da032` | v0.7 | `factory`/`factoryData`, split paymaster fields |
| `0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108` | v0.8 | v0.7 fields + `eip7702Auth`, ERC-712 hashing |
| `0x433709009B8330FDa32311DF1C2AFA402eD8D009` | v0.9 | v0.8 fields + `paymasterSignature` |

The field layout is auto-detected from the request body as well (presence of
`factory`, `eip7702Auth`, or `paymasterSignature`), so callers can mix versions
against a multi-EntryPoint bundler. The sections below show the v0.6 (legacy)
shape first, followed by the expanded v0.7+ shape.

## Standard ERC-4337 Methods

### `eth_sendUserOperation`

Submit a `UserOperation` to the bundler's mempool.

**Parameters:**
- `userOperation` (object): The UserOperation to submit
- `entryPoint` (string): EntryPoint contract address

**UserOperation Fields (v0.6):**

| Field | Type | Description |
|-------|------|-------------|
| `sender` | `address` | Smart account address |
| `nonce` | `uint256` | Account nonce |
| `initCode` | `bytes` | Contract initialization code (empty for existing accounts) |
| `callData` | `bytes` | Transaction calldata |
| `callGasLimit` | `uint256` | Gas limit for execution phase |
| `verificationGasLimit` | `uint256` | Gas limit for verification phase |
| `preVerificationGas` | `uint256` | Gas overhead for bundler |
| `maxFeePerGas` | `uint256` | Maximum fee per gas (EIP-1559) |
| `maxPriorityFeePerGas` | `uint256` | Maximum priority fee per gas (EIP-1559) |
| `paymasterAndData` | `bytes` | Paymaster address and data (empty if no paymaster) |
| `signature` | `bytes` | Account signature |

**UserOperation Fields (v0.7+):**

Starting with v0.7 the packed `initCode` and `paymasterAndData` fields are split
into discrete fields. Optional groups (`factory*`, `paymaster*`) may be omitted
entirely when unused.

| Field | Type | Versions | Description |
|-------|------|----------|-------------|
| `sender` | `address` | all | Smart account address |
| `nonce` | `uint256` | all | Account nonce |
| `factory` | `address` | v0.7+ | Account factory address (replaces `initCode` prefix). Omit for existing accounts. The marker `0x7702` denotes an EIP-7702 delegated EOA sender. |
| `factoryData` | `bytes` | v0.7+ | Calldata passed to the factory (replaces `initCode` suffix) |
| `callData` | `bytes` | all | Transaction calldata |
| `callGasLimit` | `uint256` | all | Gas limit for execution phase |
| `verificationGasLimit` | `uint256` | all | Gas limit for verification phase |
| `preVerificationGas` | `uint256` | all | Gas overhead for bundler |
| `maxFeePerGas` | `uint256` | all | Maximum fee per gas (EIP-1559) |
| `maxPriorityFeePerGas` | `uint256` | all | Maximum priority fee per gas (EIP-1559) |
| `paymaster` | `address` | v0.7+ | Paymaster address (replaces `paymasterAndData` prefix). Omit when unsponsored. |
| `paymasterVerificationGasLimit` | `uint256` | v0.7+ | Gas limit for the paymaster's `validatePaymasterUserOp` |
| `paymasterPostOpGasLimit` | `uint256` | v0.7+ | Gas limit for the paymaster's `postOp` |
| `paymasterData` | `bytes` | v0.7+ | Paymaster-specific data (replaces `paymasterAndData` suffix) |
| `eip7702Auth` | `object` | v0.8+ | EIP-7702 authorization tuple (`chainId`, `address`, `nonce`, `yParity`, `r`, `s`). Required when `factory` is the `0x7702` marker. |
| `paymasterSignature` | `bytes` | v0.9 | Paymaster signature, excluded from the `userOpHash`. Optional. |
| `signature` | `bytes` | all | Account signature |

**Request (v0.6):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_sendUserOperation",
  "params": [
    {
      "sender": "0x...",
      "nonce": "0x0",
      "initCode": "0x...",
      "callData": "0x...",
      "callGasLimit": "0x...",
      "verificationGasLimit": "0x...",
      "preVerificationGas": "0x...",
      "maxFeePerGas": "0x...",
      "maxPriorityFeePerGas": "0x...",
      "paymasterAndData": "0x",
      "signature": "0x..."
    },
    "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"
  ]
}
```

**Request (v0.9 expanded format, with EIP-7702 and paymaster signature):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_sendUserOperation",
  "params": [
    {
      "sender": "0x...",
      "nonce": "0x0",
      "factory": "0x7702",
      "factoryData": "0x",
      "callData": "0x...",
      "callGasLimit": "0x...",
      "verificationGasLimit": "0x...",
      "preVerificationGas": "0x...",
      "maxFeePerGas": "0x...",
      "maxPriorityFeePerGas": "0x...",
      "paymaster": "0x...",
      "paymasterVerificationGasLimit": "0x...",
      "paymasterPostOpGasLimit": "0x...",
      "paymasterData": "0x...",
      "eip7702Auth": {
        "chainId": "0x1",
        "address": "0x...",
        "nonce": "0x0",
        "yParity": 0,
        "r": "0x...",
        "s": "0x..."
      },
      "paymasterSignature": "0x...",
      "signature": "0x..."
    },
    "0x433709009B8330FDa32311DF1C2AFA402eD8D009"
  ]
}
```

> For an account that already exists and is unsponsored, omit the `factory*`,
> `paymaster*`, `eip7702Auth`, and `paymasterSignature` fields entirely.

**Response (Success):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1a2b3c..."
}
```

**Response (Error):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid UserOp signature or paymaster signature",
    "data": null
  }
}
```

**Error Codes:**

| Code | Name | Description |
|------|------|-------------|
| `-32602` | `INVALID_FIELDS` | Invalid field values in UserOperation |
| `-32500` | `REJECTED_BY_EP_OR_ACCOUNT` | Rejected by EntryPoint or account |
| `-32501` | `REJECTED_BY_PAYMASTER` | Rejected by paymaster |
| `-32502` | `BANNED_OPCODE` | Banned opcode detected |
| `-32503` | `INVALID_SIGNATURE` | Invalid signature |
| `-32504` | `SHORT_DEADLINE` | UserOperation expires too soon |

### `eth_estimateUserOperationGas`

Estimate gas values for a `UserOperation`.

**Parameters:**
- `userOperation` (object): The UserOperation to estimate
- `entryPoint` (string): EntryPoint contract address
- `stateOverrideSet` (object, optional): State override for simulation

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_estimateUserOperationGas",
  "params": [
    {
      "sender": "0x...",
      "nonce": "0x0",
      "initCode": "0x",
      "callData": "0x...",
      "callGasLimit": "0x...",
      "verificationGasLimit": "0x...",
      "preVerificationGas": "0x...",
      "maxFeePerGas": "0x0",
      "maxPriorityFeePerGas": "0x0",
      "paymasterAndData": "0x",
      "signature": "0x..."
    },
    "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"
  ]
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "preVerificationGas": "0x...",
    "verificationGasLimit": "0x...",
    "callGasLimit": "0x...",
    "paymasterVerificationGasLimit": "0x...",
    "paymasterPostOpGasLimit": "0x...",
    "verificationGas": "0x..."
  }
}
```

**Notes:**
- If `maxFeePerGas` is `0`, the bundler will fetch current gas prices for estimation
- The `signature` field should contain a dummy signature of correct length
- `paymasterVerificationGasLimit` and `paymasterPostOpGasLimit` are returned only
  for v0.7+ requests that include a `paymaster`; they are omitted otherwise
- `verificationGas` is a deprecated v0.6 alias for `verificationGasLimit`, kept
  for backward compatibility

### `eth_getUserOperationReceipt`

Get the receipt for a submitted `UserOperation`.

**Parameters:**
- `userOpHash` (string): Hash returned by `eth_sendUserOperation`

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_getUserOperationReceipt",
  "params": ["0x1a2b3c..."]
}
```

**Response (Found):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "userOpHash": "0x...",
    "entryPoint": "0x...",
    "sender": "0x...",
    "nonce": "0x...",
    "paymaster": "0x...",
    "actualGasCost": "0x...",
    "actualGasUsed": "0x...",
    "success": true,
    "reason": "0x",
    "logs": [...],
    "receipt": {
      "transactionHash": "0x...",
      "transactionIndex": "0x...",
      "blockHash": "0x...",
      "blockNumber": "0x...",
      "from": "0x...",
      "to": "0x...",
      "cumulativeGasUsed": "0x...",
      "gasUsed": "0x...",
      "contractAddress": null,
      "logs": [...],
      "logsBloom": "0x...",
      "status": "0x1"
    }
  }
}
```

**Response (Not Found):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": null
}
```

### `eth_getUserOperationByHash`

Get a `UserOperation` by its hash.

**Parameters:**
- `userOpHash` (string): Hash returned by `eth_sendUserOperation`

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_getUserOperationByHash",
  "params": ["0x1a2b3c..."]
}
```

**Response (Found):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "userOperation": {
      "sender": "0x...",
      "nonce": "0x...",
      "initCode": "0x...",
      "callData": "0x...",
      "callGasLimit": "0x...",
      "verificationGasLimit": "0x...",
      "preVerificationGas": "0x...",
      "maxFeePerGas": "0x...",
      "maxPriorityFeePerGas": "0x...",
      "paymasterAndData": "0x...",
      "signature": "0x..."
    },
    "entryPoint": "0x...",
    "blockNumber": "0x...",
    "blockHash": "0x...",
    "transactionHash": "0x..."
  }
}
```

### `eth_supportedEntryPoints`

Get the list of supported EntryPoint addresses.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_supportedEntryPoints",
  "params": []
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": ["0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"]
}
```

### `eth_chainId`

Get the chain ID the bundler is connected to.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_chainId",
  "params": []
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1"
}
```

## Paymaster Methods

### `pm_accounts`

Get supported paymaster accounts for an EntryPoint.

**Parameters:**
- `entryPoint` (string): EntryPoint contract address

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pm_accounts",
  "params": ["0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"]
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": ["0x2b9755b522757931981ca5b90e821786B75382AB"]
}
```

### `pm_sponsorUserOperation`

Sponsor a `UserOperation` with a paymaster.

**Parameters:**
- `userOperation` (object): The UserOperation to sponsor
- `entryPoint` (string): EntryPoint contract address
- `context` (object): Sponsorship context

**Context Types:**

| Type | Description |
|------|-------------|
| `payg` | Pay-as-you-go sponsorship |

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pm_sponsorUserOperation",
  "params": [
    {
      "sender": "0x...",
      "nonce": "0x0",
      "initCode": "0x...",
      "callData": "0x...",
      "callGasLimit": "0x...",
      "verificationGasLimit": "0x...",
      "preVerificationGas": "0x...",
      "maxFeePerGas": "0x...",
      "maxPriorityFeePerGas": "0x...",
      "paymasterAndData": "0x",
      "signature": "0x..."
    },
    "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789",
    {
      "type": "payg"
    }
  ]
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "paymasterAndData": "0x...",
    "preVerificationGas": "0x...",
    "verificationGasLimit": "0x...",
    "callGasLimit": "0x..."
  }
}
```

For a **v0.9** EntryPoint the paymaster signature is returned separately in a
`paymasterSignature` field instead of being appended to `paymasterAndData` (which
is the case for v0.6–v0.8). The caller copies it into the UserOperation's
`paymasterSignature` field, where it is excluded from the `userOpHash`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "paymasterAndData": "0x...",
    "paymasterSignature": "0x...",
    "preVerificationGas": "0x...",
    "verificationGasLimit": "0x...",
    "callGasLimit": "0x..."
  }
}
```

## Health Checks

The bundler exposes separate liveness and readiness probes for orchestrators.

### `GET /live`

Liveness probe. Returns `200` as long as the process is up; it has no upstream
dependencies, so a failure means the process itself is broken and should be
restarted. `GET /ping` is a backwards-compatible alias.

**Request:**

```http
GET http://localhost:4337/live
```

**Response:**

```http
HTTP/1.1 200 OK

{ "status": "ok" }
```

### `GET /ready`

Readiness probe. Checks the dependencies required to serve traffic — the
upstream eth JSON-RPC endpoint and the mempool database. Returns `200` when all
checks pass and `503 Service Unavailable` when any dependency is degraded, so a
load balancer can stop routing to the instance without it being restarted. The
probes are bounded by a short internal timeout. Check values are stable,
sanitized strings (`ok` / `unavailable`); the underlying error is written to the
server log, not the response.

**Request:**

```http
GET http://localhost:4337/ready
```

**Response (healthy):**

```http
HTTP/1.1 200 OK

{
  "ready": true,
  "checks": { "eth_client": "ok", "db": "ok" }
}
```

**Response (eth RPC unreachable):**

```http
HTTP/1.1 503 Service Unavailable

{
  "ready": false,
  "checks": { "eth_client": "unavailable", "db": "ok" }
}
```

## Error Response Format

All errors follow the JSON-RPC 2.0 error format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Error description",
    "data": "Additional error data or null"
  }
}
```

### Common Error Codes

| Code | Message | HTTP Status |
|------|---------|-------------|
| `-32700` | Parse error | 400 |
| `-32600` | Invalid Request | 400 |
| `-32601` | Method not found | 404 |
| `-32602` | Invalid params | 400 |
| `-32603` | Internal error | 500 |
| `-32500` | Rejected by EntryPoint or account | 400 |
| `-32501` | Rejected by paymaster | 400 |
| `-32502` | Banned opcode | 400 |
| `-32503` | Invalid signature | 400 |
| `-32504` | Short deadline | 400 |
| `-32505` | Banned code hash | 400 |
| `-32506` | Invalid paymasterAndData | 400 |
| `-32507` | Paymaster deposit too low | 400 |

## Rate Limiting

The bundler does not implement built-in rate limiting. Use a reverse proxy (nginx, traefik, etc.) for rate limiting if needed.

## CORS

CORS is enabled by default for all origins. In production, configure your reverse proxy to restrict CORS origins.

## WebSocket Support

WebSocket is not supported. All communication is via HTTP POST.
