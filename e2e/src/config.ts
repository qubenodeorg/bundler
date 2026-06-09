import { getAddress } from "ethers";

export type ChainName = "ethereum" | "arbitrum";

export interface ChainConfig {
  name: ChainName;
  chainId: number;
  forkUrlEnv: string;
  defaultPort: number;
  bundlerPort: number;
}

export const CHAINS: Record<ChainName, ChainConfig> = {
  ethereum: {
    name: "ethereum",
    chainId: 1,
    forkUrlEnv: "ETH_MAINNET_RPC_URL",
    defaultPort: 8545,
    bundlerPort: 4337,
  },
  arbitrum: {
    name: "arbitrum",
    chainId: 42161,
    forkUrlEnv: "ARB_MAINNET_RPC_URL",
    defaultPort: 8546,
    bundlerPort: 4338,
  },
};

// Canonical v0.9 EntryPoint address. The e2e bootstrap script installs the
// deployed bytecode at this address using anvil_setCode so the tests can
// pretend the official deployment exists on the forked chain.
export const ENTRY_POINT_V09 = getAddress(
  "0x433709009B8330FDa32311DF1C2AFA402eD8D009",
);

// Anvil's deterministic account #0 — funded with 10000 ETH by default. Used as
// the EOA owner that signs user operations in the tests.
export const ANVIL_ACCOUNT_0 = {
  address: getAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
  privateKey:
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
};

// Anvil's deterministic account #9 — used as the bundler relayer EOA.
export const ANVIL_ACCOUNT_9 = {
  address: getAddress("0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"),
  privateKey:
    "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6",
};

export interface EnvConfig {
  anvilRpcUrl: string;
  bundlerRpcUrl: string;
  artifactsDir: string;
  chain: ChainName;
}

function getRequired(name: string): string {
  const v = process.env[name];
  if (!v) {
    throw new Error(`required env var ${name} is not set`);
  }
  return v;
}

export function loadEnv(): EnvConfig {
  const chain = (process.env.E2E_CHAIN ?? "ethereum") as ChainName;
  if (!CHAINS[chain]) {
    throw new Error(`unsupported E2E_CHAIN=${chain}`);
  }
  return {
    chain,
    anvilRpcUrl: getRequired("E2E_ANVIL_URL"),
    bundlerRpcUrl: getRequired("E2E_BUNDLER_URL"),
    artifactsDir:
      process.env.E2E_ARTIFACTS_DIR ?? new URL("../artifacts", import.meta.url).pathname,
  };
}
