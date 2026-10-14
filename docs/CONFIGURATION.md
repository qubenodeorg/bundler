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
| `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` | All four canonical EntryPoints (v0.9, v0.8, v0.7, v0.6) | Comma-separated list of EntryPoint addresses |
| `ERC4337_BUNDLER_ENTRYPOINT_VERSION` | `auto` | ERC-4337 version: `auto`, `v0.6`, `v0.7`, `v0.8`, `v0.9` |
| `ERC4337_BUNDLER_BENEFICIARY` | Derived from private key | Address to receive gas fees |

#### EntryPoint Version Detection

The bundler serves ERC-4337 v0.6, v0.7, v0.8, and v0.9 **concurrently from a
single instance** — no separate process per version. Each configured EntryPoint
is served at its own version simultaneously; the per-op version (derived from
the target EntryPoint) controls UserOperation parsing, the userOpHash scheme,
gas overhead, and which validation rules apply. By default all four canonical
EntryPoints are configured, so one binary handles every version at once.

With `ERC4337_BUNDLER_ENTRYPOINT_VERSION=auto` (the default), each configured
EntryPoint's version is detected **independently** from its address:

| EntryPoint address | Detected version |
|--------------------|------------------|
| `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` | `v0.6` |
| `0x0000000071727De22E5E9d8BAf0edAc6f37da032` | `v0.7` |
| `0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108` | `v0.8` |
| `0x433709009B8330FDa32311DF1C2AFA402eD8D009` | `v0.9` |

Setting `ERC4337_BUNDLER_ENTRYPOINT_VERSION` to an explicit version **overrides**
auto-detection and pins that single version for every configured EntryPoint
address (collapsing the instance to one version). Use this when running a
non-canonical / locally deployed EntryPoint (e.g. in tests) whose address is not
in the table above — without the override, an unknown address falls back to
`v0.6`. An unsupported version string is a fatal configuration error.

If no EntryPoint is configured and the version is `auto`, the bundler defaults
to `v0.9`.

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

### Storage Configuration

The mempool, reputation counters, and code-hash cache run on a pluggable
backend. See [STORAGE.md](STORAGE.md) for the architecture and scaling model.

| Variable | Default | Description |
|----------|---------|-------------|
| `ERC4337_BUNDLER_STORE_BACKEND` | inferred | `badger`, `redis`, `valkey`, or `memory`. Inferred from `STORE_URL` (Redis/Valkey) when empty, else BadgerDB |
| `ERC4337_BUNDLER_STORE_URL` | None | Connection URL: `redis://`, `rediss://` (TLS), or `valkey://`. Use `rediss://` in production |
| `ERC4337_BUNDLER_STORE_ADDRS` | None | Comma-separated `host:port` list for Sentinel/Cluster mode |
| `ERC4337_BUNDLER_STORE_MASTER_NAME` | None | Sentinel master name (enables Sentinel mode) |
| `ERC4337_BUNDLER_STORE_USERNAME` | None | Redis/Valkey ACL username |
| `ERC4337_BUNDLER_STORE_PASSWORD` | None | Redis/Valkey password |
| `ERC4337_BUNDLER_STORE_DB` | `0` | Redis logical database index |
| `ERC4337_BUNDLER_STORE_POOL_SIZE` | driver default | Connection pool size |
| `ERC4337_BUNDLER_STORE_NAMESPACE` | None | Deployment prefix added to every key |
| `ERC4337_BUNDLER_STORE_OP_TTL_SECONDS` | `MAX_OP_TTL_SECONDS` | TTL for pending ops in a shared backend; `0` disables |

The chain ID is always part of the key namespace for Redis/Valkey, so a single
cluster can safely serve multiple chains. BadgerDB is node-local and keeps the
legacy un-namespaced layout so existing data keeps loading.

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

### v0.9 EntryPoint (version auto-detected)

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...
# v0.9 EntryPoint address → version auto-detected as v0.9
ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS=0x433709009B8330FDa32311DF1C2AFA402eD8D009
```

### Pinned version for a custom/local EntryPoint

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=http://127.0.0.1:8545
ERC4337_BUNDLER_PRIVATE_KEY=0x...
# Non-canonical address → pin the version explicitly
ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS=0xMyLocallyDeployedEntryPoint
ERC4337_BUNDLER_ENTRYPOINT_VERSION=v0.7
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

### Shared Storage (Redis/Valkey, TLS)

```bash
ERC4337_BUNDLER_ETH_CLIENT_URL=https://mainnet.infura.io/v3/YOUR_PROJECT_ID
ERC4337_BUNDLER_PRIVATE_KEY=0x...

# Use a shared backend so several replicas see the same mempool.
ERC4337_BUNDLER_STORE_BACKEND=redis
ERC4337_BUNDLER_STORE_URL=rediss://:PASSWORD@redis.example.com:6379/0
ERC4337_BUNDLER_STORE_NAMESPACE=qubenode-mainnet
# Defaults to MAX_OP_TTL_SECONDS; set explicitly to bound key growth.
ERC4337_BUNDLER_STORE_OP_TTL_SECONDS=180
```

For Sentinel, list the sentinels and the master name:

```bash
ERC4337_BUNDLER_STORE_BACKEND=redis
ERC4337_BUNDLER_STORE_ADDRS=sentinel-0:26379,sentinel-1:26379,sentinel-2:26379
ERC4337_BUNDLER_STORE_MASTER_NAME=mymaster
ERC4337_BUNDLER_STORE_PASSWORD=...
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
   - `ERC4337_BUNDLER_ENTRYPOINT_VERSION` set to a value other than `auto`, `v0.6`, `v0.7`, `v0.8`, or `v0.9`

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
