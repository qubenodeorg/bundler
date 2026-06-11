import { JsonRpcProvider, TransactionReceipt, toQuantity } from "ethers";

// waitForReceipt polls eth_getTransactionReceipt directly. ethers'
// waitForTransaction relies on "block" events, whose polling subscriber only
// emits for blocks AFTER the first poll — on an on-demand chain like geth
// --dev, waiting on the most recent transaction would hang until some
// unrelated transaction produces the next block.
export async function waitForReceipt(
  provider: JsonRpcProvider,
  hash: string,
  timeoutMs = 30_000,
): Promise<TransactionReceipt> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const receipt = await provider.getTransactionReceipt(hash);
    if (receipt) return receipt;
    if (Date.now() > deadline) {
      throw new Error(`no receipt for ${hash} within ${timeoutMs}ms`);
    }
    await new Promise((r) => setTimeout(r, 100));
  }
}

// Helpers for the geth --dev node backing the e2e suite. Dev mode exposes a
// single pre-funded, unlocked "developer" account through eth_accounts; all
// test funding flows from it via plain eth_sendTransaction (no cheatcodes).

export class DevNode {
  private coinbasePromise?: Promise<string>;

  constructor(public readonly provider: JsonRpcProvider) {}

  // coinbase returns the geth dev-mode developer account address.
  async coinbase(): Promise<string> {
    if (!this.coinbasePromise) {
      this.coinbasePromise = this.provider
        .send("eth_accounts", [])
        .then((accounts: string[]) => {
          if (!accounts?.length) {
            throw new Error(
              "node exposes no unlocked accounts — is it running with geth --dev?",
            );
          }
          return accounts[0];
        });
    }
    return this.coinbasePromise;
  }

  // fund transfers amountWei from the dev account to the target address and
  // waits for the transfer to be mined.
  async fund(to: string, amountWei: bigint): Promise<void> {
    const from = await this.coinbase();
    const hash: string = await this.provider.send("eth_sendTransaction", [
      { from, to, value: toQuantity(amountWei) },
    ]);
    const receipt = await waitForReceipt(this.provider, hash);
    if (receipt.status !== 1) {
      throw new Error(`dev-account funding tx reverted: ${hash}`);
    }
  }

  // ensureBalance tops the target address up to at least amountWei.
  async ensureBalance(address: string, amountWei: bigint): Promise<void> {
    const current = await this.provider.getBalance(address);
    if (current >= amountWei) return;
    await this.fund(address, amountWei - current);
  }
}
