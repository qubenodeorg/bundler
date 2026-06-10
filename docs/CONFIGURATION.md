# Configuration

This document describes all configuration options for the qubenode-bundler. Configuration is loaded from environment variables, with optional `.env` file support via Viper.

## Required Variables

These variables **must** be set for the bundler to start:

| Variable | Description | Example |
|----------|-------------|---------|
| `ERC4337_BUNDLER_ETH_CLIENT_URL` | Ethereum JSON-RPC endpoint URL | `https://mainnet.infura.io/v3/...` |
| `ERC4337_BUNDLER_PRIVATE_KEY` | Bundler EOA private key (hex, with or without 0x) | `0xabc123...` |

## Optional Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_PORT` | `4337` | HTTP server port |
| `ERC4337_BUNDLER_DATA_DIRECTORY` | `/tmp/qubenode_bundler` | BadgerDB data directory |
| `ERC4337_BUNDLER_GIN_MODE` | `release` | Gin framework mode (`debug`, `release`, `test`) |

### EntryPoint Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` | `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` | Comma-separated list of EntryPoint addresses |
| `ERC4337_BUNDLER_BENEFICIARY` | Derived from private key | Address to receive gas fees |

### Gas Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_MAX_VERIFICATION_GAS` | `6000000` | Maximum verification gas limit per UserOperation |
| `ERC4337_BUNDLER_MAX_BATCH_GAS_LIMIT` | `18000000` | Maximum total gas limit per batch |
| `ERC4337_BUNDLER_MAX_OP_TTL_SECONDS` | `180` | Time-to-live for pending UserOperations (seconds) |

### Mempool Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_OP_LOOKUP_LIMIT` | `2000` | Maximum number of blocks to look back for UserOperation receipts |

### Searcher Mode (Required for `--mode searcher`)

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_ETH_BUILDER_URLS` | None | Comma-separated Flashbots/block builder URLs |
| `ERC4337_BUNDLER_BLOCKS_IN_THE_FUTURE` | `6` | Maximum blocks in the future for bundle submission |

### Tracing Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_NATIVE_BUNDLER_COLLECTOR_TRACER` | `erc7562Tracer` | Validation (ERC-7562) tracer: `erc7562Tracer` (native, default) or `js` (embedded JS fallback for nodes without native tracer support) |
| `ERC4337_BUNDLER_NATIVE_BUNDLER_EXECUTOR_TRACER` | `callTracer` | Execution/gas-estimation tracer: `callTracer` (native, default; run with `{"withLog": true}`) or `js` (embedded JS fallback) |

Native tracers run inside the node and are the standard go-ethereum / reth /
erigon path; the JS fallback executes one interpreter callback per opcode and
is far slower. Use `js` only when the node lacks `erc7562Tracer`/`callTracer`
support. The active tracers are logged at startup as `tracer configuration`.

### Observability (OpenTelemetry)

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_OTEL_SERVICE_NAME` | None | OpenTelemetry service name (enables tracing if set) |
| `ERC4337_BUNDLER_OTEL_COLLECTOR_URL` | None | OTLP collector gRPC endpoint |
| `ERC4337_BUNDLER_OTEL_COLLECTOR_HEADERS` | None | Collector headers in `key1=val1&key2=val2` format |
| `ERC4337_BUNDLER_OTEL_INSECURE_MODE` | `false` | Use insecure gRPC connection to collector |

### Alternative Mempools

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_ALT_MEMPOOL_IPFS_GATEWAY` | None | IPFS gateway URL for alternative mempool rules |
| `ERC4337_BUNDLER_ALT_MEMPOOL_IDS` | None | Comma-separated alternative mempool IDs |

### Network-Specific Flags

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_IS_OP_STACK_NETWORK` | `false` | Enable OP Stack gas oracle (Optimism, Base, Lyra, etc.) |
| `ERC4337_BUNDLER_IS_ARB_STACK_NETWORK` | `false` | Enable Arbitrum gas buffer adjustments |
| `ERC4337_BUNDLER_IS_RIP7212_SUPPORTED` | `false` | Enable RIP-7212 (secp256r1) signature validation |

### Debug Mode

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_DEBUG_MODE` | `false` | Enable debug endpoints and single-op batches |

## Configuration Examples

### Private Mode (Ethereum Mainnet)

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...
ERC4337_BUNDLER_PORT=4337
ERC4337_BUNDLER_BENEFICIARY=0x...
```

### Searcher Mode (Ethereum Mainnet)

```bash
# Required for all modes
ERC4337_BUNDLER_ETH_CLIENT_URL=https://mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...

# Searcher mode specific
ERC4337_BUNDLER_ETH_BUILDER_URLS=https://relay.flashbots.net
ERC4337_BUNDLER_BLOCKS_IN_THE_FUTURE=6
```

### Optimism Network

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://optimism-mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...
ERC4337_BUNDLER_IS_OP_STACK_NETWORK=true
```

### Arbitrum Network

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://arbitrum-mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...
ERC4337_BUNDLER_IS_ARB_STACK_NETWORK=true
```

### With Observability

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...

# OpenTelemetry
ERC4337_BUNDLER_OTEL_SERVICE_NAME=qubenode-bundler
ERC4337_BUNDLER_OTEL_COLLECTOR_URL=otel-collector:4317
ERC4337_BUNDLER_OTEL_INSECURE_MODE=true
```

## Validation Rules

The bundler validates configuration at startup and will panic with a fatal error if:

1. **Required variables missing:**
   - `ERC4337_BUNDLER_ETH_CLIENT_URL` not set
   - `ERC4337_BUNDLER_PRIVATE_KEY` not set

2. **Mode-specific missing:**
   - `ERC4337_BUNDLER_ETH_BUILDER_URLS` not set in searcher mode

3. **Inconsistent settings:**
   - `ERC4337_BUNDLER_OTEL_SERVICE_NAME` set without `ERC4337_BUNDLER_OTEL_COLLECTOR_URL`
   - `ERC4337_BUNDLER_ALT_MEMPOOL_IDS` set without `ERC4337_BUNDLER_ALT_MEMPOOL_IPFS_GATEWAY`

4. **Invalid values:**
   - Unsupported chain ID in searcher mode (only Ethereum mainnet, Goerli, Sepolia)

## Environment File

Use the provided Makefile target to generate a base `.env` file:

```bash
make generate-environment
```

This will create a `.env` file with all variables commented out. Uncomment and fill in the values you need.

**Note:** Running this command will override any existing `.env` file.

## Docker Configuration

When running in Docker, pass environment variables via `-e` flags or an env file:

```bash
docker run -e ERC4337_BUNDLER_ETH_CLIENT_URL=... \
           -e ERC4337_BUNDLER_PRIVATE_KEY=... \
           -p 4337:4337 \
           qubenode-bundler start --mode private
```

Or with an env file:

```bash
docker run --env-file .env -p 4337:4337 qubenode-bundler start --mode private
```

## CLI Flags

In addition to environment variables, the bundler supports CLI flags:

```bash
qubenode-bundler start --mode private    # Run in private mode
qubenode-bundler start --mode searcher   # Run in searcher mode
```

The `--mode` flag is **required**.

## Configuration Priority

Configuration values are loaded in the following priority (highest to lowest):

1. Environment variables
2. `.env` file (if present)
3. Default values

This means environment variables always override `.env` file values.
