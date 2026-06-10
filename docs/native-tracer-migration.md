# Native Tracer Migration — Engineering Plan (IC2)

**Owner:** _TBD_  ·  **Reviewer:** _TBD_  ·  **Status:** Implemented (pending live-node validation)  ·  **Branch base:** `main`

## Implementation status (2026-06-10, branch `chore-use-native-tracor`)

Phases 0–3 and the 2b mux helper are implemented; both latent bugs from §3 were
confirmed statically and fixed. Summary of what landed and where it deviates
from the plan:

- **Both §3 latent bugs confirmed.** (1) The production validation path for
  v0.7+ ops was `checks/simulate.go` → root `simulation.TraceSimulateValidation`,
  which traced v0.6 `simulateValidation` (nonexistent on v0.7+ EntryPoints)
  with a hardcoded `callTracer` parsed into `BundlerCollectorReturn` — shape
  mismatch, rules saw empty data. (2) v0.7/0.8/0.9 `TraceSimulateHandleOp`
  ignored `in.Tracer`, sent the JS execution tracer, and parsed the response
  as a callTracer tree — total mis-parse, `ExecutionGasLimit` was always 0.
- **New:** `pkg/tracer/opcodes.go` (256-entry table + tolerant key parsing),
  `pkg/tracer/erc7562.go` (native frame types; accepts hex/decimal opcode keys
  and both extCodeAccessInfo encodings), `pkg/tracer/erc7562_adapter.go`
  (`FromErc7562` + per-top-level-call hook), `pkg/tracer/native.go` (tracer
  selection + muxTracer envelope/config),
  `pkg/entrypoint/execution/util.go` rewritten (structural innerHandleOp phase
  detection via ABI-derived selectors, 63/64 recursion, UserOperationEvent +
  getCurrentUserOpHash extraction), `pkg/entrypoint/v09/execution/muxtrace.go`
  (`TraceCombined`, one muxTracer round-trip).
- **Deviation — flat `Calls` kept (1.3):** instead of refactoring
  `NewCallStack` to walk the tree, the adapter synthesizes the JS-format flat
  calls list. Dropping the flat representation is impossible while the JS
  fallback exists (it feeds the same consumers), and synthesis keeps one
  `ProcessTraceResult` for both paths. Revisit at Phase 4 when JS is deleted.
- **Deviation — native is the default (3.1):** `""`/unset selects native;
  only explicit `js` selects the JS fallback. Rationale: the JS wiring was
  broken in production (bugs above), so there is no working JS behavior to
  soak against, and the plan is perf-driven.
- **Decision — selectors derived from bindings ABI**, not hardcoded: the JS
  tracers' hardcoded v0.9 selectors are stale (`handleOps` is `0x765e827f`
  per ABI, JS said `0x1a5d1cd7`; `getCurrentUserOpHash` is `0xb0a398d1`, JS
  said `0x60e6b9fb`) — another reason the native path derives everything from
  `*MetaData.GetAbi()`.
- **Fixed in passing:** `checks/simulate.go` now dispatches v0.7/v0.8/v0.9
  validation to the version-specific simulation packages (tracing `handleOps`)
  instead of converting to v0.6 and tracing `simulateValidation`; the JS
  branch of v0.7+ execution now parses the JS response as
  `BundlerExecutionReturn` (fixing the mis-parse for the fallback path too).
- **Not done (needs a debug-enabled node):** Phase 0 live fixtures — the
  committed fixtures under `pkg/tracer/testdata/` are synthetic,
  geth-erc7562-shaped. Validate against the target node before flipping prod:
  0.2 banned-opcode visibility in `usedOpcodes`, 0.3 `withLog` UserOperationEvent
  capture, and the muxTracer envelope. The tolerant decoders cover the known
  encoding variants.
- **Not done:** parity harness (§7) — of limited value because the JS path
  never worked in prod for v0.7+; compare against live traces instead.
  Phase 4 JS deletion — deferred until soak, per plan. 2b production wiring —
  `TraceCombined` exists but no current flow runs validation and execution in
  the same request; wire it when such a flow appears.

## 1. Goal

Replace the two embedded JavaScript tracers in `pkg/tracer/` with native node tracers:

1. **Collector** (ERC‑7562 validation rules) → native **`erc7562Tracer`**.
2. **Execution** (gas estimation / OOG / reverts) → native **`callTracer`** (`{"withLog": true}`),
   processed by a corrected `ProcessBundlerExecution` adapter.

End state: the `*.js` tracer files and the JS string-loading machinery are deleted; both
`debug_traceCall` paths use native tracers; all version paths (v0.6 / v0.7 / v0.8 / v0.9) behave
identically and correctly.

### Non‑goals
- No change to validation *rules* themselves (ERC‑7562 semantics stay the same).
- No change to the gas‑estimation binary‑search algorithm in `pkg/gas/estimate.go` (only the data
  it consumes is re‑sourced).
- No new mempool/altmempool behavior.

## 2. Why (one paragraph)

The JS tracers run inside the node on a Goja interpreter, one callback per opcode. The native
`erc7562Tracer` and `callTracer` produce equivalent (better-structured) data far faster and are the
standardized path used by go-ethereum / reth / erigon. The repo already anticipated this: config
fields `NativeBundlerCollectorTracer` / `NativeBundlerExecutorTracer` exist, the execution side
already parses a `callTracer`-shaped tree, and v0.6 already uses `callTracer`. This task finishes and
makes-correct that half-done migration.

## 3. Current state (read this first — it is messy)

| Area | File | Reality today |
|---|---|---|
| Config flags | `internal/config/values.go:29-30,132-133,196-197` | `NativeBundlerCollectorTracer` + `NativeBundlerExecutorTracer` env vars exist |
| Flag use (collector) | `internal/start/private.go:119`, `searcher.go:115` | passed into `checks.New(...)` → `simulate.go:71 Tracer:` |
| Flag use (executor) | `searcher.go:146` | referenced but not consistently threaded |
| Gas estimate tracer | `internal/start/private.go:148` | hardcoded `"callTracer"` |
| Collector v0.6 | `pkg/entrypoint/simulation/tracevalidation.go:61` | hardcodes `"callTracer"`, unmarshals into `BundlerCollectorReturn` — **shape mismatch, suspect** |
| Collector v0.7/8/9 | `pkg/entrypoint/v0{7,8,9}/simulation/tracevalidation.go` | uses JS `collector.Collector` |
| Execution v0.6 | `pkg/entrypoint/execution/trace.go:76` | uses `in.Tracer` (`callTracer`) → `Call` → `ProcessBundlerExecution` |
| Execution v0.7/8/9 | `pkg/entrypoint/v0{7,8,9}/execution/trace.go:88` | passes JS `exec.Execution` **but parses as `v06exec.Call` (callTracer tree)** — **mis-parsed today** |
| Execution adapter | `pkg/entrypoint/execution/util.go` | crude phase detection; **no 63/64 rule** (sums `gasUsed`); **no UserOperationEvent**; **no getCurrentUserOpHash** |

> ⚠️ Two latent bugs are implied above (collector v0.6 shape mismatch; execution v0.7/8/9
> tracer/parse mismatch). Confirm each with a live trace in Phase 0 before assuming behavior.

## 4. Key facts the implementer must internalize

1. **`erc7562Tracer` ignores struct-logger options.** Pass
   `{"tracer":"erc7562Tracer","tracerConfig":{"withLog":true}}`. Not `enableMemory` etc.
2. **`erc7562Tracer` opcode keys are numeric.** `usedOpcodes` / the `opcode` field in
   `contractSize` & `extCodeAccessInfo` are opcode **bytes** (`"0x54"`=SLOAD, `0xf1`=CALL,
   `0xfa`=STATICCALL). `BannedOpCodeSet` (`simulateutils.go:19`) matches **names**. You must add a
   256-entry number→name table. Verify GAS/NUMBER/TIMESTAMP/etc. actually appear in `usedOpcodes`
   when used (the ignore-list differs from the JS tracer).
3. **`erc7562Tracer` output is a nested call tree;** the JS collector output was pre-grouped into
   `callsFromEntryPoint[]` + flat `calls[]`. You write an adapter that walks the tree.
4. **`callTracer` is frame-level** — it cannot see the depth‑1 `NUMBER` phase marker the JS execution
   tracer used. Phase split must be derived from call-tree **structure** (the `innerHandleOp` →
   `sender` subtree) or EntryPoint `AAxx` revert codes, not opcodes.
5. **63/64 rule.** `executionGasLimit` is NOT a sum of `gasUsed`. Reproduce the JS recursion
   (`BundlerExecutionTracer.js:88-97`): bottom‑up, `required += used - nested.used +
   ceil(nested.required * 64 / 63)`, computed only over the execution-phase subtree.
6. **`muxTracer`** can run both natives in one `debug_traceCall`
   (`{"tracer":"muxTracer","tracerConfig":{"erc7562Tracer":{...},"callTracer":{"withLog":true}}}`).
   See §4a — this is the #2 performance lever, not optional polish.

## 4a. Performance priorities (this migration is perf-driven)

Dominant cost is node-side trace + JSON over RPC, repeated up to `maxRetries=7` in the gas binary
search (`estimate.go:22`). Optimise round-trips and JS-vs-native, not Go struct-walking. Ranked:

1. **Native everywhere (no Goja).** The whole point. Do **not** keep JS for any version (see 1.6) —
   a JS fallback path that actually runs in prod silently forfeits the win for those ops.
2. **`muxTracer` to collapse the validation+execution double-trace.** ✅ **VALIDATED on the target
   node** — a `muxTracer{erc7562Tracer,callTracer}` call returns both results in one round-trip with a
   single EVM execution (identical root `gasUsed` across both sub-results). Response envelope:
   `result.callTracer` + `result.erc7562Tracer` as sibling keys, each the same shape as the
   standalone tracer. Go type: `struct { CallTracer CallFrame; Erc7562Tracer Erc7562Frame }`.
   Today each op does two `debug_traceCall`s (collector + execution). For **v0.9 both trace the same
   `ep.HandleOps` call** (`v09/simulation/tracevalidation.go`, `v09/execution/trace.go`), so muxing
   gives one round-trip instead of two. Caveats: v0.6 traces `simulateValidation` (collector) vs
   `handleOps` (execution) — different calls, **cannot** mux. And the gas search loop re-traces
   `handleOps` N times but only needs `callTracer` each iteration — collector runs once. So muxing
   wins on the one-time validation+execution pass (notably v0.9), not the search loop. **Verify the
   traced calldata is identical before muxing.**
   Note: callTracer also returns `beforeEVMTransfers`/`afterEVMTransfers` (geth extension) — ignore
   or capture, not required.
3. **Tree-walk over flat `[]CallInfo`** (1.3) — free, negligible.

> JS fallback (§3.1) is for *unsupported nodes only*. If it runs on a supported node it defeats the
> migration — make the startup probe pick native whenever available.

## 5. Consumers the adapters must satisfy (do not break these)

**Collector → `tracer.BundlerCollectorReturn`** consumed by:
- `ProcessTraceResult` (`.../simulation/tracevalidation.go`) — iterates `OOG`, `ExtCodeAccessInfo`,
  `Opcodes` (vs `BannedOpCodeSet`/`BannedUnstakedOpCodeSet`), `ContractSize`.
- `NewKnownEntity` (`knownentity.go:27`) — maps sender/factory/paymaster → `CallFromEntryPointInfo`.
- `NewCallStack` (`callstack.go:30`) — consumes `[]tracer.CallInfo` for paymaster-context / `balanceOf`
  / value-transfer checks.
- `newStorageSlotsByEntity` (`storageslots.go:27`) — consumes `Keccak []string`.

**Execution → `tracer.BundlerExecutionReturn`** consumed by:
- `pkg/gas/estimate.go:360` (`ExecutionGasLimit` → `callGasLimit`), `:333/:377/:398` (OOG via
  error strings derived from `ValidationOOG`/`ExecutionOOG`).
- `pkg/entrypoint/*/execution/trace.go` — `Reverts`, `Output`, `Error`, `CurrentUserOpHash` (v0.9),
  and (target) `UserOperationEvent`.

The field set of `BundlerCollectorReturn` / `BundlerExecutionReturn` (`pkg/tracer/types.go`,
`pkg/entrypoint/execution/util.go:10`) is the contract. Keep it stable; only change producers.

---

## 6. Work breakdown

### Phase 0 — Verification spikes (no production code) ~0.5 day
- [ ] **0.1** Against a `debug`-enabled node, run `erc7562Tracer` on a real v0.7 `handleOps`
  simulation and a v0.6 `simulateValidation`. Save JSON fixtures under `pkg/tracer/testdata/`.
- [ ] **0.2** Run a trace that deliberately hits a banned opcode (e.g. `TIMESTAMP`/`NUMBER`) and
  confirm it shows up in `usedOpcodes`. Record the opcode-byte values observed.
- [ ] **0.3** Run `callTracer` `{withLog:true}` on the same v0.7 handleOps and save the fixture.
  Confirm the UserOperationEvent log and (v0.9) `getCurrentUserOpHash` STATICCALL are present.
  ⚠️ **`withLog` is still UNVALIDATED** — the muxTracer spike used a Multicall3 call that emits no
  events, so no `logs` fields appeared. The UserOperationEvent extraction path is unproven until a
  real event-emitting `handleOps` trace shows `logs` populated on the emitting frame. Make this
  fixture an op that actually emits `UserOperationEvent`.
- [ ] **0.3b** muxTracer round-trip: ✅ already confirmed working on the target node (envelope
  `result.{callTracer,erc7562Tracer}`, single EVM run). Save that response as a fixture too.
- [ ] **0.4** Confirm/deny the two suspected latent bugs (§3). Write findings in the PR description.

**Exit:** fixtures committed; opcode table values confirmed; node tracer support confirmed.

### Phase 1 — Collector → `erc7562Tracer` ~2–3 days

- [ ] **1.1 Opcode table.** Add `pkg/tracer/opcodes.go`: `func OpName(code uint64) string` (full
  256 map). Unit-tested against every name in `BannedOpCodeSet` + `BannedUnstakedOpCodeSet`.
- [ ] **1.2 Native types.** Add `pkg/tracer/erc7562.go`: `Erc7562Frame` struct matching the JSON
  (`type,from,to,input,output,value,gas,gasUsed,usedOpcodes map[string]float64,
  accessedSlots{reads/writes/transientReads/transientWrites}, extCodeAccessInfo[], contractSize
  map[addr]{contractSize,opcode}, keccak[], logs[], outOfGas, calls[]Erc7562Frame`).
- [ ] **1.3 Adapter.** Add `pkg/tracer/erc7562_adapter.go`:
  `func FromErc7562(root *Erc7562Frame, entryPoint common.Address) (*BundlerCollectorReturn, error)`.
  Mapping rules:
  - The **top-level calls from the EntryPoint** = direct children of the frame whose `to ==
    entryPoint`. Each becomes one `CallFromEntryPointInfo`:
    - `TopLevelMethodSig` = first 4 bytes of child `input`; `TopLevelTargetAddress` = child `to`.
    - `Opcodes` = subtree-aggregate of `usedOpcodes`, keys mapped via `OpName`.
    - `Access[frame.to]` = subtree-aggregate of `accessedSlots.reads/writes` (key by the frame's
      `to`, mirroring JS `log.contract.getAddress()`).
    - `ContractSize`, `ExtCodeAccessInfo` = subtree-aggregate (map `opcode` byte → name).
    - `OOG` = any `outOfGas` in subtree.
    - `TSTORE`/`TLOAD` = counts from `transientWrites`/`transientReads` (or opcodes `0x5d`/`0x5c`).
  - `Keccak` = global aggregate of all frames' `keccak`.
  - `Calls`: **Decision (resolved): refactor `NewCallStack` to walk the `erc7562` tree directly**
    and drop the flat `[]CallInfo` representation. One pass, no intermediate slice allocation,
    less code. (End-to-end perf impact is negligible vs the RPC/trace cost, but it is strictly
    cheaper and cleaner, so prefer it.)
  - v0.7+ flags (`TSTORE/TLOAD`) come from above; `EIP7702Auth`, `PaymasterSignature`,
    `getCurrentUserOpHash` are **not** derivable from `erc7562Tracer` — see 1.6.
- [ ] **1.4 Wire collector selection.** Introduce a single chooser, e.g.
  `tracer.CollectorTracer(cfg) (name string, useNative bool)` driven by
  `NativeBundlerCollectorTracer` (`""` or `"js"` ⇒ JS fallback; `"erc7562Tracer"` ⇒ native). Thread
  it through all four `simulation/tracevalidation.go` files so they either send the JS string + parse
  `BundlerCollectorReturn`, or send `erc7562Tracer` + parse `Erc7562Frame` → `FromErc7562`.
  Remove the v0.6 hardcoded `"callTracer"` (§3 bug).
- [ ] **1.5 Tests.** Golden tests: feed the Phase 0 fixtures through `FromErc7562` and assert the
  resulting `BundlerCollectorReturn` drives `ProcessTraceResult` to the same accept/reject as the JS
  path. Add a banned-opcode fixture that must reject.
- [ ] **1.6 Version-specific captures.** For v0.8 `EIP7702Auth`, v0.9 `PaymasterSignature` /
  `getCurrentUserOpHash`: **Decision (resolved, perf-driven): inline-derive these from the native
  call tree — do NOT keep the JS collector for v0.8/v0.9.** Keeping JS for those versions means they
  never get the native speedup (full Goja per-opcode cost on every trace), which defeats the
  migration. `getCurrentUserOpHash` = output of the EntryPoint STATICCALL; `PaymasterSignature` /
  `EIP7702Auth` = derive from the relevant frame inputs / auth list. Budget extra time here; it is
  the price of all-native.

**Exit:** with `NativeBundlerCollectorTracer=erc7562Tracer`, all simulation tests pass for the
versions in scope; JS path still works when flag unset.

### Phase 2 — Execution → `callTracer` ~2–3 days

- [ ] **2.1 Standardize the tracer string.** In v0.7/v0.8/v0.9 `execution/trace.go`, replace
  `Tracer: exec.Execution` with the configured executor tracer (default `"callTracer"`,
  `{"withLog": true}`), matching v0.6. Remove the JS `exec.Execution` lookups.
- [ ] **2.2 Parse logs.** Extend `v06exec.Call` (`util.go:25`) with `Logs []LogInfo` (callTracer
  `withLog` emits `logs:[{address,topics,data}]` per frame).
- [ ] **2.3 Fix `ProcessBundlerExecution`** (`util.go:39`):
  - **Phase detection:** identify the execution-phase subtree structurally — the EntryPoint's
    `innerHandleOp`/account-execution call — instead of "first call leaving the EntryPoint".
    Document the selector(s) used per version.
  - **63/64 `ExecutionGasLimit`:** implement the bottom-up recursion (§4.5) over the execution
    subtree. Replace the `+= gasUsed` line.
  - **UserOperationEvent:** scan frame `logs` for `topics[0] ==` the version's
    `UserOperationEvent` signature; populate `BundlerExecutionReturn.UserOperationEvent`
    (add the field; it exists on the `pkg/tracer` type but not on the `execution` package type —
    reconcile the two `BundlerExecutionReturn` definitions, see 2.5).
  - **getCurrentUserOpHash (v0.9):** find the STATICCALL to the EntryPoint selector and capture its
    `output` into `CurrentUserOpHash`.
- [ ] **2.4 Validation vs execution OOG:** keep relying on EntryPoint `AAxx` revert codes (already
  matched in `estimate.go:isValidationOOG`) plus the structural phase flag for pure OOG. Add tests
  for both an AA4x validation OOG and a deep execution OOG.
- [ ] **2.5 Reconcile duplicate types.** There are two `BundlerExecutionReturn` definitions
  (`pkg/tracer/types.go:98` and `pkg/entrypoint/execution/util.go:10`). Collapse to one (prefer the
  `pkg/tracer` one) so `UserOperationEvent`/`CurrentUserOpHash` flow without copying.
- [ ] **2.6 Tests.** Golden tests from the Phase 0 callTracer fixture: assert `ExecutionGasLimit`
  matches the JS tracer's value within tolerance on a known op; assert reverts/event extraction.

**Exit:** gas estimation produces the same `callGasLimit` (±small tolerance) as the JS execution
tracer on a fixture set; all four versions use `callTracer`.

### Phase 2b — `muxTracer` single round-trip (perf lever #2) ~1 day
- [ ] **2b.1** For v0.9 only (and any version where the collector and execution trace the *same*
  calldata — verify first), replace the two separate `debug_traceCall`s with one `muxTracer` call
  returning both `erc7562Tracer` and `callTracer` results; split the muxed response and feed the two
  existing adapters. Do this only for the one-time validation+execution pass, not the gas search loop.
- [ ] **2b.2** Benchmark: ops/sec and p50/p99 trace latency, muxed vs two-call, on a representative
  op set. Keep the two-call path for versions that can't mux.

**Exit:** v0.9 validation+execution uses a single round-trip; benchmark shows the expected ~2×
reduction in trace round-trips for that pass.

### Phase 3 — Config, fallback, wiring ~0.5 day
- [ ] **3.1** Document env vars in README/config: `..._native_bundler_collector_tracer`
  (`erc7562Tracer`|`js`), `..._native_bundler_executor_tracer` (`callTracer`|`js`). Defaults:
  decide JS-default for one release, native-default after soak (raise with reviewer).
- [ ] **3.2** Ensure both `internal/start/private.go` and `searcher.go` thread both flags
  consistently (today the executor flag is under-wired; the gas path hardcodes `callTracer` at
  `private.go:148`).
- [ ] **3.3** Startup log line stating which tracer (native/js) is active for collector & executor.

### Phase 4 — Cleanup (only after native is default & soaked) ~0.5 day
- [ ] **4.1** Delete `BundlerCollectorTracer*.js` and `BundlerExecutionTracer*.js`.
- [ ] **4.2** Delete the JS loader: `parse`, `NewTracers`, `Tracers`, `BundlerTracerSet`,
  `ForVersion`, the `//go:embed` block, `Loaded` (`pkg/tracer/load.go`, `values.go`).
- [ ] **4.3** Remove `js_test.go`, `load_test.go`, and now-dead `entryPointPlaceholder` logic.
- [ ] **4.4** `go vet ./... && golangci-lint run` clean; remove unused deps.

## 7. Testing strategy
- **Unit:** opcode table (every banned name resolves); `FromErc7562` golden tests; 63/64 recursion
  unit test with a hand-built tree; phase-detection unit tests.
- **Integration (fixtures):** Phase 0 JSON fixtures committed under `pkg/tracer/testdata/`; table
  tests run both JS and native adapters and assert identical accept/reject + matching gas numbers.
- **Parity harness (temporary):** behind a debug flag, run BOTH tracers for one release and log when
  native and JS disagree (accept/reject or `callGasLimit` delta > tolerance). Remove in Phase 4.
- **Regression:** existing `pkg/gas`, `pkg/entrypoint/**/simulation`, `pkg/modules/checks` suites
  must stay green with the flag unset (JS) and set (native).

## 8. Risks & mitigations
| Risk | Mitigation |
|---|---|
| Node lacks `erc7562Tracer`/`callTracer` | Keep JS fallback; startup probe + clear error |
| Opcode ignore-list differs → missed banned opcode | Phase 0.2 explicit banned-opcode fixture; table test |
| 63/64 reconstruction under/over-estimates gas | Parity harness + tolerance test vs JS before flipping default |
| Phase detection wrong on exotic call shapes (7702, paymaster) | Structural detection per version + fixtures for each; staged default flip |
| v0.8/v0.9 captures not derivable from native | Keep JS for those captures (1.6) or follow-up ticket |

## 9. Acceptance criteria
1. With native flags set, the full test suite is green across v0.6–v0.9.
2. Parity harness shows zero accept/reject disagreements and `callGasLimit` within tolerance on the
   fixture corpus.
3. v0.7/v0.8/v0.9 no longer pass a JS tracer string to `debug_traceCall`.
4. (Phase 4) No `*.js` tracer files or JS loader remain; build/lint clean.

## 10. Suggested PR sequence (keep each reviewable)
1. PR-0: fixtures + opcode table + native types (no behavior change).
2. PR-1: collector adapter + flagged wiring + tests (JS still default).
3. PR-2: execution `callTracer` standardization + `ProcessBundlerExecution` fix + type reconcile.
4. PR-3: config docs + parity harness + startup logging.
5. PR-4: flip defaults to native (after soak).
6. PR-5: delete JS tracers + loader.
