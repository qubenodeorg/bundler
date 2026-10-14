# Storage

The bundler keeps three pieces of mutable state: the pending **mempool**, the
**entity reputation** counters, and the **code-hash cache**. These are accessed
exclusively through the interfaces in `pkg/store`, so the backing store can be
swapped without touching the client, bundler, or modules.

## Backends

| Backend | Config value | Shared across replicas | Persistence | Intended use |
|---------|--------------|------------------------|-------------|--------------|
| BadgerDB | `badger` (default) | No | On disk | Local dev, single-instance deployments |
| Redis | `redis` | Yes | Configurable | Horizontally scaled production |
| Valkey | `valkey` | Yes | Configurable | Wire-compatible with Redis |
| Memory | `memory` | No | None | Tests, throw-away local runs |

A backend is selected with `ERC4337_BUNDLER_STORE_BACKEND`. When it is left
empty the bundler infers it: a non-empty `ERC4337_BUNDLER_STORE_URL` implies
Redis/Valkey, otherwise BadgerDB is used. This keeps existing deployments
working unchanged.

## Interfaces

```go
type Store interface {
    Mempool()    MempoolStore
    Reputation() ReputationStore
    CodeHash()   CodeHashStore
    Locker()     Locker
    Ping(ctx context.Context) error
    Close() error
}
```

Each concern has its own narrow interface (`pkg/store/mempool.go`,
`reputation.go`, `codehash.go`). Read operations return `store.ErrNotFound` for
a missing key and `store.ErrUnavailable` for a backend failure, so callers and
the readiness probe can tell "absent" from "down".

`Ping` is used by the `/ready` probe. A failure fails readiness (HTTP 503) but
not liveness, matching the Kubernetes probe contract.

## Key layout

All keys are namespaced by an optional deployment prefix and by chain ID
(`<namespace>:<chainID>:...`). Leaving both unset reproduces the legacy
BadgerDB layout so existing data keeps loading. The Redis backend always
includes the chain ID so one cluster can serve many chains.

| Concern | BadgerDB / memory key | Redis key |
|---------|-----------------------|-----------|
| Pending op | `mempool:<ep>:<sender>:<nonce>` | `...:mempool:{<ep>}:op:<sender>:<nonce>` |
| Arrival order | in-process index | `...:mempool:{<ep>}:order` (ZSET, score = sequence) |
| Per-entity index | in-process index | `...:mempool:{<ep>}:entity:<addr>` (ZSET) |
| Entity index bookkeeping | in-process index | `...:mempool:{<ep>}:idx:<sender>:<nonce>` (SET) |
| Sequence | in-process counter | `...:mempool:{<ep>}:seq` (INCR) |
| Reputation | `entity:opsCount:<addr>` | `...:reputation:<addr>` (HASH) |
| Code hashes | `checks:codeHashes:<userOpHash>` | `...:checks:codeHashes:<userOpHash>` |
| Locks | not shared | `...:lock:<name>` |

The `{<entryPoint>}` hash tag makes every key touched by a single mempool
operation hash to the same Redis Cluster slot, so the atomic scripts below are
cluster-safe.

## Atomicity and TTL

Adding or removing a Redis op is a single Lua script that writes the op value,
the arrival-order ZSET, every entity ZSET, and a small bookkeeping SET together.
Replacing an op with the same `(sender, nonce)` first removes the previous
factory/paymaster index entries, so a swapped paymaster cannot leave a stale
index behind. Reputation updates for a single logical event (for example one op
touching a sender, a factory, and a paymaster) are applied in one script so the
counters stay consistent.

Pending ops in a shared backend are given a TTL
(`ERC4337_BUNDLER_STORE_OP_TTL_SECONDS`), which defaults to
`ERC4337_BUNDLER_MAX_OP_TTL_SECONDS`. An op whose value key expires is pruned
from the ordered indexes lazily on the next read, so a crashed replica cannot
leave op state behind forever and key growth stays bounded.

## Scaling model

The store abstraction makes the bundler's state external, a prerequisite for
running more than one replica. Today the supported topology is **one active
bundler per store**:

- Both startup modes, `private` and `searcher`, run the batch loop. There is
  **no read-only replica mode**, so a second instance started against the same
  store will also try to build and submit bundles.
- To keep a rolling-deploy overlap (or any accidental multi-instance setup)
  from double-bundling, `Bundler.Process` takes a distributed per-EntryPoint
  lock for the duration of a round (via `Store.Locker()`) and skips the round
  when another replica holds it. Redis implements a TTL'd, token-checked lock;
  single-node backends return a no-op locker.
- Beyond that overlap protection, **do not run multiple active instances
  against one store**: explicit per-op claims with lease expiry and
  crash-reclaim are not implemented yet, so ops in an in-flight batch are not
  protected from a second bundler. Use exactly one active instance per store
  (roadmap section 1.4 covers the remaining coordination work).

For a single-chain, single-instance deployment the default BadgerDB backend
needs no coordination at all.

## Readiness

`/ready` reports the upstream node (`eth_client`) and the storage backend
(`store`). When the backend cannot be reached the response is `503` with
`"store": "unavailable"`, so load balancers drain the instance without the
orchestrator restarting it.

## Backward compatibility

`ERC4337_BUNDLER_DATA_DIRECTORY` continues to work: with no store URL the
bundler opens a BadgerDB in that directory using the original key layout, so an
existing deployment upgrades in place without losing pending ops.
