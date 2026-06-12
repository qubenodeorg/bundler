import { fileURLToPath } from "node:url";

import { getAddress } from "ethers";

// Canonical v0.9 EntryPoint address (account-abstraction v0.9.0 deterministic
// deployment). The local geth --dev chain CREATE2-deploys the npm artifact's
// bytecode instead, which constructs at a different (but equally
// deterministic) address — see deploy-entrypoint.ts. The actual address for a
// run is plumbed through the E2E_ENTRY_POINT env var.
export const CANONICAL_ENTRY_POINT_V09 = getAddress(
  "0x433709009B8330FDa32311DF1C2AFA402eD8D009",
);

export const ENTRY_POINT_V09 = getAddress(
  process.env.E2E_ENTRY_POINT ?? CANONICAL_ENTRY_POINT_V09,
);

// Deterministic-deployment proxy (Arachnid CREATE2 proxy). On a fresh geth
// --dev chain it is installed with the canonical keyless (Nick's method)
// transaction: fund the one-time deployer address and broadcast the presigned
// tx. Requires the node to accept pre-EIP-155 transactions
// (--rpc.allow-unprotected-txs).
export const CREATE2_PROXY = getAddress(
  "0x4e59b44847b379578588920cA78FbF26c0B4956C",
);
export const CREATE2_PROXY_DEPLOYER = getAddress(
  "0x3fAB184622Dc19b6109349B94811493BF2a45362",
);
// 100k gas at 100 gwei — the exact budget the presigned tx spends.
export const CREATE2_PROXY_DEPLOY_COST = 100_000n * 100_000_000_000n;
export const CREATE2_PROXY_DEPLOY_TX =
  // RLP header + nonce 0, gasPrice 100 gwei, gas 100k, no to, no value
  "0xf8a58085174876e800830186a08080b853" +
  // init code: copies the runtime below into place
  "604580600e600039806000f350fe" +
  // runtime: PUSH32 0xff..e0 mask + CREATE2 dispatcher
  "7f" + "f".repeat(62) + "e0" +
  "3601600081602082378035828234f58015156039578182fd5b80825250505060" +
  "14600cf3" +
  // keyless signature (v=27, r=s=0x2222...22)
  "1b" + "a0" + "22".repeat(32) + "a0" + "22".repeat(32);

// Salt used by account-abstraction v0.9.0 for the EntryPoint deterministic
// deployment (hardhat.config.ts SALT).
export const ENTRY_POINT_V09_SALT =
  "0x7702864008ddeab30aa67b7adc3d2653bc8d162714b1fe8fe4582df814f3bf61";

// Well-known test key (anvil/hardhat account #0) — the EOA owner that signs
// user operations in the tests. Funded from the geth dev account at setup.
export const OWNER_ACCOUNT = {
  address: getAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
  privateKey:
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
};

// Well-known test key (anvil/hardhat account #9) — the bundler relayer EOA.
// Must match RELAYER_PK in scripts/e2e/run.sh.
export const RELAYER_ACCOUNT = {
  address: getAddress("0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"),
  privateKey:
    "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6",
};

export interface EnvConfig {
  nodeRpcUrl: string;
  bundlerRpcUrl: string;
  artifactsDir: string;
}

function getRequired(name: string): string {
  const v = process.env[name];
  if (!v) {
    throw new Error(`required env var ${name} is not set`);
  }
  return v;
}

export function loadEnv(): EnvConfig {
  return {
    nodeRpcUrl: getRequired("E2E_NODE_URL"),
    bundlerRpcUrl: getRequired("E2E_BUNDLER_URL"),
    artifactsDir:
      process.env.E2E_ARTIFACTS_DIR ??
      fileURLToPath(new URL("../artifacts", import.meta.url)),
  };
}
