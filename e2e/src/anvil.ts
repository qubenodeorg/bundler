import { JsonRpcProvider, toBeHex } from "ethers";

// Anvil cheat-code RPC helpers. Each thin wrapper sends a single JSON-RPC call
// to the local anvil node so tests can prep state (set bytecode, mint balance,
// mine a block, advance time, snapshot/revert).

export class AnvilCheats {
  constructor(private readonly provider: JsonRpcProvider) {}

  async setCode(address: string, code: string): Promise<void> {
    await this.provider.send("anvil_setCode", [address, code]);
  }

  async setBalance(address: string, balanceWei: bigint): Promise<void> {
    await this.provider.send("anvil_setBalance", [address, toBeHex(balanceWei)]);
  }

  async setNextBlockBaseFeePerGas(baseFeeWei: bigint): Promise<void> {
    await this.provider.send("anvil_setNextBlockBaseFeePerGas", [toBeHex(baseFeeWei)]);
  }

  async mine(blocks = 1): Promise<void> {
    await this.provider.send("anvil_mine", [toBeHex(blocks)]);
  }

  async snapshot(): Promise<string> {
    return await this.provider.send("evm_snapshot", []);
  }

  async revert(id: string): Promise<boolean> {
    return await this.provider.send("evm_revert", [id]);
  }

  async impersonate(address: string): Promise<void> {
    await this.provider.send("anvil_impersonateAccount", [address]);
  }

  async stopImpersonating(address: string): Promise<void> {
    await this.provider.send("anvil_stopImpersonatingAccount", [address]);
  }
}
