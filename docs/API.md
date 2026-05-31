# API Reference

This document describes the JSON-RPC API exposed by the qubenode-bundler. All endpoints follow the [ERC-4337 v0.6 specification](https://eips.ethereum.org/EIPS/eip-4337).

## Base URL

```
POST http://localhost:4337/
POST http://localhost:4337/rpc
```

## Content Type

```
Content-Type: application/json
```

## Standard ERC-4337 Methods

### `eth_sendUserOperation`

Submit a `UserOperation` to the bundler's mempool.

**Parameters:**
- `userOperation` (object): The UserOperation to submit
- `entryPoint` (string): EntryPoint contract address

**UserOperation Fields:**

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

**Request:**
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
    "verificationGas": "0x..."
  }
}
```

**Notes:**
- If `maxFeePerGas` is `0`, the bundler will fetch current gas prices for estimation
- The `signature` field should contain a dummy signature of correct length

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

## Health Check

### `GET /ping`

Simple health check endpoint.

**Request:**
```
GET http://localhost:4337/ping
```

**Response:**
```
HTTP/1.1 200 OK
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
