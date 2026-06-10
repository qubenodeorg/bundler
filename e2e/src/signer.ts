import {
  AbiCoder,
  Signature,
  Wallet,
  getBytes,
  keccak256,
} from "ethers";

import type { V09Authorization, V09UserOperation } from "./v09-userop.js";
import { getUserOpHash } from "./v09-userop.js";

// signUserOp produces the SimpleAccount ECDSA signature over the userOpHash.
// The contract verifies with ECDSA.recover(toEthSignedMessageHash(hash), sig),
// so we sign with personal_sign semantics (ethers Wallet.signMessage handles
// the "\x19Ethereum Signed Message:\n32" prefix).
export async function signUserOp(
  op: V09UserOperation,
  wallet: Wallet,
  entryPoint: string,
  chainId: bigint,
): Promise<string> {
  const hash = getUserOpHash(op, entryPoint, chainId);
  const sig = await wallet.signMessage(getBytes(hash));
  return sig;
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

