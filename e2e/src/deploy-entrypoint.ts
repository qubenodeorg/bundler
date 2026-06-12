// Deploys the v0.9 EntryPoint for an e2e run and prints its address.
//
// The EntryPoint cannot be installed by injecting deployedBytecode: its
// constructor deploys the SenderCreator and bakes its address into an
// immutable, so the artifact's deployedBytecode has that slot zeroed and
// createSender() would call address(0). Instead we construct it for real
// through the deterministic CREATE2 proxy (the same mechanism as the official
// deployment), which also places the SenderCreator at the create-nonce-1
// address that EntryPointSimulations expects during bundler-side estimation.
//
// On a fresh geth --dev chain the CREATE2 proxy itself is installed first via
// the canonical keyless (Nick's method) transaction. The npm artifact
// (currently 0.9.0-rc.1) CREATE2-deploys to its own deterministic address —
// computed, not hardcoded, so bumping the artifact to the final 0.9.0
// bytecode makes it converge to the canonical address automatically.
import { JsonRpcProvider, getCreate2Address, keccak256 } from "ethers";

import {
  CANONICAL_ENTRY_POINT_V09,
  CREATE2_PROXY,
  CREATE2_PROXY_DEPLOYER,
  CREATE2_PROXY_DEPLOY_COST,
  CREATE2_PROXY_DEPLOY_TX,
  ENTRY_POINT_V09_SALT,
} from "./config.js";
import { DevNode, waitForReceipt } from "./node.js";
import { loadArtifact } from "./artifacts.js";

async function hasCode(provider: JsonRpcProvider, addr: string): Promise<boolean> {
  const code = await provider.getCode(addr);
  return code !== undefined && code !== "0x";
}

// ensureCreate2ProxyDeployed installs the Arachnid deterministic-deployment
// proxy with its canonical presigned transaction: fund the one-time deployer
// address, then broadcast the keyless pre-EIP-155 tx (the node must run with
// --rpc.allow-unprotected-txs).
async function ensureCreate2ProxyDeployed(
  provider: JsonRpcProvider,
  dev: DevNode,
): Promise<void> {
  if (await hasCode(provider, CREATE2_PROXY)) return;

  await dev.ensureBalance(CREATE2_PROXY_DEPLOYER, CREATE2_PROXY_DEPLOY_COST);
  const hash: string = await provider.send("eth_sendRawTransaction", [
    CREATE2_PROXY_DEPLOY_TX,
  ]);
  const receipt = await waitForReceipt(provider, hash);
  if (receipt.status !== 1) {
    throw new Error(`CREATE2 proxy deploy tx reverted: ${hash}`);
  }
  // Query at the receipt's block: the receipt can become visible a moment
  // before the node's "latest" head pointer advances to the mined block.
  const code = await provider.getCode(CREATE2_PROXY, receipt.blockNumber);
  if (code === "0x") {
    throw new Error(`CREATE2 proxy deploy produced no code at ${CREATE2_PROXY}`);
  }
}

// ensureEntryPointDeployed makes sure a fully-constructed v0.9 EntryPoint
// exists on the target chain and returns its address.
export async function ensureEntryPointDeployed(
  provider: JsonRpcProvider,
): Promise<string> {
  if (await hasCode(provider, CANONICAL_ENTRY_POINT_V09)) {
    return CANONICAL_ENTRY_POINT_V09;
  }

  const art = loadArtifact("EntryPoint");
  const epAddress = getCreate2Address(
    CREATE2_PROXY,
    ENTRY_POINT_V09_SALT,
    keccak256(art.bytecode),
  );
  if (await hasCode(provider, epAddress)) {
    return epAddress;
  }

  const dev = new DevNode(provider);
  await ensureCreate2ProxyDeployed(provider, dev);

  // The construction itself is sent from the unlocked geth dev account; the
  // CREATE2 address depends only on the proxy, salt, and init code.
  const from = await dev.coinbase();
  const hash: string = await provider.send("eth_sendTransaction", [
    {
      from,
      to: CREATE2_PROXY,
      data: ENTRY_POINT_V09_SALT + art.bytecode.slice(2),
      gas: "0xa7d8c0", // 11M — geth dev genesis caps blocks at 11.5M
    },
  ]);
  const receipt = await waitForReceipt(provider, hash);
  if (receipt.status !== 1) {
    throw new Error(`EntryPoint CREATE2 deploy tx reverted: ${hash}`);
  }

  // Query at the receipt's block: the provider's cached "latest" tag can
  // briefly lag the just-mined block on auto-mining nodes.
  const code = await provider.getCode(epAddress, receipt.blockNumber);
  if (code === "0x") {
    throw new Error(
      `EntryPoint CREATE2 deploy produced no code at ${epAddress}`,
    );
  }
  return epAddress;
}

// CLI mode: node --loader ts-node/esm src/deploy-entrypoint.ts <node-url>
// Prints only the EntryPoint address on stdout (diagnostics go to stderr).
const invokedAsScript =
  process.argv[1] !== undefined &&
  import.meta.url.endsWith(process.argv[1].split("/").pop() ?? "");

if (invokedAsScript) {
  const [, , nodeUrl] = process.argv;
  if (!nodeUrl) {
    console.error("usage: deploy-entrypoint.ts <node-url>");
    process.exit(2);
  }
  const provider = new JsonRpcProvider(nodeUrl);
  ensureEntryPointDeployed(provider)
    .then((addr) => {
      console.error(`entrypoint ready at ${addr}`);
      console.log(addr);
      process.exit(0);
    })
    .catch((err) => {
      console.error(err);
      process.exit(1);
    });
}
