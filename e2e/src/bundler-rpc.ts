// Lightweight JSON-RPC client targeted at the bundler. We don't use ethers'
// JsonRpcProvider here because that machinery is wired for the eth_ namespace;
// we want to send arbitrary methods (eth_sendUserOperation, pm_sponsorUserOperation)
// and receive the raw result without any "verifyResult" interception.

export interface JsonRpcError {
  code: number;
  message: string;
  data?: unknown;
}

export class JsonRpcCallError extends Error {
  constructor(public readonly method: string, public readonly error: JsonRpcError) {
    super(`${method} failed (${error.code}): ${error.message}`);
  }
}

export class BundlerRpc {
  private id = 0;

  constructor(private readonly url: string) {}

  async call<T = unknown>(method: string, params: unknown[]): Promise<T> {
    this.id += 1;
    const res = await fetch(this.url, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: this.id, method, params }),
    });
    if (!res.ok) {
      throw new Error(`${method} HTTP ${res.status} ${res.statusText}`);
    }
    const body = (await res.json()) as { result?: T; error?: JsonRpcError };
    if (body.error) {
      throw new JsonRpcCallError(method, body.error);
    }
    return body.result as T;
  }

  // Bundler RPC surface.
  sendUserOperation(op: Record<string, unknown>, entryPoint: string): Promise<string> {
    return this.call<string>("eth_sendUserOperation", [op, entryPoint]);
  }

  estimateUserOperationGas(
    op: Record<string, unknown>,
    entryPoint: string,
    stateOverride?: Record<string, unknown>,
  ): Promise<Record<string, string>> {
    const params: unknown[] = [op, entryPoint];
    if (stateOverride) params.push(stateOverride);
    return this.call<Record<string, string>>("eth_estimateUserOperationGas", params);
  }

  getUserOperationReceipt(userOpHash: string): Promise<Record<string, unknown> | null> {
    return this.call<Record<string, unknown> | null>("eth_getUserOperationReceipt", [userOpHash]);
  }

  getUserOperationByHash(userOpHash: string): Promise<Record<string, unknown> | null> {
    return this.call<Record<string, unknown> | null>("eth_getUserOperationByHash", [userOpHash]);
  }

  supportedEntryPoints(): Promise<string[]> {
    return this.call<string[]>("eth_supportedEntryPoints", []);
  }

  chainId(): Promise<string> {
    return this.call<string>("eth_chainId", []);
  }

  sponsorUserOperation(
    op: Record<string, unknown>,
    entryPoint: string,
    ctx: Record<string, string>,
  ): Promise<Record<string, string>> {
    return this.call<Record<string, string>>(
      "pm_sponsorUserOperation",
      [op, entryPoint, ctx],
    );
  }
}

// pollReceipt blocks until either a non-null receipt is available or the
// timeout elapses. The bundler only writes a receipt after the op is included
// in a block, so callers typically pair this with an anvil_mine.
export async function pollReceipt(
  bundler: BundlerRpc,
  userOpHash: string,
  opts: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<Record<string, unknown>> {
  const timeoutMs = opts.timeoutMs ?? 60_000;
  const intervalMs = opts.intervalMs ?? 1_000;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const r = await bundler.getUserOperationReceipt(userOpHash);
    if (r) return r;
    await new Promise((res) => setTimeout(res, intervalMs));
  }
  throw new Error(`receipt for ${userOpHash} not produced within ${timeoutMs}ms`);
}
