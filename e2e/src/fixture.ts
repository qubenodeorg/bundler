import fs from "node:fs";
import path from "node:path";

import { JsonRpcProvider, Wallet, parseEther } from "ethers";

import { AnvilCheats } from "./anvil.js";
import { BundlerRpc } from "./bundler-rpc.js";
import { FactoryClient, deployFactory, fundAccount } from "./accounts.js";
import {
  ANVIL_ACCOUNT_0,
  ANVIL_ACCOUNT_9,
  ENTRY_POINT_V09,
  EnvConfig,
  loadEnv,
} from "./config.js";
import { loadArtifact } from "./artifacts.js";

export interface E2EContext {
  env: EnvConfig;
  provider: JsonRpcProvider;
  anvil: AnvilCheats;
  bundler: BundlerRpc;
  owner: Wallet;
  relayer: Wallet;
  factory: FactoryClient;
  chainId: bigint;
}

// setup installs the v0.9 EntryPoint deployedBytecode at the canonical address,
// deploys (or reuses) the SimpleAccountFactory, and funds the test accounts.
// It is idempotent — multiple calls only re-fund balances.
export async function setupE2EContext(): Promise<E2EContext> {
  const env = loadEnv();
  const provider = new JsonRpcProvider(env.anvilRpcUrl);
  const anvil = new AnvilCheats(provider);
  const bundler = new BundlerRpc(env.bundlerRpcUrl);

  const network = await provider.getNetwork();
  const chainId = network.chainId;

  const owner = new Wallet(ANVIL_ACCOUNT_0.privateKey, provider);
  const relayer = new Wallet(ANVIL_ACCOUNT_9.privateKey, provider);

  await ensureEntryPointInstalled(anvil);
  await anvil.setBalance(owner.address, parseEther("10000"));
  await anvil.setBalance(relayer.address, parseEther("10000"));

  const factoryAddress = await ensureFactoryDeployed(relayer);
  const factory = new FactoryClient(provider, factoryAddress);

  return { env, provider, anvil, bundler, owner, relayer, factory, chainId };
}

// ensureEntryPointInstalled fast-paths when the address already has code (e.g.
// a prior test in the same anvil session already injected it).
async function ensureEntryPointInstalled(anvil: AnvilCheats): Promise<void> {
  const provider = anvil.provider;
  const existing = await provider.getCode(ENTRY_POINT_V09);
  if (existing && existing !== "0x") return;
  const art = loadArtifact("EntryPoint");
  await anvil.setCode(ENTRY_POINT_V09, art.deployedBytecode);
}

// ensureFactoryDeployed reads the deployed factory address from a side-channel
// file written by scripts/e2e/run.sh (so the bundler and the tests agree on
// the same address). If absent, the first test that needs a factory deploys
// one and writes the file.
async function ensureFactoryDeployed(deployer: Wallet): Promise<string> {
  const stateFile =
    process.env.E2E_STATE_FILE ??
    path.join(process.cwd(), ".e2e-state.json");
  if (fs.existsSync(stateFile)) {
    const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
    // Only reuse a cached factory address if it still has code on-chain — a
    // stale .e2e-state.json from a prior anvil session would otherwise point at
    // an undeployed address and fail on first interaction.
    if (typeof state.factory === "string") {
      const code = await deployer.provider!.getCode(state.factory);
      if (code && code !== "0x") return state.factory;
    }
  }
  const address = await deployFactory(deployer);
  const state = fs.existsSync(stateFile)
    ? JSON.parse(fs.readFileSync(stateFile, "utf8"))
    : {};
  state.factory = address;
  fs.writeFileSync(stateFile, JSON.stringify(state, null, 2));
  return address;
}

// fundCounterfactual ensures the smart-account address has enough native ETH
// to cover prefund before its factory has run. Without a paymaster the
// EntryPoint expects the sender to hold the prefund balance.
export async function fundCounterfactual(
  ctx: E2EContext,
  sender: string,
  amountWei: bigint,
): Promise<void> {
  const current = await ctx.provider.getBalance(sender);
  if (current >= amountWei) return;
  await fundAccount(ctx.relayer, sender, amountWei - current);
}
