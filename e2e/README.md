# End-to-End Tests (ERC-4337 v0.9)

TypeScript test suite that drives the qubenode-bundler against a locally forked
Ethereum mainnet or Arbitrum One.

## Layout

| Path | Purpose |
| --- | --- |
| `src/config.ts` | Per-chain config + canonical EntryPoint address. |
| `src/anvil.ts` | Wrappers for `anvil_setCode`, `anvil_setBalance`, etc. |
| `src/bundler-rpc.ts` | JSON-RPC client for the bundler's RPC surface. |
| `src/v09-userop.ts` | TS port of `pkg/userop/v09` — hashing, packing, JSON shape. |
| `src/signer.ts` | UserOp + EIP-7702 signing helpers. |
| `src/accounts.ts` | SimpleAccount / SimpleAccountFactory helpers. |
| `src/artifacts.ts` | Loads contract artifacts from `e2e/artifacts/`. |
| `src/fixture.ts` | Test fixture: installs EntryPoint, deploys factory. |
| `src/fetch-artifacts.ts` | One-time script that pulls v0.9 artifacts from npm. |
| `test/ethereum.test.ts` | Mainnet-fork tests (RPC surface, paymaster sig, 7702). |
| `test/arbitrum.test.ts` | Arbitrum-fork tests (L1 calldata PVG buffer, L2 receipt). |

## Prerequisites

- `node` and `npm` (for the test runner)
- `anvil` (from foundry — used as the local chain)
- `go` (to build the bundler)
- An RPC URL for the fork you want to run

## Running

The orchestration script in `scripts/e2e/run.sh` does everything:

```sh
# Ethereum mainnet fork suite
ETH_MAINNET_RPC_URL=https://your-node make e2e-ethereum

# Arbitrum One mainnet fork suite
ARB_MAINNET_RPC_URL=https://your-node make e2e-arbitrum

# Both, sequentially
ETH_MAINNET_RPC_URL=... ARB_MAINNET_RPC_URL=... make e2e
```

Equivalent direct invocation:

```sh
scripts/e2e/run.sh ethereum
scripts/e2e/run.sh arbitrum
scripts/e2e/run.sh all
```

If `*_RPC_URL` is unset, anvil starts without a fork. Most tests still pass
because the suite only needs the v0.9 EntryPoint bytecode (installed via
`anvil_setCode`) and a freshly-deployed SimpleAccountFactory; tests that
require live mainnet state degrade gracefully.

## How the script bootstraps state

1. Boot anvil on `127.0.0.1:8545` (or `:8546` for Arbitrum). With a fork URL,
   anvil mirrors mainnet/Arbitrum state.
2. Run `npm install` and fetch v0.9 artifacts (`EntryPoint`,
   `SimpleAccount`, `SimpleAccountFactory`, `Simple7702Account`) from the
   `@account-abstraction/contracts@0.9.0-rc.1` npm package on first run.
   Artifacts cache in `e2e/artifacts/`.
3. `anvil_setCode` installs the v0.9 `EntryPoint` deployedBytecode at the
   canonical address `0x4337…d009`.
4. Deploy a fresh `SimpleAccountFactory` from anvil account #9 (the bundler
   relayer), and persist its address in `.e2e-state.json` so the bundler
   and the tests use the same factory.
5. Build the bundler binary (`go build`), then start it on port `:4337`
   (or `:4338`) wired to the anvil node, with the v0.9 EntryPoint in
   `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` and (for Arbitrum)
   `ERC4337_BUNDLER_IS_ARB_STACK_NETWORK=true`.
6. Wait for `GET /ping` to return 200, then run `vitest`.

## Bundler logs

When a run fails, the bundler stdout/stderr is captured in
`tmp/e2e/bundler-<chain>.log` and anvil's in `tmp/e2e/anvil-<chain>.log`.
