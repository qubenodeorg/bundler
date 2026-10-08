# Cloud-Native Production Roadmap

Checklist for turning the single-instance, embedded-storage bundler into a
horizontally scalable, cloud-native service.

**Scope of this roadmap**

1. **Pluggable storage** — back the mempool, reputation, and code-hash caches with
   Redis/Valkey (and allow other backends) instead of the node-local BadgerDB.
2. **Pluggable signers** — sign bundle transactions through a provider-agnostic
   interface (AWS KMS, Google Cloud KMS, Azure Key Vault, Turnkey, Privy, HashiCorp
   Vault, PKCS#11/HSM, Web3Signer, raw key) rather than a raw
   `ERC4337_BUNDLER_PRIVATE_KEY`.
3. **Configurable node access** — full vs. archive node, configurable **per chain**,
   with automatic detection of the connected node's capabilities/type.

Each section lists current state, target state, concrete tasks, and acceptance
criteria.

---

## 0. Current State / Gap Analysis

| Area | Today | Cloud-native gap |
|------|-------|------------------|
| Mempool | BadgerDB opened per-process (`internal/start/private.go:58`), in-memory sorted queues (`pkg/mempool/queue.go`) | Node-local, single-writer; two replicas cannot share the pool or coordinate bundling |
| Reputation | BadgerDB in `pkg/modules/entities/reputation.go` | Same; per-replica counters diverge |
| Code-hash cache | BadgerDB in `pkg/modules/checks/db.go` | Same; safe to share or make per-replica |
| DB lifecycle | `badger.Open` + value-log GC goroutine (`internal/start/db.go`) | No connection pooling, TLS, or health/backoff semantics |
| Readiness | `db.View(...)` no-op probe assumes `*badger.DB` (`internal/start/health.go:65`) | Tied to a concrete type |
| Signer | `pkg/signer/signer.go` holds a raw `*ecdsa.PrivateKey`; signed in-process via `bind.NewKeyedTransactorWithChainID` and `crypto.Sign` | No remote/KMS/HSM path; key material in env |
| Node | One `ERC4337_BUNDLER_ETH_CLIENT_URL`, one `rpc.Client` (`internal/config/values.go:20`) | No full/archive distinction, no per-chain config, no failover, no capability detection |
| Chain | One chain per process; rollup flags (`IsOpStackNetwork`, `IsArbStackNetwork`) | No multi-chain routing |

**Non-negotiable invariants to preserve**

- [ ] No two bundler replicas may build/submit overlapping bundles for the same
      `(entryPoint, sender, nonce)`.
- [ ] The mempool must survive a rolling deploy (no lost pending `UserOperation`s).
- [ ] Signer address used at startup must be verified and exposed as the beneficiary.
- [ ] A missing/failed dependency must fail readiness, not liveness.

---

## 1. Pluggable Storage

### 1.1 Define a storage interface

- [ ] Introduce `pkg/store` (or `internal/store`) with narrow interfaces per concern:
  - [ ] `MempoolStore` — `Add(ep, op)`, `Remove(ep, ops...)`, `Ops(ep, sender)`,
        `All(ep)`, `Clear()`, plus a way to stream by entity/nonce ordering.
  - [ ] `ReputationStore` — `GetStatus`, `OpsCount`, `Increment*`, `Override`, `Dump`, `Clear`.
  - [ ] `CodeHashStore` — `Save`, `Get`, `Remove`.
  - [ ] `Closer` / `Pinger` — `Close()`, `Ping(ctx)`.
- [ ] Return typed errors so callers/health can distinguish "not found" from
      "backend unavailable" (mapping from `badger.ErrKeyNotFound`).
- [ ] Keep interfaces context-aware (`ctx context.Context`) to support network
      backends and cancellation.
- [ ] Add a `Factory`/`Open(cfg)` selector: `badger`, `redis`, `valkey`, `memory`.

### 1.2 BadgerDB backend (extraction, not removal)

- [ ] Move existing Badger logic behind the new interfaces
      (`pkg/mempool/instance.go`, `pkg/mempool/db.go`,
      `pkg/modules/entities/reputation.go`, `pkg/modules/checks/db.go`).
- [ ] Keep `badger` as the default for local dev / single-node deployments.
- [ ] Remove the `*badger.DB` types from `internal/start/*`, `internal/start/health.go`,
      and `internal/start/db.go`; inject the interface instead.

### 1.3 Redis/Valkey backend

- [ ] Add a client dependency (`github.com/redis/go-redis/v9`; Valkey is wire-compatible).
- [ ] Key layout (namespaced by deployment + chain ID):
  - [ ] Mempool op: `mempool:{ep}:{sender}:{nonce}` → serialized `UserOperation` JSON.
  - [ ] Arrival ordering: `ZSET mempool:order:{ep}` scored by arrival time/sequence.
  - [ ] Per-sender index: `ZSET mempool:sender:{ep}:{sender}` scored by nonce.
  - [ ] Per-factory / per-paymaster indexes: `ZSET mempool:entity:{ep}:{addr}`.
  - [ ] Reputation counters: `HASH reputation:{ep}:{addr}` (opsSeen, opsIncluded).
  - [ ] Code hashes: `mempool:codeHashes:{userOpHash}`.
- [ ] Make writes atomic per op using `MULTI`/Lua so an op and all its indexes are
      updated together (replacement of a same-`(sender,nonce)` op must not leak
      stale index entries). Mirror `RemoveOps` cleanup at `pkg/mempool/queue.go:80`.
- [ ] Apply a TTL to pending ops (`ERC4337_BUNDLER_MAX_OP_TTL_SECONDS`) so a crashed
      replica cannot leave op state forever; reconcile TTL with `expire.DropExpired`.
- [ ] Use a distributed lock / compare-and-set for op replacement so concurrent
      replicas cannot both accept conflicting replacements for the same nonce.
- [ ] Support connection options:
  - [ ] `ERC4337_BUNDLER_STORE_URL` (redis:// / rediss:// / valkey://).
  - [ ] Sentinel and cluster modes.
  - [ ] TLS (CA cert, client cert/key), auth (username/password), db index.
  - [ ] Pool size, timeouts, dial/read/write retries.
- [ ] Implement `Ping` with a short timeout for readiness.
- [ ] Encrypt-in-transit (rediss) documented and default in production examples.

### 1.4 Coordination for horizontal scaling

- [ ] Add a distributed lock manager (Redis lock or Kubernetes Lease):
  - [ ] Per-`(ep, sender)` bundle lock so only one replica bundles a sender at a time.
  - [ ] Optional single-active-bundler leader election (simplest correct model);
        followers serve read-only RPCs (`eth_getUserOperationReceipt`, etc.).
- [ ] Make op claims explicit: when the bundler selects a batch, mark ops as
      `in-flight` (with lease expiry) so a crashed replica's ops are reclaimed.
- [ ] Define reclaim/expiry semantics and test replica-kill-mid-bundle.

### 1.5 Multi-chain namespacing

- [ ] Prefix every key with chain ID so one Redis/Valkey cluster can serve many
      chains (`ERC4337_BUNDLER_CHAIN_ID` is implicit today; make it explicit).
- [ ] Ensure a single process can serve multiple chains, or document the
      one-process-per-chain topology.

### 1.6 Acceptance criteria

- [ ] Running tests: kill replica A mid-bundle; replica B reclaims ops after the
      lease expiry and no op is bundled twice.
- [ ] Rolling deploy loses zero pending `UserOperation`s.
- [ ] `redis-cli` shows bounded key growth (TTL enforced) under sustained load.
- [ ] Readiness reports `store: unavailable` / `503` when Redis is unreachable.
- [ ] Badger and Redis backends pass the same store test suite.

---

## 2. Pluggable Signers

### 2.1 Define a signer interface

- [ ] Replace the concrete `pkg/signer.EOA` field usage with an interface, e.g.
      `pkg/signer.Signer`:

      ```go
      type Signer interface {
          Address(ctx context.Context) (common.Address, error)
          // SignHash signs a 32-byte digest; returns 65-byte [R||S||V].
          SignHash(ctx context.Context, digest []byte) ([]byte, error)
          // SignTx signs an unsigned tx for the given chain and returns it signed.
          SignTx(ctx context.Context, chainID *big.Int, tx *types.Transaction) (*types.Transaction, error)
      }
      ```

- [ ] Keep `EOA` as the `raw` implementation (current behavior) for local dev.
- [ ] Add a `signer.New(cfg)` factory selecting by `ERC4337_BUNDLER_SIGNER_TYPE`.

### 2.2 Implementations

- [ ] `raw` — hex key from env (existing behavior; keep for dev only, warn in prod).
- [ ] `aws-kms` — AWS KMS asymmetric secp256k1 key; `Sign` on the digest,
      key ID via config; verify the returned public key matches the expected address.
- [ ] `gcp-kms` — Cloud KMS `AsymmetricSign` (secp256k1).
- [ ] `azure-keyvault` — Key Vault `sign` / managed HSM.
- [ ] `turnkey` — Turnkey API signer (organization + private key ID).
- [ ] `privy` — Privy wallet API signer.
- [ ] `vault` — HashiCorp Vault Transit `sign` (secp256k1).
- [ ] `pkcs11` — PKCS#11 / HSM (SoftHSM for tests, cloud HSM in prod).
- [ ] `web3signer` — remote signing via Web3Signer REST.
- [ ] Document that providers must support **low-S normalized, recovery-id** secp256k1
      signatures (or normalize in the adapter).

### 2.3 Wiring changes

- [ ] Replace `bind.NewKeyedTransactorWithChainID(opts.EOA.PrivateKey, chainID)`:
      - [ ] `pkg/entrypoint/transaction/handleops.go:63,104`
      - [ ] `pkg/entrypoint/v07/transaction/handleops.go:89,136`
      - [ ] `pkg/entrypoint/v08/transaction/handleops.go:91,143`
      - [ ] `pkg/entrypoint/v09/transaction/handleops.go:91,142`
      Use `bind.TransactOpts{ Signer: func(addr, tx) }` backed by `Signer.SignTx`,
      or build/sign txs directly and broadcast through the RPC client.
- [ ] Replace direct `crypto.Sign(..., opts.EOA.PrivateKey)` in EIP-7702 paths:
      - [ ] `pkg/entrypoint/v08/transaction/eip7702.go:149`
      - [ ] `pkg/entrypoint/v09/transaction/eip7702.go:149`
      with `Signer.SignHash`.
- [ ] Replace `*signer.EOA` fields in:
      - [ ] `pkg/modules/relay/relayer.go:35`
      - [ ] `pkg/modules/builder` (searcher mode)
      - [ ] `pkg/client/client.go`, `pkg/client/debug.go`
      - [ ] `internal/start/private.go:51`, `internal/start/searcher.go:49`
- [ ] Replace `internal/config/values.go:203` beneficiary derivation with
      `Signer.Address(ctx)` (startup only, cached).
- [ ] Add signer startup self-test: `Address()` plus a sign/verify round-trip before
      serving traffic; abort startup on mismatch.

### 2.4 Operational concerns

- [ ] Config per chain (multi-chain): signer key/ID may differ per chain ID.
- [ ] Timeout + retry/backoff on remote signing calls (they are network-bound and
      on the bundle critical path).
- [ ] Optional in-process signing result cache is **not** possible (nonces/signing
      are not idempotent); instead ensure single-flight around each tx signing.
- [ ] Metrics: signing latency, error rate, provider throttling.
- [ ] Secrets come from the secret manager/KMS, never plaintext env, in production.
- [ ] IAM guidance per provider (least-privilege `kms:Sign` / equivalent).
- [ ] Fail readiness (not liveness) if the signer is unreachable.

### 2.5 Acceptance criteria

- [ ] Same behavior across raw, KMS (AWS/GCP/Azure), and SaaS (Turnkey/Privy) adapters
      in an integration test matrix.
- [ ] Signed tx recovers to the configured address on Ethereum and an L2.
- [ ] EIP-7702 (v0.8/v0.9) bundle signing works through a remote signer.
- [ ] Startup fails clearly when the configured signer cannot produce the expected address.
- [ ] No signer raw private key is ever logged or serialized.

---

## 3. Node Connectivity: Full vs. Archive, Per Chain, Auto-Detected

### 3.1 Configuration model

- [ ] Replace the single `ERC4337_BUNDLER_ETH_CLIENT_URL` with a chain-keyed config
      structure. Example shape:

      ```yaml
      chains:
        1:
          node_url: https://eth-mainnet.example
          node_failover_urls: [https://eth-mainnet-backup.example]
          node_type: auto            # full | archive | auto
          archive_url: https://eth-archive.example   # used when available/historical access needed
          ws_url: wss://eth-mainnet.example
          is_op_stack: false
          is_arb_stack: false
        42161:
          node_url: https://arb.example
          node_type: full
      ```

- [ ] Env var equivalents for single-chain deployments, preserving backward
      compatibility:
      - [ ] `ERC4337_BUNDLER_ETH_CLIENT_URL` (existing).
      - [ ] `ERC4337_BUNDLER_ETH_ARCHIVE_URL` (optional separate archive endpoint).
      - [ ] `ERC4337_BUNDLER_NODE_TYPE=auto|full|archive` (default `auto`).
      - [ ] `ERC4337_BUNDLER_ETH_CLIENT_FALLBACK_URLS` (comma-separated failover).
      - [ ] Per-chain prefix, e.g. `ERC4337_BUNDLER_CHAIN_42161_ETH_CLIENT_URL`,
            or a `ERC4337_BUNDLER_CHAINS_FILE` path to the YAML above.
- [ ] Validate config: reject `node_type: archive` when no archive-capable endpoint
      is configured, unless `auto` is selected.

### 3.2 Node capability / type detection

- [ ] Implement a `DetectNodeCapabilities(ctx, rpc)` probe run at startup and
      periodically, producing a capability set:
  - [ ] `Eth client`: `web3_clientVersion` / `eth_chainId` (sanity).
  - [ ] Sync/completeness: `eth_syncing` (must be `false`), `net_peerCount` > 0.
  - [ ] **Archive detection** — query historical state beyond the prune window:
        - [ ] `eth_getBalance(addr, block = current-1_000_000)` (or chain-specific depth);
              a `missing trie node` / null error ⇒ pruned/full node.
        - [ ] `debug_getRawBlock` / `eth_getBlockByNumber` at a deep old height.
        - [ ] `eth_getProof` at a deep old block (also tests proof support).
  - [ ] `debug_traceCall` support (required by collector/executor tracers —
        `pkg/entrypoint/*/simulation`, `pkg/entrypoint/*/execution`).
  - [ ] State-override support in `eth_call` / `debug_traceCall` (`pkg/state`).
  - [ ] `debug_traceTransaction` support (receipt/history paths).
  - [ ] `eth_getLogs` max block range / result cap (used by
        `GetUserOpReceiptWithEthClient` and `GetUserOpByHashWithEthClient`).
  - [ ] `eth_feeHistory` / EIP-1559 support; `eth_maxPriorityFeePerGas`.
  - [ ] Custom native tracer support (muxTracer/callTracer/erc7562Tracer) vs. only
        built-ins.
- [ ] Resolve `auto` → effective type: `archive` if historical probes succeed,
      else `full`; log the detected type and capabilities at startup.
- [ ] Expose detection results (node type, capabilities) on a `/nodeinfo` debug
      endpoint and in readiness/logs.

### 3.3 Feature gating by capability

- [ ] Gate features on capabilities rather than assuming `debug` is available:
  - [ ] `eth_getUserOperationByHash` / receipt scans need historical logs / archive
        depth beyond the lookup window (`OpLookupLimit`).
  - [ ] Simulation/tracing requires `debug_traceCall` + state overrides.
  - [ ] If archive is missing and a historical lookup is requested, return a clear
        JSON-RPC error instead of a generic failure.
- [ ] Decide and document which paths strictly require archive:
  - [ ] Recent receipt lookups (within block range) usually work on a full node.
  - [ ] Deep historical lookups require archive.
  - [ ] `debug_traceCall` at `latest` works on a full node; tracing historical
        blocks requires archive.
- [ ] Add a startup validation that fails fast when the configured `node_type`
      promises more than the endpoint delivers (e.g. `archive` configured but probes
      show pruned state) — or downgrade with a loud warning in `auto`.

### 3.4 Connections and resilience

- [ ] Support HTTP and WebSocket; use WS for new-head subscriptions where available.
- [ ] Add failover across multiple endpoints with health tracking (round-robin or
      primary/secondary), and separate archive endpoint for historical calls.
- [ ] Configure per-call timeouts, retries, and a rate limiter to protect upstreams.
- [ ] Distinguish transient vs. terminal RPC errors; retry only transient ones.
- [ ] Circuit-break a failing endpoint and route to fallback.
- [ ] Readiness should reflect upstream node health (already probes
      `eth.BlockNumber` at `internal/start/health.go:55`) plus archive availability
      when archive features are enabled.
- [ ] Add metrics: per-endpoint latency, error rate, failover count, detected type.

### 3.5 Acceptance criteria

- [ ] `node_type: auto` correctly labels a pruned full node as `full` and an
      archive node as `archive` in a live test.
- [ ] Historical lookup on a full node returns a clear, documented error; the same
      lookup on an archive endpoint succeeds.
- [ ] Configuring `archive` against a pruned node fails startup (or warns in `auto`).
- [ ] Primary/fallback failover keeps the bundler serving during an endpoint outage.
- [ ] Per-chain config drives the correct endpoint and rollup flags for ≥2 chains.

---

## 4. Statelessness & Horizontal Scaling

- [ ] Remove all node-local mutable state required for correctness:
  - [ ] Mempool (section 1).
  - [ ] Reputation (section 1).
  - [ ] Any in-process sequence/arrival counters used for ordering (`pkg/mempool/queue.go`
        uses `sortedset` scores computed from `GetCount()`/`Nonce` — move ordering
        into the store).
- [ ] Make replicas interchangeable: no sticky sessions required for correctness
      beyond in-flight operations.
- [ ] Add readiness/liveness/startup probes compatible with Kubernetes
      (`/live`, `/ready` exist; add `/startup` if slow init).
- [ ] Expose graceful shutdown that stops the batch loop before closing the store
      (pattern already in `internal/start/private.go:219`); ensure it also releases
      store leases/locks.
- [ ] Define pod disruption budget / rolling update behavior in deployment docs.

---

## 5. Configuration & Secrets

- [ ] Adopt a single, documented config schema for storage, signer, and node
      (env vars for simple setups; file/YAML for multi-chain).
- [ ] Validate all required config at startup with actionable error messages.
- [ ] Support secret injection via files/secrets manager (not just env):
      DB password, TLS material, signer credentials.
- [ ] Backward compatibility: existing `ERC4337_BUNDLER_ETH_CLIENT_URL` +
      `ERC4337_BUNDLER_PRIVATE_KEY` must keep working (mapping to `raw` signer and
      `auto` node type) for one release, with a deprecation warning.
- [ ] Update `.env.example`, `docker-compose.yml`, and `docs/CONFIGURATION.md`.

---

## 6. Observability

- [ ] Extend OpenTelemetry metrics for the new boundaries:
  - [ ] Store latency/errors, lock contention, lease reclaims.
  - [ ] Signer latency/errors, provider throttling.
  - [ ] Node per-endpoint latency/errors, failover, detected type, archive usage.
- [ ] Add structured logs for node-type detection and signer selection at startup.
- [ ] Add dashboards/alerts: bundler inclusion latency, dropped-op rate, signer
      failure rate, store availability, node availability.
- [ ] Ensure readiness includes store + signer + node (currently store + eth only).

---

## 7. Security

- [ ] No raw private key required in production; prefer KMS/HSM/SaaS signer.
- [ ] TLS everywhere (Redis, RPC, signer APIs); verify certificates.
- [ ] Least-privilege IAM per component; document required permissions.
- [ ] Never log secrets, signed payloads, or raw key material.
- [ ] Network policies: bundler egress limited to configured RPC endpoints,
      store, and signer.
- [ ] Run as non-root (already distroless/`nonroot` per `Dockerfile`); keep
      read-only root filesystem where possible.

---

## 8. Testing Strategy

- [ ] Store conformance suite run against Badger, Redis, and Valkey (same tests).
- [ ] Multi-replica integration test: two bundler instances sharing a store, no
      duplicate/conflicting bundles; kill/reclaim test.
- [ ] Signer conformance suite with a mock remote signer plus at least one real
      KMS integration in CI (AWS/GCP) and one SaaS signer (Turnkey or Privy).
- [ ] Node detection tests using recorded RPC fixtures for full vs. archive nodes.
- [ ] Failover test: primary node down → traffic continues on fallback.
- [ ] E2E: existing `e2e/` and compliance suites pass with Redis store + remote
      signer + archive/full matrix.
- [ ] Chaos: Redis restart, signer timeout, RPC 429/5xx, network partition.

---

## 9. Deployment / Packaging

- [ ] Provide Kubernetes manifests (Deployment, Service, PDB, HPA, NetworkPolicy,
      Secret) alongside `docker-compose.yml`.
- [ ] Provide a managed Redis/Valkey example (TLS + auth + sentinel/cluster).
- [ ] Document topology options:
  - [ ] Single active bundler + read replicas (simplest correct).
  - [ ] Active/active with distributed locks and op leases.
- [ ] Document per-chain deployment (one deployment per chain vs. multi-chain).
- [ ] Update `docs/DEPLOYMENT.md` and `README.md`.

---

## 10. Documentation

- [ ] New `docs/STORAGE.md` — backends, key layout, TTL, locking, scaling model.
- [ ] New `docs/SIGNERS.md` — interface, per-provider setup, IAM, failure modes.
- [ ] New `docs/NODES.md` — full vs. archive, capability matrix, detection,
      per-chain configuration.
- [ ] Update `docs/CONFIGURATION.md`, `docs/ARCHITECTURE.md`, `docs/API.md`
      (new error codes for unavailable archive/capabilities).
- [ ] Migration guide from BadgerDB/embedded key to Redis/Valkey and remote signer.

---

## 11. Phasing & Milestones

### Phase 1 — Storage abstraction (foundation)
- [ ] Define store interfaces; extract Badger behind them.
- [ ] Implement Redis/Valkey backend with atomic op+index writes and TTL.
- [ ] Store conformance suite; readiness via store `Ping`.

### Phase 2 — Signer abstraction
- [ ] Define `Signer` interface; keep `raw`; add one KMS (AWS or GCP) + one SaaS
      (Turnkey/Privy).
- [ ] Rewire all `bind.NewKeyedTransactor*` and `crypto.Sign` call sites.
- [ ] Startup address verification + self-test.

### Phase 3 — Node model
- [ ] Chain-keyed config, full/archive per chain, failover.
- [ ] Capability/type auto-detection + feature gating + `/nodeinfo`.

### Phase 4 — Horizontal scaling
- [ ] Distributed locking / op leases + leader election; replica kill/reclaim tests.
- [ ] Multi-chain namespacing.

### Phase 5 — Hardening & release
- [ ] Observability, security review, K8s manifests, docs, deprecation shims.

---

## 12. Definition of Done

- [ ] Storage, signer, and node access are all interface-driven with ≥2 real backends
      each (Badger+Redis; raw+KMS+SaaS; full+archive).
- [ ] Two replicas run safely against one shared store with no double bundling.
- [ ] Bundle signing works through a remote/KMS signer end-to-end on Ethereum + L2.
- [ ] `node_type: auto` correctly identifies and gates features for full vs. archive.
- [ ] Rolling deploy loses no pending ops; failover keeps the service healthy.
- [ ] Readiness reflects store, signer, and node health.
- [ ] No raw private key is required in production.
- [ ] All docs and examples updated; existing single-instance config still works.
