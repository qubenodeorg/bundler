import { beforeAll, describe, expect, it } from "vitest";

import { parseEther } from "ethers";

import { setupE2EContext, fundCounterfactual, type E2EContext } from "../src/fixture.js";
import { encodeExecute } from "../src/accounts.js";
import { ENTRY_POINT_V09 } from "../src/config.js";
import { signUserOp } from "../src/signer.js";
import {
  type V09UserOperation,
  applyEstimatedGas,
  toRpcJson,
} from "../src/v09-userop.js";
import { pollReceipt } from "../src/bundler-rpc.js";

const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000";
const ARBITRUM_CHAIN_ID = 42161n;

describe("arbitrum one fork — v0.9 bundler", () => {
  let ctx: E2EContext;

  beforeAll(async () => {
    ctx = await setupE2EContext();
    if (ctx.chainId !== ARBITRUM_CHAIN_ID) {
      throw new Error(
        `expected chainId=${ARBITRUM_CHAIN_ID} but got ${ctx.chainId}. ` +
          `Run scripts/e2e/run.sh arbitrum, not the ethereum variant.`,
      );
    }
  });

  async function buildOp(saltSeed: bigint): Promise<{
    op: V09UserOperation;
    sender: string;
    target: string;
    amount: bigint;
  }> {
    const sender = await ctx.factory.counterfactualAddress(ctx.owner.address, saltSeed);
    await fundCounterfactual(ctx, sender, parseEther("1"));

    const target = ctx.relayer.address;
    const amount = parseEther("0.001");
    const callData = encodeExecute(target, amount, "0x");

    const op: V09UserOperation = {
      sender,
      nonce: 0n,
      factory: ctx.factory.address,
      factoryData: ctx.factory.factoryData(ctx.owner.address, saltSeed),
      callData,
      callGasLimit: 200_000n,
      verificationGasLimit: 500_000n,
      // Arbitrum L1 calldata cost gets folded into PVG by the bundler with a
      // 16x buffer factor; start with a generous baseline and let estimation
      // tighten it.
      preVerificationGas: 1_000_000n,
      maxFeePerGas: 200_000_000n,
      maxPriorityFeePerGas: 0n,
      paymaster: ZERO_ADDRESS,
      paymasterVerificationGasLimit: 0n,
      paymasterPostOpGasLimit: 0n,
      paymasterData: "0x",
      signature: "0x",
    };

    return { op, sender, target, amount };
  }

  it("eth_chainId reports arbitrum one", async () => {
    const chainHex = await ctx.bundler.chainId();
    expect(BigInt(chainHex)).toBe(ARBITRUM_CHAIN_ID);
  });

  it("eth_estimateUserOperationGas applies Arbitrum L1-data PVG buffer", async () => {
    const { op } = await buildOp(0xa1n);
    op.signature = "0x" + "ee".repeat(65);

    const est = await ctx.bundler.estimateUserOperationGas(toRpcJson(op), ENTRY_POINT_V09);

    // On Arbitrum the PVG buffer factor is 16x — the resulting preVerificationGas
    // is dominated by L1 calldata cost and should be materially larger than the
    // ~50k baseline you'd see on Ethereum L1 mainnet for the same op.
    expect(BigInt(est.preVerificationGas)).toBeGreaterThan(50_000n);

    expect(est).toHaveProperty("paymasterVerificationGasLimit");
    expect(est).toHaveProperty("paymasterPostOpGasLimit");
  });

  it("eth_sendUserOperation completes on Arbitrum fork (deploy + transfer)", async () => {
    const { op, sender, target, amount } = await buildOp(0xa2n);
    op.signature = "0x" + "ee".repeat(65);
    const est = await ctx.bundler.estimateUserOperationGas(toRpcJson(op), ENTRY_POINT_V09);
    const finalOp = applyEstimatedGas(op, est);
    finalOp.signature = await signUserOp(finalOp, ctx.owner, ENTRY_POINT_V09, ctx.chainId);

    const targetBefore = await ctx.provider.getBalance(target);
    const hash = await ctx.bundler.sendUserOperation(toRpcJson(finalOp), ENTRY_POINT_V09);

    const receipt = await pollReceipt(ctx.bundler, hash, { timeoutMs: 90_000 });
    expect(receipt.success).toBe(true);
    expect((receipt.sender as string).toLowerCase()).toBe(sender.toLowerCase());

    const targetAfter = await ctx.provider.getBalance(target);
    expect(targetAfter - targetBefore).toBe(amount);

    const code = await ctx.provider.getCode(sender);
    expect(code.length).toBeGreaterThan(2);
  });

  it("relayer transaction lands on the Arbitrum fork (L2-specific receipt format)", async () => {
    const { op } = await buildOp(0xa3n);
    op.signature = "0x" + "ee".repeat(65);
    const est = await ctx.bundler.estimateUserOperationGas(toRpcJson(op), ENTRY_POINT_V09);
    const finalOp = applyEstimatedGas(op, est);
    finalOp.signature = await signUserOp(finalOp, ctx.owner, ENTRY_POINT_V09, ctx.chainId);

    const hash = await ctx.bundler.sendUserOperation(toRpcJson(finalOp), ENTRY_POINT_V09);
    const receipt = await pollReceipt(ctx.bundler, hash, { timeoutMs: 90_000 });

    expect(receipt).toHaveProperty("receipt");
    const txReceipt = receipt.receipt as Record<string, unknown>;

    // Arbitrum produces a real receipt — gasUsed must be a 0x-encoded uint
    // and effective gas price reflects the L2 baseline (~0.01 gwei minimum).
    expect(txReceipt.transactionHash).toMatch(/^0x[0-9a-f]{64}$/i);
    expect(BigInt(txReceipt.gasUsed as string)).toBeGreaterThan(0n);
    expect(BigInt(txReceipt.effectiveGasPrice as string)).toBeGreaterThan(0n);
  });
});
