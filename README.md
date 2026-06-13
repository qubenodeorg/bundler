# Qubenode Bundler

![core status](https://github.com/zarbanio/bundler/actions/workflows/core.yml/badge.svg?branch=main)

A fast, reliable, and modular Go implementation of an ERC-4337 Bundler with
support for EntryPoint **v0.6, v0.7, v0.8, and v0.9** from a single binary.

## Supported Versions

| Version | EntryPoint | Highlights |
|---------|------------|------------|
| v0.6 | `0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789` | Original packed `initCode` / `paymasterAndData` |
| v0.7 | `0x0000000071727De22E5E9d8BAf0edAc6f37da032` | Split `factory`/`paymaster` fields, off-chain validation, 10% unused-gas penalty |
| v0.8 | `0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108` | EIP-7702 delegated EOAs, ERC-712 `userOpHash` |
| v0.9 | `0x433709009B8330FDa32311DF1C2AFA402eD8D009` | Detached `paymasterSignature`, block-number validity |

The version is auto-detected from the configured EntryPoint address and can be
pinned with `ERC4337_BUNDLER_ENTRYPOINT_VERSION`. See the
[Migration Guide](./MIGRATION.md) for upgrading from a v0.6-only deployment and
the [breaking changes](./MIGRATION.md#breaking-changes) notice.

> **Note:** v0.9 support tracks the `v0.9.0` account-abstraction contracts and is
> experimental until upstream ships a stable v0.9 release.

## Quick Start

```bash
# Build
make build

# Generate environment file
make generate-environment

# Edit .env with your configuration

# Run in private mode
./qubenode-bundler start --mode private
```

## Documentation

- [Architecture](./docs/ARCHITECTURE.md) - System architecture and component overview
- [Data Flow](./docs/DATA_FLOW.md) - UserOperation lifecycle and processing flows
- [Configuration](./docs/CONFIGURATION.md) - Environment variables and deployment options
- [API Reference](./docs/API.md) - JSON-RPC endpoints and paymaster API
- [Modules](./docs/MODULES.md) - Middleware module system guide
- [Migration Guide](./MIGRATION.md) - Upgrading from v0.6 to v0.7/v0.8/v0.9
- [Development](./docs/DEVELOPMENT.md) - Developer setup and contribution guide
- [GoDoc](./docs/GODOC.md) - Auto-generated package documentation

## Prerequisites

- Go 1.20 or later
- Access to a node with `debug` API enabled for custom tracing

## Setup

```bash
# Installs https://github.com/air-verse/air for live reloading.
# Runs go mod tidy.
make install-dev

# Generates base .env file.
# All variables in this file are required and should be filled.
# Running this command WILL override current .env file.
make generate-environment

# Parses private key in .env file and prints public key and address.
make fetch-wallet
```

## Run bundler in `private` mode

Start a local bundler instance:

```bash
make dev-private-mode
```

If you need to reset the embedded database:

```bash
# This will delete the default data directory at /tmp/qubenode_bundler
make dev-reset-default-data-dir
```

## Run bundler in `searcher` mode

```bash
make dev-searcher-mode
```

## Docker

```bash
docker build -t qubenode-bundler .
docker run -e ERC4337_BUNDLER_ETH_CLIENT_URL=... \
           -e ERC4337_BUNDLER_PRIVATE_KEY=... \
           -p 4337:4337 \
           qubenode-bundler
```

## License

Distributed under the GPL-3.0 License. See [LICENSE](./LICENSE) for more information.

## Support

For technical questions and support, please open an issue on GitHub.
