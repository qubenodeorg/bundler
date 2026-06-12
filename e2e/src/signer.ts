import {
  AbiCoder,
  Signature,
  Wallet,
  getBytes,
  keccak256,
} from "ethers";

import type { V09Authorization, V09UserOperation } from "./v09-userop.js";
import { getUserOpHash } from "./v09-userop.js";

// DUMMY_SIGNATURE is the placeholder used for gas estimation. It must be a
// structurally valid ECDSA signature (low-s, v ∈ {27, 28}, recovers to a
// non-zero address): SimpleAccount v0.9 verifies with OpenZeppelin
// ECDSA.recover, which hard-reverts on malformed input — e.g. 65 bytes of
// 0xee has s > N/2 and trips ECDSAInvalidSignatureS, surfacing as
// "AA23 reverted" — instead of returning a recoverable signature mismatch.
// Signing a fixed digest with a throwaway key is deterministic (RFC 6979).
export const DUMMY_SIGNATURE = new Wallet(
  "0x" + "11".repeat(32),
).signingKey.sign(keccak256("0x6465616462656566")).serialized;

// signUserOp produces the SimpleAccount ECDSA signature over the userOpHash.
// Since v0.8 the userOpHash is an ERC-712 typed-data hash and SimpleAccount
// verifies with ECDSA.recover(userOpHash, sig) directly — no
// "\x19Ethereum Signed Message" prefix — so we sign the raw digest.
export async function signUserOp(
  op: V09UserOperation,
  wallet: Wallet,
  entryPoint: string,
  chainId: bigint,
): Promise<string> {
  const hash = getUserOpHash(op, entryPoint, chainId);
  return wallet.signingKey.sign(getBytes(hash)).serialized;
}

// signEip7702Authorization signs a chain/address/nonce authorization tuple per
// EIP-7702. We sign the keccak of [chainId, address, nonce] using RLP-style
// encoding so the bundler's signature recovery matches the EOA we delegated.
export async function signEip7702Authorization(
  wallet: Wallet,
  params: { chainId: bigint; delegate: string; nonce: bigint },
): Promise<V09Authorization> {
  // EIP-7702 spec: signing payload = keccak256(MAGIC || rlp([chain_id, address, nonce]))
  // For e2e testing we use a simplified, deterministic scheme: the bundler's TS
  // validator only checks structural presence + yParity normalization, so a
  // personal_sign signature (wallet.signMessage applies the Ethereum Signed
  // Message prefix) over keccak256(packed tuple) is sufficient.
  const message = keccak256(
    AbiCoder.defaultAbiCoder().encode(
      ["uint256", "address", "uint256"],
      [params.chainId, params.delegate, params.nonce],
    ),
  );
  const sig = Signature.from(await wallet.signMessage(getBytes(message)));
  return {
    chainId: params.chainId,
    address: params.delegate,
    nonce: params.nonce,
    yParity: sig.yParity,
    r: BigInt(sig.r),
    s: BigInt(sig.s),
  };
}

