import {
  AbiCoder,
  concat,
  keccak256,
  toBeHex,
  toUtf8Bytes,
  zeroPadValue,
} from "ethers";

// V09UserOperation mirrors the JSON-RPC shape the bundler expects in
// eth_sendUserOperation / eth_estimateUserOperationGas for ERC-4337 v0.9.
// Numeric fields are passed as bigint and serialized to 0x-hex when sending.
export interface V09UserOperation {
  sender: string;
  nonce: bigint;
  factory: string;
  factoryData: string;
  callData: string;
  callGasLimit: bigint;
  verificationGasLimit: bigint;
  preVerificationGas: bigint;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
  paymaster: string;
  paymasterVerificationGasLimit: bigint;
  paymasterPostOpGasLimit: bigint;
  paymasterData: string;
  signature: string;
  eip7702Auth?: V09Authorization;
  paymasterSignature?: string;
}

export interface V09Authorization {
  chainId: bigint;
  address: string;
  nonce: bigint;
  yParity: number;
  r: bigint;
  s: bigint;
}

const ZERO_ADDRESS = "0x0000000000000000000000000000000000000000";
const MAX_UINT128 = (1n << 128n) - 1n;

function packUint128Pair(high: bigint, low: bigint): string {
  const packed = ((high & MAX_UINT128) << 128n) | (low & MAX_UINT128);
  return zeroPadValue(toBeHex(packed), 32);
}

export function accountGasLimits(op: V09UserOperation): string {
  return packUint128Pair(op.verificationGasLimit, op.callGasLimit);
}

// gasFees packs maxPriorityFeePerGas (high 128 bits) and maxFeePerGas
// (low 128 bits) per UserOperationLib.unpackGasFees.
export function gasFees(op: V09UserOperation): string {
  return packUint128Pair(op.maxPriorityFeePerGas, op.maxFeePerGas);
}

export function packInitCode(op: V09UserOperation): string {
  if (op.factory.toLowerCase() === ZERO_ADDRESS) {
    return "0x";
  }
  return concat([op.factory, op.factoryData]);
}

export function packPaymasterAndData(op: V09UserOperation): string {
  if (op.paymaster.toLowerCase() === ZERO_ADDRESS) {
    return "0x";
  }
  return concat([
    op.paymaster,
    packUint128Pair(op.paymasterVerificationGasLimit, op.paymasterPostOpGasLimit),
    op.paymasterData,
  ]);
}

export function packForSignature(op: V09UserOperation): string {
  const initCode = packInitCode(op);
  const paymasterAndData = packPaymasterAndData(op);

  return AbiCoder.defaultAbiCoder().encode(
    [
      "address",
      "uint256",
      "bytes32",
      "bytes32",
      "bytes32",
      "uint256",
      "bytes32",
      "bytes32",
    ],
    [
      op.sender,
      op.nonce,
      keccak256(initCode),
      keccak256(op.callData),
      accountGasLimits(op),
      op.preVerificationGas,
      gasFees(op),
      keccak256(paymasterAndData),
    ],
  );
}

// PACKED_USEROP_TYPEHASH / EIP-712 domain constants mirroring
// UserOperationLib.sol — the v0.8+ EntryPoint hashes userOps as ERC-712
// typed data (domain name "ERC4337", version "1", verifyingContract = EP).
const PACKED_USEROP_TYPEHASH = keccak256(
  toUtf8Bytes(
    "PackedUserOperation(address sender,uint256 nonce,bytes initCode,bytes callData,bytes32 accountGasLimits,uint256 preVerificationGas,bytes32 gasFees,bytes paymasterAndData)",
  ),
);
const EIP712_DOMAIN_TYPEHASH = keccak256(
  toUtf8Bytes(
    "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
  ),
);
const DOMAIN_NAME_HASH = keccak256(toUtf8Bytes("ERC4337"));
const DOMAIN_VERSION_HASH = keccak256(toUtf8Bytes("1"));

export function domainSeparator(entryPoint: string, chainId: bigint): string {
  return keccak256(
    concat([
      EIP712_DOMAIN_TYPEHASH,
      DOMAIN_NAME_HASH,
      DOMAIN_VERSION_HASH,
      zeroPadValue(toBeHex(chainId), 32),
      zeroPadValue(entryPoint, 32),
    ]),
  );
}

// getUserOpHash mirrors v09.UserOperation.GetUserOpHash (the EntryPoint's
// ERC-712 typed-data hash). Note: paymasterSignature is intentionally
// excluded from the hash per v0.9 spec, so two ops differing only in
// paymasterSignature MUST hash to the same value.
export function getUserOpHash(
  op: V09UserOperation,
  entryPoint: string,
  chainId: bigint,
): string {
  const structHash = keccak256(
    concat([PACKED_USEROP_TYPEHASH, packForSignature(op)]),
  );
  return keccak256(
    concat(["0x1901", domainSeparator(entryPoint, chainId), structHash]),
  );
}

// toRpcJson converts a V09UserOperation into the JSON-RPC payload shape the
// bundler expects (all numeric fields as 0x-hex strings).
export function toRpcJson(op: V09UserOperation): Record<string, unknown> {
  const out: Record<string, unknown> = {
    sender: op.sender,
    nonce: toBeHex(op.nonce),
    factory: op.factory,
    factoryData: op.factoryData,
    callData: op.callData,
    callGasLimit: toBeHex(op.callGasLimit),
    verificationGasLimit: toBeHex(op.verificationGasLimit),
    preVerificationGas: toBeHex(op.preVerificationGas),
    maxFeePerGas: toBeHex(op.maxFeePerGas),
    maxPriorityFeePerGas: toBeHex(op.maxPriorityFeePerGas),
    paymaster: op.paymaster,
    paymasterVerificationGasLimit: toBeHex(op.paymasterVerificationGasLimit),
    paymasterPostOpGasLimit: toBeHex(op.paymasterPostOpGasLimit),
    paymasterData: op.paymasterData,
    signature: op.signature,
  };
  if (op.paymasterSignature) {
    out.paymasterSignature = op.paymasterSignature;
  }
  if (op.eip7702Auth) {
    out.eip7702Auth = {
      chainId: toBeHex(op.eip7702Auth.chainId),
      address: op.eip7702Auth.address,
      nonce: toBeHex(op.eip7702Auth.nonce),
      yParity: op.eip7702Auth.yParity,
      r: toBeHex(op.eip7702Auth.r),
      s: toBeHex(op.eip7702Auth.s),
    };
  }
  return out;
}

// applyEstimatedGas merges the bundler's gas estimate response back into a draft
// userOp so the caller can re-sign and re-send it.
export function applyEstimatedGas(
  op: V09UserOperation,
  estimate: Record<string, string>,
): V09UserOperation {
  const next: V09UserOperation = { ...op };
  if (estimate.preVerificationGas) {
    next.preVerificationGas = BigInt(estimate.preVerificationGas);
  }
  if (estimate.verificationGasLimit) {
    next.verificationGasLimit = BigInt(estimate.verificationGasLimit);
  }
  if (estimate.callGasLimit) {
    next.callGasLimit = BigInt(estimate.callGasLimit);
  }
  if (estimate.paymasterVerificationGasLimit) {
    next.paymasterVerificationGasLimit = BigInt(estimate.paymasterVerificationGasLimit);
  }
  if (estimate.paymasterPostOpGasLimit) {
    next.paymasterPostOpGasLimit = BigInt(estimate.paymasterPostOpGasLimit);
  }
  return next;
}

