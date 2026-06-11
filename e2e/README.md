# End-to-End Tests (ERC-4337 v0.9)

TypeScript test suite that drives the qubenode-bundler against a local
`geth --dev` chain. No forks and no external RPC URLs: the chain starts empty
and everything the suite needs is deployed during setup.

## Layout

| Path | Purpose |
| --- | --- |
| `src/config.ts` | Test config, well-known keys, CREATE2 proxy constants. |
| `src/node.ts` | geth dev-mode helpers (funding from the dev account). |
| `src/bundler-rpc.ts` | JSON-RPC client for the bundler's RPC surface. |
| `src/v09-userop.ts` | TS port of `pkg/userop/v09` — hashing, packing, JSON shape. |
| `src/signer.ts` | UserOp + EIP-7702 signing helpers. |
| `src/accounts.ts` | SimpleAccount / SimpleAccountFactory helpers. |
| `src/artifacts.ts` | Loads contract artifacts from `e2e/artifacts/`. |
| `src/deploy-entrypoint.ts` | CREATE2-deploys the v0.9 EntryPoint (and Arachnid proxy). |
| `src/fixture.ts` | Test fixture: deploys EntryPoint + factory, funds accounts. |
| `src/fetch-artifacts.ts` | One-time script that pulls v0.9 artifacts from npm. |
| `test/ethereum.test.ts` | The suite (RPC surface, send + receipt, paymaster sig, 7702). |

## Prerequisites

- `node` and `npm` (for the test runner)
- `go` (to build the bundler)
- `geth` **or** `docker` (run.sh prefers a local geth binary and falls back to
  the pinned `ethereum/client-go` image)

## Running

The orchestration script in `scripts/e2e/run.sh` does everything:

```sh
make e2e
# or directly:
scripts/e2e/run.sh
```

## How the script bootstraps state

1. Build the bundler binary (`go build`).
2. Boot `geth --dev` on `127.0.0.1:8545` with
   `--http.api eth,net,web3,debug --rpc.allow-unprotected-txs`. Dev mode mines
   on demand and exposes one unlocked, pre-funded developer account; it also
   ships the native `erc7562Tracer`/`callTracer`, so the bundler runs its
   default native-tracer path.
3. Run `npm install` and fetch v0.9 artifacts (`EntryPoint`, `SimpleAccount`,
   `SimpleAccountFactory`, `Simple7702Account`) from the
   `@account-abstraction/contracts` npm package on first run. Artifacts cache
   in `e2e/artifacts/`.
4. CREATE2-deploy the v0.9 `EntryPoint` for real: install the Arachnid
   deterministic-deployment proxy via its canonical keyless transaction, then
   construct the EntryPoint through it (constructor-set immutables like
   `senderCreator` must be valid, so `deployedBytecode` injection is not an
   option). The resulting address is passed to the bundler and the tests via
   `E2E_ENTRY_POINT`.
5. Start the bundler on `:4337` wired to the geth node, with the EntryPoint in
   `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` and
   `ERC4337_BUNDLER_ENTRYPOINT_VERSION=v0.9`.
6. Wait for `GET /ping` to return 200, then run `vitest`. The fixture funds
   the owner/relayer accounts from the geth dev account and deploys a
   `SimpleAccountFactory`, persisting its address in `.e2e-state.json` so the
   bundler and the tests use the same factory.

## Logs

When a run fails, the bundler stdout/stderr is captured in
`tmp/e2e/bundler.log` and geth's in `tmp/e2e/geth.log`.
