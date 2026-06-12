#!/usr/bin/env bash
# scripts/e2e/run.sh — orchestrate the v0.9 e2e suite against geth --dev.
#
# What it does:
#   1. Boots a geth dev-mode node (local binary if installed, docker otherwise).
#   2. Builds the qubenode-bundler binary.
#   3. CREATE2-deploys the v0.9 EntryPoint (and the Arachnid proxy) on it.
#   4. Starts the bundler in the background, pointed at the node.
#   5. Runs the TS test suite via vitest.
#   6. Tears everything down (geth + bundler), even on failure.
#
# No fork RPC URLs are required: the chain starts empty and every contract the
# suite needs is deployed during setup. geth dev mode ships the native
# erc7562Tracer/callTracer, so the bundler runs its default native-tracer path.
#
# Usage:
#   scripts/e2e/run.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_DIR="${REPO_ROOT}/e2e"
ARTIFACTS_DIR="${E2E_DIR}/artifacts"
LOG_DIR="${REPO_ROOT}/tmp/e2e"
mkdir -p "${LOG_DIR}"

GETH_IMAGE="ethereum/client-go:v1.16.9"
GETH_PORT="${E2E_GETH_PORT:-8545}"
BUNDLER_PORT="${E2E_BUNDLER_PORT:-4337}"
GETH_ARGS=(
  --dev
  --http
  --http.addr 0.0.0.0
  --http.port "${GETH_PORT}"
  --http.api eth,net,web3,debug
  --http.vhosts '*'
  --rpc.allow-unprotected-txs
  --miner.gaslimit 30000000
)

# Test signing keys (well-known anvil/hardhat accounts #0 and #9).
# OWNER_PK: account #0 (kept for reference; the owner key is supplied to the
# TS suite via config.ts, not consumed by this script).
OWNER_PK="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
RELAYER_PK="0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"

GETH_PID=""
GETH_CONTAINER=""
BUNDLER_PID=""

cleanup() {
  set +e
  if [[ -n "${BUNDLER_PID}" ]] && kill -0 "${BUNDLER_PID}" 2>/dev/null; then
    kill "${BUNDLER_PID}"
    wait "${BUNDLER_PID}" 2>/dev/null
  fi
  if [[ -n "${GETH_PID}" ]] && kill -0 "${GETH_PID}" 2>/dev/null; then
    kill "${GETH_PID}"
    wait "${GETH_PID}" 2>/dev/null
  fi
  if [[ -n "${GETH_CONTAINER}" ]]; then
    docker logs "${GETH_CONTAINER}" >"${LOG_DIR}/geth.log" 2>&1
    docker rm -f "${GETH_CONTAINER}" >/dev/null 2>&1
  fi
}
trap cleanup EXIT INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}
require_cmd node
require_cmd npm

start_geth() {
  if command -v geth >/dev/null 2>&1; then
    echo "starting geth --dev on :${GETH_PORT} (local binary)"
    geth "${GETH_ARGS[@]}" >"${LOG_DIR}/geth.log" 2>&1 &
    GETH_PID=$!
  else
    require_cmd docker
    echo "starting geth --dev on :${GETH_PORT} (docker ${GETH_IMAGE})"
    GETH_CONTAINER=$(
      docker run -d --rm -p "127.0.0.1:${GETH_PORT}:${GETH_PORT}" \
        "${GETH_IMAGE}" "${GETH_ARGS[@]}"
    )
  fi
}

wait_http() {
  local url="$1" deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    if curl -sf -o /dev/null -X POST -H 'content-type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"web3_clientVersion","params":[]}' "$url"; then
      return 0
    fi
    sleep 0.5
  done
  echo "error: ${url} did not become ready in 60s" >&2
  return 1
}

wait_bundler() {
  local url="$1" deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    if curl -sf -o /dev/null "${url%/}/ping"; then
      return 0
    fi
    sleep 0.5
  done
  echo "error: bundler at ${url} did not become ready in 60s" >&2
  return 1
}

ensure_artifacts() {
  # Install deps unconditionally: in CI the artifacts directory can be
  # restored from cache, but node_modules never is, and the deploy and test
  # steps below need it either way.
  (cd "${E2E_DIR}" && npm install --silent)
  if [[ ! -f "${ARTIFACTS_DIR}/EntryPoint.json" ]]; then
    echo "fetching v0.9 contract artifacts"
    (cd "${E2E_DIR}" && npm run --silent fetch-artifacts)
  fi
}

# Build the bundler binary first so a compile error fails fast.
(cd "${REPO_ROOT}" && go build -o "${LOG_DIR}/qubenode-bundler" .)

start_geth
wait_http "http://127.0.0.1:${GETH_PORT}"

ensure_artifacts

# Construct the v0.9 EntryPoint for real (CREATE2 via the Arachnid proxy,
# installed by its canonical keyless transaction) so constructor-set
# immutables (senderCreator) are valid. The npm artifact's deterministic
# address is computed, not hardcoded.
entrypoint=$(
  cd "${E2E_DIR}" && \
  E2E_ARTIFACTS_DIR="${ARTIFACTS_DIR}" \
  npm run --silent deploy-entrypoint -- "http://127.0.0.1:${GETH_PORT}"
)
echo "using EntryPoint ${entrypoint}"

# Configure the bundler.
rm -f "${REPO_ROOT}/.e2e-state.json"

data_dir="${REPO_ROOT}/tmp/qubenode_bundler_e2e"
rm -rf "${data_dir}"

echo "starting bundler on :${BUNDLER_PORT}"
ERC4337_BUNDLER_ETH_CLIENT_URL="http://127.0.0.1:${GETH_PORT}" \
ERC4337_BUNDLER_PRIVATE_KEY="${RELAYER_PK#0x}" \
ERC4337_BUNDLER_PORT="${BUNDLER_PORT}" \
ERC4337_BUNDLER_DATA_DIRECTORY="${data_dir}" \
ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS="${entrypoint}" \
ERC4337_BUNDLER_ENTRYPOINT_VERSION="v0.9" \
ERC4337_BUNDLER_DEBUG_MODE="true" \
ERC4337_BUNDLER_BENEFICIARY="0xa0Ee7A142d267C1f36714E4a8F75612F20a79720" \
"${LOG_DIR}/qubenode-bundler" start --mode private \
  >"${LOG_DIR}/bundler.log" 2>&1 &
BUNDLER_PID=$!
wait_bundler "http://127.0.0.1:${BUNDLER_PORT}"

echo "running e2e suite"
(
  cd "${E2E_DIR}"
  E2E_ENTRY_POINT="${entrypoint}" \
  E2E_NODE_URL="http://127.0.0.1:${GETH_PORT}" \
  E2E_BUNDLER_URL="http://127.0.0.1:${BUNDLER_PORT}" \
  E2E_STATE_FILE="${REPO_ROOT}/.e2e-state.json" \
  E2E_ARTIFACTS_DIR="${ARTIFACTS_DIR}" \
  npm run --silent test:ethereum
)
