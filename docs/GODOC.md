# GoDoc Documentation

This project uses standard [GoDoc](https://pkg.go.dev/golang.org/x/pkgsite/cmd/pkgsite) conventions for package documentation. All exported types, functions, and methods should include comprehensive documentation comments.

## Generating Documentation

### Local Documentation Server

Generate and browse documentation locally:

```bash
make docs
```

This will:
1. Install `pkgsite` if not already installed
2. Start a local documentation server
3. Open your browser to `http://localhost:8080`

### Manual Generation

```bash
# Install pkgsite
go install golang.org/x/pkgsite/cmd/pkgsite@latest

# Run server
pkgsite -open .
```

### Static HTML Generation

For static documentation:

```bash
# Using godoc
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060

# Then visit http://localhost:6060/pkg/github.com/qubenode/bundler/
```

## Documentation Standards

### Package Comments

Every package should have a package comment at the top:

```go
// Package client provides the mediator for processing incoming UserOperations
// to the bundler.
package client
```

### Function Comments

Exported functions should have comments describing what they do:

```go
// SendUserOperation implements the method call for eth_sendUserOperation.
// It returns true if userOp was accepted otherwise returns an error.
func (i *Client) SendUserOperation(op map[string]any, ep string) (string, error) {
```

### Type Comments

Exported types should describe their purpose:

```go
// Client controls the end to end process of adding incoming UserOperations
// to the mempool. It also implements the required RPC methods as specified
// in EIP-4337.
type Client struct {
```

### Example Comments

Include examples for complex functions:

```go
// New initializes a new EIP-4337 bundler which can be extended with modules.
//
// Example:
//
//     b := bundler.New(mem, chain, entryPoints)
//     b.UseModules(
//         checks.PaymasterDeposit(),
//         relay.SendUserOperation(),
//     )
//     b.Run()
func New(mempool *mempool.Mempool, chainID *big.Int, supportedEntryPoints []common.Address) *Bundler {
```

## Key Packages

| Package | Description |
|---------|-------------|
| `pkg/client` | Incoming UserOperation handler and RPC implementation |
| `pkg/bundler` | Outgoing batch processor |
| `pkg/mempool` | Persistent pending operation pool |
| `pkg/modules` | Middleware interfaces and composition |
| `pkg/userop` | UserOperation data model |
| `pkg/entrypoint` | EntryPoint contract bindings and simulation |
| `pkg/gas` | Gas calculations and L2 adjustments |
| `pkg/signer` | EOA key management |
| `pkg/jsonrpc` | HTTP JSON-RPC server |

## Continuous Documentation

Documentation is automatically published to GitHub Pages on every push to `main` via `.github/workflows/docs.yml`.

Visit: [https://qubenode.github.io/bundler](https://qubenode.github.io/bundler)
