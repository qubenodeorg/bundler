// Deploys the v0.9 EntryPoint for an e2e run and prints its address.
//
// The EntryPoint cannot be installed with anvil_setCode: its constructor
// deploys the SenderCreator and bakes its address into an immutable, so the
// artifact's deployedBytecode has that slot zeroed and createSender() would
// call address(0). Instead we construct it for real through the deterministic
// CREATE2 proxy (the same mechanism as the official deployment), which also
// places the SenderCreator at the create-nonce-1 address that
// EntryPointSimulations expects during bundler-side estimation.
//
// On a forked chain the canonical deployment already exists and is used
// as-is. Unforked, the npm artifact (currently 0.9.0-rc.1) CREATE2-deploys to
// its own deterministic address — computed, not hardcoded, so bumping the
// artifact to the final 0.9.0 bytecode makes it converge back to the
// canonical address automatically.
import { JsonRpcProvider, Wallet, getCreate2Address, keccak256 } from "ethers";

import {
  ANVIL_ACCOUNT_9,
  CANONICAL_ENTRY_POINT_V09,
  CREATE2_PROXY,
  CREATE2_PROXY_RUNTIME,
  ENTRY_POINT_V09_SALT,
} from "./config.js";
import { loadArtifact } from "./artifacts.js";

async function hasCode(provider: JsonRpcProvider, addr: string): Promise<boolean> {
  const code = await provider.getCode(addr);
  return code !== undefined && code !== "0x";
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

  if (!(await hasCode(provider, CREATE2_PROXY))) {
    await provider.send("anvil_setCode", [CREATE2_PROXY, CREATE2_PROXY_RUNTIME]);
  }

  const deployer = new Wallet(ANVIL_ACCOUNT_9.privateKey, provider);
  const tx = await deployer.sendTransaction({
    to: CREATE2_PROXY,
    data: ENTRY_POINT_V09_SALT + art.bytecode.slice(2),
    gasLimit: 12_000_000n,
  });
  const receipt = await tx.wait();
  if (receipt?.status !== 1) {
    throw new Error(`EntryPoint CREATE2 deploy tx reverted: ${tx.hash}`);
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

// CLI mode: node --loader ts-node/esm src/deploy-entrypoint.ts <anvil-url>
// Prints only the EntryPoint address on stdout (diagnostics go to stderr).
const invokedAsScript =
  process.argv[1] !== undefined &&
  import.meta.url.endsWith(process.argv[1].split("/").pop() ?? "");

if (invokedAsScript) {
  const [, , anvilUrl] = process.argv;
  if (!anvilUrl) {
    console.error("usage: deploy-entrypoint.ts <anvil-url>");
    process.exit(2);
  }
  const provider = new JsonRpcProvider(anvilUrl);
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
