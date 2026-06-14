# Deploying on Dokploy (Docker Compose)

The bundler ships a production `docker-compose.yml` that runs a single instance
serving all ERC-4337 versions (v0.6–v0.9) and reads the EOA private key from a
**Docker secret** rather than a plaintext environment variable.

## What's in the box

- `docker-compose.yml` — the service definition (build, healthcheck, volume,
  secret wiring, restart policy).
- `secrets/bundler_private_key.txt.example` — template for the key file.
- `.env.example` — non-secret config (RPC URL, etc.).

## How the secret is wired

The binary only reads the key from `ERC4337_BUNDLER_PRIVATE_KEY`. Compose mounts
the secret file at `/run/secrets/bundler_private_key`, and the service entrypoint
loads it into that variable at startup — stripping whitespace and an optional
`0x` prefix — then `exec`s the bundler so it stays PID 1 and shuts down cleanly
on SIGTERM. The key never appears in the compose file, image, or `docker inspect`.

## Steps

1. **Create the secret file** next to `docker-compose.yml`:

   ```sh
   install -m 0444 /dev/null secrets/bundler_private_key.txt
   printf '%s' 'YOUR_BARE_HEX_KEY' > secrets/bundler_private_key.txt   # no 0x, no newline
   chmod 0444 secrets/bundler_private_key.txt
   ```

   > The in-container bundler runs as uid **10001**. With plain `docker compose`
   > (Dokploy's default, non-swarm) the secret keeps the host file's permissions,
   > so it must be world-readable (`0444`) for that user to read it. The
   > entrypoint fails fast with a clear message if it can't.

2. **Set non-secret config.** In Dokploy's *Environment* settings (or a `.env`
   file beside the compose file) set at minimum:

   ```
   ERC4337_BUNDLER_ETH_CLIENT_URL=https://your-rpc-endpoint
   ```

   `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` defaults to all four canonical
   EntryPoints, so leave it unset to serve every version. See
   [CONFIGURATION.md](./CONFIGURATION.md) for the full list.

3. **Deploy.** Point the Dokploy *Docker Compose* service at this repo. Dokploy
   builds the image and runs the stack. To expose it, either keep the
   `4337:4337` port mapping or attach a domain and let Dokploy's Traefik proxy
   route to container port `4337` (then you can drop the `ports:` mapping).

## Verifying

- Liveness: `GET /live` → `{"status":"ok"}` (used by the healthcheck).
- Readiness: `GET /ready` → `{"ready":true,"checks":{"db":"ok","eth_client":"ok"}}`.
- Versions served: `eth_supportedEntryPoints` returns all four EntryPoint
  addresses.
- Confirm the right key loaded: the startup log prints the derived
  `EOA Address` — it must match the bundler EOA you funded for gas.

The BadgerDB mempool persists in the named `bundler-data` volume across restarts
and redeploys.
