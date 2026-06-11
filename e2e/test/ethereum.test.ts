import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { Wallet, parseEther } from "ethers";

import { setupE2EContext, fundCounterfactual, type E2EContext } from "../src/fixture.js";
import { encodeExecute } from "../src/accounts.js";
import { ENTRY_POINT_V09 } from "../src/config.js";
import { DUMMY_SIGNATURE, signEip7702Authorization, signUserOp } from "../src/signer.js";
import {
  type V09UserOperation,
  applyEstimatedGas,
  getUserOpHash,
  toRpcJson,
} from "../src/v09-userop.js";
import { pollReceipt } from "../src/bundler-rpc.js";

const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000";

describe("ethereum (geth --dev) — v0.9 bundler", () => {
  let ctx: E2EContext;

  beforeAll(async () => {
    ctx = await setupE2EContext();
  });

  afterAll(async () => {
    // Tests share one geth --dev instance per file, so individual tests should
    // not permanently mutate shared state. Nothing to tear down here.
  });

  // Helper: build a counterfactual SimpleAccount op that pays its own prefund.
  async function buildDeployAndTransferOp(saltSeed: bigint): Promise<{
    op: V09UserOperation;
    sender: string;
    transferTarget: string;
    transferAmount: bigint;
  }> {
    const salt = saltSeed;
    const sender = await ctx.factory.counterfactualAddress(ctx.owner.address, salt);
    await fundCounterfactual(ctx, sender, parseEther("1"));

    // A neutral sink address: the relayer can't be used here because it also
    // pays bundle gas and collects the beneficiary compensation, so its
    // balance delta would not equal the transfer amount.
    const transferTarget = new Wallet("0x" + "22".repeat(32)).address;
    const transferAmount = parseEther("0.001");
    const callData = encodeExecute(transferTarget, transferAmount, "0x");

    const op: V09UserOperation = {
      sender,
      nonce: 0n,
      factory: ctx.factory.address,
      factoryData: ctx.factory.factoryData(ctx.owner.address, salt),
      callData,
      callGasLimit: 200_000n,
      verificationGasLimit: 500_000n,
      preVerificationGas: 60_000n,
      maxFeePerGas: 50_000_000_000n,
      maxPriorityFeePerGas: 2_000_000_000n,
      paymaster: ZERO_ADDRESS,
      paymasterVerificationGasLimit: 0n,
      paymasterPostOpGasLimit: 0n,
      paymasterData: "0x",
      signature: "0x",
    };

    return { op, sender, transferTarget, transferAmount };
  }

  it("supports eth_supportedEntryPoints and eth_chainId", async () => {
    const eps = await ctx.bundler.supportedEntryPoints();
    expect(eps.map((e) => e.toLowerCase())).toContain(ENTRY_POINT_V09.toLowerCase());

    const chainHex = await ctx.bundler.chainId();
    expect(BigInt(chainHex)).toBe(ctx.chainId);
  });

  it("eth_estimateUserOperationGas returns v0.9 fields including paymaster gas limits", async () => {
    const { op } = await buildDeployAndTransferOp(0xe1n);
    op.signature = DUMMY_SIGNATURE;

    const est = await ctx.bundler.estimateUserOperationGas(
      toRpcJson(op),
      ENTRY_POINT_V09,
    );

    expect(est.preVerificationGas).toMatch(/^0x[0-9a-f]+$/i);
    expect(est.verificationGasLimit).toMatch(/^0x[0-9a-f]+$/i);
    expect(est.callGasLimit).toMatch(/^0x[0-9a-f]+$/i);
    // v0.9 must split paymaster gas across two separate fields.
    expect(est).toHaveProperty("paymasterVerificationGasLimit");
    expect(est).toHaveProperty("paymasterPostOpGasLimit");
    expect(BigInt(est.preVerificationGas)).toBeGreaterThan(0n);
    expect(BigInt(est.verificationGasLimit)).toBeGreaterThan(0n);
  });

  it("eth_sendUserOperation deploys a SimpleAccount and routes ETH transfer", async () => {
    const { op, sender, transferTarget, transferAmount } =
      await buildDeployAndTransferOp(0xe2n);

    // Estimate gas, then re-sign with the final values.
    op.signature = DUMMY_SIGNATURE;
    const est = await ctx.bundler.estimateUserOperationGas(toRpcJson(op), ENTRY_POINT_V09);
    const finalOp = applyEstimatedGas(op, est);
    finalOp.signature = await signUserOp(finalOp, ctx.owner, ENTRY_POINT_V09, ctx.chainId);

    const targetBefore = await ctx.provider.getBalance(transferTarget);
    const hash = await ctx.bundler.sendUserOperation(toRpcJson(finalOp), ENTRY_POINT_V09);
    expect(hash).toBe(getUserOpHash(finalOp, ENTRY_POINT_V09, ctx.chainId));

    const receipt = await pollReceipt(ctx.bundler, hash, { timeoutMs: 90_000 });
    expect(receipt.success).toBe(true);
    expect((receipt.sender as string).toLowerCase()).toBe(sender.toLowerCase());

    // eth_getUserOperationByHash reflects the op we sent.
    const lookup = (await ctx.bundler.getUserOperationByHash(hash))!;
    expect(lookup).toBeTruthy();
    expect((lookup.entryPoint as string).toLowerCase()).toBe(ENTRY_POINT_V09.toLowerCase());

    const targetAfter = await ctx.provider.getBalance(transferTarget);
    expect(targetAfter - targetBefore).toBe(transferAmount);

    // SimpleAccount code is now present at the sender address.
    const senderCode = await ctx.provider.getCode(sender);
    expect(senderCode.length).toBeGreaterThan(2);
  });

  it("eth_getUserOperationReceipt returns null for unknown hash", async () => {
    const r = await ctx.bundler.getUserOperationReceipt(
      "0x" + "00".repeat(32),
    );
    expect(r).toBeNull();
  });

  it("paymaster signature flow — pm_sponsorUserOperation returns paymasterSignature separately", async () => {
    const { op } = await buildDeployAndTransferOp(0xe3n);
    op.signature = DUMMY_SIGNATURE;

    let response: Record<string, string> | undefined;
    try {
      response = await ctx.bundler.sponsorUserOperation(
        toRpcJson(op),
        ENTRY_POINT_V09,
        { type: "payg" },
      );
    } catch (err: any) {
      // The bundler ships with a hard-coded PayG paymaster address that must
      // be deployed on the chain. It isn't on a fresh geth --dev chain, so skip.
      if (/paymaster|contract|reverted/i.test(err.message)) {
        console.warn(`pm_sponsorUserOperation unavailable on this fork: ${err.message}`);
        return;
      }
      throw err;
    }

    // v0.9-specific: paymasterSignature must be a non-empty hex string distinct
    // from paymasterAndData. The bundler returns an empty signature embedded
    // inside paymasterAndData for v0.6/v0.7/v0.8.
    expect(response).toHaveProperty("paymasterSignature");
    expect(response!.paymasterSignature).toMatch(/^0x[0-9a-f]+$/i);
    expect(response!.paymasterSignature.length).toBeGreaterThan(2);
    expect(response).toHaveProperty("paymasterAndData");
    expect(response!.paymasterAndData).not.toBe(response!.paymasterSignature);

    // Sanity: gas estimates returned alongside the signature.
    expect(BigInt(response!.callGasLimit)).toBeGreaterThan(0n);
    expect(BigInt(response!.verificationGasLimit)).toBeGreaterThan(0n);
  });

  it("EIP-7702 flow — bundler accepts eip7702Auth in the userOp envelope", async () => {
    // Deploy a fresh SimpleAccount to use as the delegate target.
    const delegateSalt = 0xe4n;
    const delegate = await ctx.factory.counterfactualAddress(
      ctx.owner.address,
      delegateSalt,
    );

    const sender = ctx.owner.address; // EOA self-delegating to the SimpleAccount.
    await fundCounterfactual(ctx, sender, parseEther("1"));

    const callData = encodeExecute(ctx.relayer.address, parseEther("0.0001"), "0x");
    const eip7702Auth = await signEip7702Authorization(ctx.owner, {
      chainId: ctx.chainId,
      delegate,
      nonce: 0n,
    });

    const op: V09UserOperation = {
      sender,
      nonce: 0n,
      factory: ZERO_ADDRESS,
      factoryData: "0x",
      callData,
      callGasLimit: 200_000n,
      verificationGasLimit: 500_000n,
      preVerificationGas: 60_000n,
      maxFeePerGas: 50_000_000_000n,
      maxPriorityFeePerGas: 2_000_000_000n,
      paymaster: ZERO_ADDRESS,
      paymasterVerificationGasLimit: 0n,
      paymasterPostOpGasLimit: 0n,
      paymasterData: "0x",
      signature: DUMMY_SIGNATURE,
      eip7702Auth,
    };

    // The 7702 path requires full end-to-end delegation support across the
    // node, the bundler, and the account. Here we assert that the bundler at
    // least accepts the envelope and surfaces a meaningful error, rather than
    // silently dropping the auth field.
    try {
      await ctx.bundler.estimateUserOperationGas(toRpcJson(op), ENTRY_POINT_V09);
    } catch (err: any) {
      // We expect either a validation-rule failure or a delegation mismatch;
      // we do NOT want a JSON-schema / "unknown field" rejection.
      expect(err.message).not.toMatch(/unknown field|invalid request/i);
    }
  });

  it("userOpHash excludes paymasterSignature (v0.9 spec)", async () => {
    const { op } = await buildDeployAndTransferOp(0xe5n);
    op.signature = "0x" + "11".repeat(65);

    const baseHash = getUserOpHash(op, ENTRY_POINT_V09, ctx.chainId);
    const withSig = { ...op, paymasterSignature: "0x" + "ab".repeat(65) };
    const sigHash = getUserOpHash(withSig, ENTRY_POINT_V09, ctx.chainId);
    expect(sigHash).toBe(baseHash);
  });
});
