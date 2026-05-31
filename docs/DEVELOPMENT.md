# Development Guide

This document provides information for developers working on the qubenode-bundler.

## Prerequisites

- [Go 1.20+](https://golang.org/dl/)
- [Node.js 18+](https://nodejs.org/) (for E2E tests)
- Access to an Ethereum node with `debug` API enabled
- [Geth](https://geth.ethereum.org/) (for local testing)

## Project Setup

```bash
# Clone the repository
git clone https://github.com/qubenode/bundler.git
cd bundler

# Install development tools and dependencies
make install-dev

# Generate environment configuration
make generate-environment
```

Edit `.env` with your configuration values. See [CONFIGURATION.md](./CONFIGURATION.md) for details.

## Development Commands

### Build

```bash
# Build binary
make build

# Or directly
go build -v -o qubenode-bundler
```

### Run

```bash
# Private mode with live reload
make dev-private-mode

# Searcher mode with live reload
make dev-searcher-mode

# Reset database
make dev-reset-default-data-dir
```

### Generate EntryPoint Bindings

After updating the ABI file:

```bash
make generate-entrypoint-pkg
```

This regenerates `pkg/entrypoint/bindings.go` from `abi/entrypoint.json`.

## Testing

### Unit Tests

```bash
go test -v ./...
```

Run specific package tests:

```bash
go test -v ./pkg/userop/
go test -v ./pkg/modules/checks/
go test -v ./pkg/mempool/
```

### E2E Tests

See [e2e/README.md](../e2e/README.md) for detailed E2E test setup.

Quick start:

```bash
# Terminal 1: Run local geth
geth --http --http.api eth,net,web3,debug --dev --allow-insecure-unlock

# Terminal 2: Deploy contracts and run tests
cd e2e
yarn install
yarn test
```

### Compliance Tests

The bundler includes compliance tests against the official ERC-4337 specification test suite. These run automatically in CI via `.github/workflows/compliance.yml`.

## Project Structure

```
.
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command
│   └── start.go           # Start subcommand
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── logger/           # Structured logging
│   ├── o11y/             # OpenTelemetry setup
│   ├── start/            # Startup logic for each mode
│   ├── testutils/        # Test mocks
│   └── utils/            # Generic utilities
├── pkg/                   # Public packages
│   ├── altmempools/      # Alternative mempool support
│   ├── arbitrum/         # Arbitrum L2 integration
│   ├── bundler/          # Batch processing engine
│   ├── client/           # JSON-RPC client implementation
│   ├── contract/         # Contract utilities
│   ├── entrypoint/       # EntryPoint contract interface
│   ├── estimator/        # Gas estimation
│   ├── gas/              # Gas calculations
│   ├── handlers/         # Paymaster handlers
│   ├── jsonrpc/          # JSON-RPC server
│   ├── mempool/          # Pending operation pool
│   ├── modules/          # Middleware system
│   ├── optimism/         # Optimism L2 integration
│   ├── signer/           # EOA key management
│   ├── tracer/           # EVM tracing
│   ├── userop/           # UserOperation types
│   └── ...
├── e2e/                   # End-to-end tests (TypeScript/Jest)
├── tests/                 # Go integration tests
├── abi/                   # Contract ABIs
├── docs/                  # Documentation
├── scripts/               # Helper scripts
├── .github/workflows/     # CI/CD
├── Makefile              # Development commands
├── Dockerfile            # Container build
└── go.mod                # Go module definition
```

## Architecture Decisions

### Why BadgerDB?

BadgerDB provides:
- Embedded operation (no external dependencies)
- Persistence across restarts
- Good write performance for our use case

### Why Modules?

The module pattern allows:
- Easy customization of validation rules
- Clean separation of concerns
- Testable individual components
- Support for different modes (private/searcher) with minimal code duplication

### Why Two Modes?

- **Private mode**: Universal compatibility, simple setup, vulnerable to frontrunning
- **Searcher mode**: MEV protection, requires compatible chain and builder access

## L2 Network Considerations

### Optimism / OP Stack

OP Stack networks require special PreVerificationGas calculations using the L1 gas oracle:

```go
ov.SetCalcPreVerificationGasFunc(
    gas.CalcOptimismPVGWithEthClient(rpc, chain, entryPoint),
)
```

### Arbitrum

Arbitrum requires a larger PVG buffer due to L1 data posting costs:

```go
ov.SetPreVerificationGasBufferFactor(16)
```

### Adding New L2 Support

1. Add chain ID to `internal/config/constants.go`
2. Add to `OpStackChains` or `ArbStackChains` set
3. Implement network-specific gas oracle in `pkg/gas/`
4. Configure in startup logic

## Adding New Validation Rules

1. Create a new module in `pkg/modules/checks/`
2. Implement validation logic
3. Add to the Client module chain in `internal/start/private.go` or `searcher.go`
4. Write unit tests in `pkg/modules/checks/*_test.go`

Example:

```go
// pkg/modules/checks/custom.go
func (s *Standalone) MyCustomCheck() modules.UserOpHandlerFunc {
    return func(ctx *modules.UserOpHandlerCtx) error {
        // Validation logic
        return nil
    }
}
```

Register in startup:

```go
c.UseModules(
    // ... existing modules
    check.MyCustomCheck(),
)
```

## Debugging

Enable debug mode to access additional endpoints and see single-op batches:

```bash
ERC4337_BUNDLER_DEBUG_MODE=true ./qubenode-bundler start --mode private
```

Debug mode:
- Sets max batch size to 1
- Enables debug RPC endpoints
- Disables relay wait timeout

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test -v ./...`
5. Run linter: `golangci-lint run`
6. Submit a pull request

## Troubleshooting

### "debug_traceCall not found"

The bundler requires a node with `debug` API enabled. Ensure your node is started with `--http.api eth,net,web3,debug`.

### "insufficient funds"

The bundler EOA needs ETH to pay for transaction gas. Fund the address shown at startup.

### Mempool not persisting

Check that `ERC4337_BUNDLER_DATA_DIRECTORY` is writable and the path exists.

### High memory usage

BadgerDB can use significant memory. For resource-constrained environments, consider tuning BadgerDB options or using a smaller value log file size.
