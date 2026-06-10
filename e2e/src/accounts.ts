import {
  Contract,
  ContractFactory,
  Interface,
  JsonRpcProvider,
  Wallet,
} from "ethers";

import { ENTRY_POINT_V09 } from "./config.js";
import { loadArtifact } from "./artifacts.js";

// SimpleAccountFactory bindings. Tests share a single deployed factory per
// chain (set up by scripts/e2e/run.sh) so multiple ops can create accounts
// off the same implementation.

const FACTORY_ABI = [
  "function createAccount(address owner, uint256 salt) returns (address)",
  "function getAddress(address owner, uint256 salt) view returns (address)",
];

const SIMPLE_ACCOUNT_ABI = [
  "function execute(address dest, uint256 value, bytes calldata func)",
];

export class FactoryClient {
  private readonly iface = new Interface(FACTORY_ABI);

  constructor(
    private readonly provider: JsonRpcProvider,
    private readonly factoryAddress: string,
  ) {}

  get address(): string {
    return this.factoryAddress;
  }

  // counterfactualAddress returns where a SimpleAccount for (owner, salt) will
  // be deployed. This is the address the userOp's `sender` field must reference
  // before the account exists on-chain.
  async counterfactualAddress(owner: string, salt: bigint): Promise<string> {
    const factory = new Contract(this.factoryAddress, FACTORY_ABI, this.provider);
    return (await factory.getFunction("getAddress")(owner, salt)) as string;
  }

  factoryData(owner: string, salt: bigint): string {
    return this.iface.encodeFunctionData("createAccount", [owner, salt]);
  }
}

// deployFactory builds and deploys a SimpleAccountFactory pointing at the
// installed v0.9 EntryPoint. Returns the deployed factory address.
export async function deployFactory(
  deployer: Wallet,
  entryPoint = ENTRY_POINT_V09,
): Promise<string> {
  const art = loadArtifact("SimpleAccountFactory");
  const factory = new ContractFactory(art.abi, art.bytecode, deployer);
  const c = await factory.deploy(entryPoint);
  await c.waitForDeployment();
  return await c.getAddress();
}

export function encodeExecute(target: string, value: bigint, data: string): string {
  const iface = new Interface(SIMPLE_ACCOUNT_ABI);
  return iface.encodeFunctionData("execute", [target, value, data]);
}

// fundAccount sends ETH to an arbitrary address using the supplied wallet. The
// SimpleAccount needs a prefund to pay the EntryPoint when no paymaster is
// attached.
export async function fundAccount(
  funder: Wallet,
  target: string,
  amountWei: bigint,
): Promise<void> {
  const tx = await funder.sendTransaction({ to: target, value: amountWei });
  await tx.wait();
}
