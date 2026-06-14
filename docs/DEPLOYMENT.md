# Deploying on Dokploy (Docker Compose)

The bundler ships a production `docker-compose.yml` that runs a single instance
serving all ERC-4337 versions (v0.6–v0.9). The EOA private key is read from the
**environment variable** `ERC4337_BUNDLER_PRIVATE_KEY`, exactly like the RPC URL.

## How the key is wired

The binary reads the key directly from `ERC4337_BUNDLER_PRIVATE_KEY`.
`docker compose` resolves it from a `.env` file next to the compose file or from
Dokploy's Environment settings. No startup wrapper or file mount is used, so the
bundler process remains PID 1 and shuts down cleanly on SIGTERM.

> The private key value is not baked into the image or stored in the Git repo,
> but like all container environment variables it is visible via
> `docker inspect` and the Docker daemon. On Dokploy, mark the variable as
> **sensitive/secret** in the Environment tab so it is masked in the UI.

## Steps (Dokploy)

1. **Set required config.** App → **Environment** tab → add at minimum:

   ```
   ERC4337_BUNDLER_ETH_CLIENT_URL=https://your-rpc-endpoint
   ERC4337_BUNDLER_PRIVATE_KEY=YOUR_BARE_HEX_KEY
   ```

   The key may be supplied with or without a leading `0x`. Dokploy writes these
   to a `.env` file next to the compose, and `docker compose` uses them for
   `${…}` interpolation. Both variables are marked **required**, so the deploy
   fails fast until they are set.

   `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` defaults to all four canonical
   EntryPoints — leave it unset to serve every version. See
   [CONFIGURATION.md](./CONFIGURATION.md) for the full list.

2. **Deploy.** Dokploy runs
   `docker compose -p <app> -f ./docker-compose.yml up -d --build`. To expose the
   service, either keep the `4337:4337` port mapping or attach a domain and let
   Dokploy's Traefik proxy route to container port `4337` (then drop the
   `ports:` mapping).

> If the container exits with `Fatal config error: erc4337_bundler_private_key not
> set`, `ERC4337_BUNDLER_PRIVATE_KEY` is missing from the Environment tab.

## Local testing (plain `docker compose`, no Dokploy)

Copy `.env.example` to `.env` and fill in the two required variables:

```sh
cp .env.example .env
# edit .env
ERC4337_BUNDLER_ETH_CLIENT_URL=https://your-rpc
ERC4337_BUNDLER_PRIVATE_KEY=YOUR_BARE_HEX_KEY
BUNDLER_VERSION=dev
```

Then run:

```sh
docker compose up --build
```

`.env` is gitignored.

## Verifying

- Liveness: `GET /live` → `{"status":"ok"}` (used by the healthcheck).
- Readiness: `GET /ready` → `{"ready":true,"checks":{"db":"ok","eth_client":"ok"}}`.
- Versions served: `eth_supportedEntryPoints` returns all four EntryPoint
  addresses.
- Confirm the right key loaded: the startup log prints the derived
  `EOA Address` — it must match the bundler EOA you funded for gas.

The BadgerDB mempool persists in the named `bundler-data` volume across restarts
and redeploys.

## Notes on the runtime image

The final image is `gcr.io/distroless/cc-debian12:nonroot`. It has no package
manager, shell, or utilities, so the bundled Docker HEALTHCHECK uses a built-in
`healthcheck` subcommand (`/app/qubenode-bundler healthcheck`) that probes
`http://127.0.0.1:4337/live`. The process runs as the unprivileged `nonroot`
user (uid 65532).
