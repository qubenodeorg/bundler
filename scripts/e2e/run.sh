#!/usr/bin/env bash
# scripts/e2e/run.sh — orchestrate the v0.9 e2e suite.
#
# What it does:
#   1. Boots an anvil node (optionally forking mainnet or Arbitrum One).
#   2. Builds the qubenode-bundler binary.
#   3. Writes a per-chain bundler .env pointing at the anvil node.
#   4. Starts the bundler in the background.
#   5. Runs the TS test suite via vitest.
#   6. Tears everything down (anvil + bundler), even on failure.
#
# Required env vars (per chain):
#   ETH_MAINNET_RPC_URL   — for E2E_CHAIN=ethereum (optional; if unset, anvil
#                           runs without a fork)
#   ARB_MAINNET_RPC_URL   — for E2E_CHAIN=arbitrum (optional)
#
# Usage:
#   scripts/e2e/run.sh ethereum          # run Ethereum mainnet fork suite
#   scripts/e2e/run.sh arbitrum          # run Arbitrum One mainnet fork suite
#   scripts/e2e/run.sh all               # run both, sequentially

set -euo pipefail

CHAIN="${1:-ethereum}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_DIR="${REPO_ROOT}/e2e"
ARTIFACTS_DIR="${E2E_DIR}/artifacts"
LOG_DIR="${REPO_ROOT}/tmp/e2e"
mkdir -p "${LOG_DIR}"

# Test signing keys (anvil deterministic accounts #0 and #9).
OWNER_PK="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
RELAYER_PK="0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"

ANVIL_PID=""
BUNDLER_PID=""

cleanup() {
  set +e
  if [[ -n "${BUNDLER_PID}" ]] && kill -0 "${BUNDLER_PID}" 2>/dev/null; then
    kill "${BUNDLER_PID}"
    wait "${BUNDLER_PID}" 2>/dev/null
  fi
  if [[ -n "${ANVIL_PID}" ]] && kill -0 "${ANVIL_PID}" 2>/dev/null; then
    kill "${ANVIL_PID}"
    wait "${ANVIL_PID}" 2>/dev/null
  fi
}
trap cleanup EXIT INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}
require_cmd anvil
require_cmd node
require_cmd npm

wait_http() {
  local url="$1" deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if curl -sf -o /dev/null -X POST -H 'content-type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"web3_clientVersion","params":[]}' "$url"; then
      return 0
    fi
    sleep 0.5
  done
  echo "error: ${url} did not become ready in 30s" >&2
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
  if [[ ! -f "${ARTIFACTS_DIR}/EntryPoint.json" ]]; then
    echo "fetching v0.9 contract artifacts"
    (cd "${E2E_DIR}" && npm install --silent && npm run --silent fetch-artifacts)
  fi
}

run_chain() {
  local chain="$1"
  local anvil_port bundler_port chain_id fork_url_var fork_url

  case "${chain}" in
    ethereum)
      anvil_port=8545
      bundler_port=4337
      chain_id=1
      fork_url_var="ETH_MAINNET_RPC_URL"
      ;;
    arbitrum)
      anvil_port=8546
      bundler_port=4338
      chain_id=42161
      fork_url_var="ARB_MAINNET_RPC_URL"
      ;;
    *)
      echo "error: unknown chain '${chain}' (expected ethereum|arbitrum)" >&2
      exit 1
      ;;
  esac

  fork_url="${!fork_url_var:-}"

  local anvil_args=(
    --port "${anvil_port}"
    --chain-id "${chain_id}"
    --block-time 1
    --silent
  )
  if [[ -n "${fork_url}" ]]; then
    anvil_args+=(--fork-url "${fork_url}")
    echo "starting anvil on :${anvil_port} forked from ${fork_url_var}"
  else
    echo "starting anvil on :${anvil_port} (no fork — set ${fork_url_var} for fork)"
  fi

  anvil "${anvil_args[@]}" >"${LOG_DIR}/anvil-${chain}.log" 2>&1 &
  ANVIL_PID=$!
  wait_http "http://127.0.0.1:${anvil_port}"

  ensure_artifacts

  # Build the bundler binary.
  (cd "${REPO_ROOT}" && go build -o "${LOG_DIR}/qubenode-bundler" .)

  # Configure the bundler.
  rm -f "${REPO_ROOT}/.e2e-state.json"

  local data_dir="/tmp/qubenode_bundler_e2e_${chain}"
  rm -rf "${data_dir}"

  local arb_flag="false"
  if [[ "${chain}" == "arbitrum" ]]; then
    arb_flag="true"
  fi

  echo "starting bundler on :${bundler_port}"
  ERC4337_BUNDLER_ETH_CLIENT_URL="http://127.0.0.1:${anvil_port}" \
  ERC4337_BUNDLER_PRIVATE_KEY="${RELAYER_PK#0x}" \
  ERC4337_BUNDLER_PORT="${bundler_port}" \
  ERC4337_BUNDLER_DATA_DIRECTORY="${data_dir}" \
  ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS="0x433709009B8330FDa32311DF1C2AFA402eD8D009" \
  ERC4337_BUNDLER_ENTRYPOINT_VERSION="v0.9" \
  ERC4337_BUNDLER_IS_ARB_STACK_NETWORK="${arb_flag}" \
  ERC4337_BUNDLER_DEBUG_MODE="true" \
  ERC4337_BUNDLER_BENEFICIARY="0xa0Ee7A142d267C1f36714E4a8F75612F20a79720" \
  "${LOG_DIR}/qubenode-bundler" start --mode private \
    >"${LOG_DIR}/bundler-${chain}.log" 2>&1 &
  BUNDLER_PID=$!
  wait_bundler "http://127.0.0.1:${bundler_port}"

  echo "running ${chain} e2e suite"
  (
    cd "${E2E_DIR}"
    E2E_CHAIN="${chain}" \
    E2E_ANVIL_URL="http://127.0.0.1:${anvil_port}" \
    E2E_BUNDLER_URL="http://127.0.0.1:${bundler_port}" \
    E2E_STATE_FILE="${REPO_ROOT}/.e2e-state.json" \
    E2E_ARTIFACTS_DIR="${ARTIFACTS_DIR}" \
    npm run --silent "test:${chain}"
  )

  cleanup
  ANVIL_PID=""
  BUNDLER_PID=""
}

case "${CHAIN}" in
  all)
    run_chain ethereum
    run_chain arbitrum
    ;;
  *)
    run_chain "${CHAIN}"
    ;;
esac
