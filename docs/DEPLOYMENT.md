# Deploying on Dokploy (Docker Compose)

The bundler ships a production `docker-compose.yml` that runs a single instance
serving all ERC-4337 versions (v0.6–v0.9) and reads the EOA private key from a
**file** mounted at `/run/secrets/bundler_private_key` — never a plaintext
environment variable.

## How the key is wired

The binary only reads the key from `ERC4337_BUNDLER_PRIVATE_KEY`. The service
entrypoint reads the mounted file at startup — stripping whitespace and an
optional `0x` prefix — exports it into that variable, then `exec`s the bundler so
it stays PID 1 and shuts down cleanly on SIGTERM. The key never appears in the
compose file, the image, the Git repo, or `docker inspect`.

Because Dokploy deploys from a **Git clone**, the key file cannot live in the
repo. Supply it through Dokploy's File Mount feature (below).

## Steps (Dokploy)

1. **Set non-secret config.** App → **Environment** tab → add at minimum:

   ```
   ERC4337_BUNDLER_ETH_CLIENT_URL=https://your-rpc-endpoint
   ```

   Dokploy writes this to a `.env` file next to the compose, which
   `docker compose` uses for `${…}` interpolation. The compose marks this
   variable **required**, so the deploy fails fast until it is set.
   `ERC4337_BUNDLER_SUPPORTED_ENTRY_POINTS` defaults to all four canonical
   EntryPoints — leave it unset to serve every version. See
   [CONFIGURATION.md](./CONFIGURATION.md) for the full list.

2. **Add the private key as a File Mount.** App → **Advanced → Volumes/Mounts**
   → add a **File Mount**:
   - **Mount path:** `/run/secrets/bundler_private_key`
   - **Content:** the bare hex key — **no `0x`**, no trailing newline
     (the entrypoint tolerates both, but keep it clean).

   This keeps the key out of environment variables and `docker inspect`; it lives
   only in Dokploy's mount and the mounted file.

3. **Deploy.** Dokploy runs
   `docker compose -p <app> -f ./docker-compose.yml up -d --build`. To expose the
   service, either keep the `4337:4337` port mapping or attach a domain and let
   Dokploy's Traefik proxy route to container port `4337` (then drop the
   `ports:` mapping).

> If the entrypoint logs `FATAL: /run/secrets/bundler_private_key missing or
> unreadable by uid 10001`, the File Mount is absent or not readable by the
> in-container user (uid **10001**). Re-check the mount path and that its content
> is set.

## Local testing (plain `docker compose`, no Dokploy)

Supply the key by bind-mounting a host file to the same path. In
`docker-compose.yml` uncomment the bind mount under `volumes:`:

```yaml
- ./secrets/bundler_private_key.txt:/run/secrets/bundler_private_key:ro
```

then create the file (`.example` provided) and run:

```sh
printf '%s' 'YOUR_BARE_HEX_KEY' > secrets/bundler_private_key.txt   # no 0x, no newline
chmod 0444 secrets/bundler_private_key.txt
ERC4337_BUNDLER_ETH_CLIENT_URL=https://your-rpc docker compose up --build
```

`secrets/bundler_private_key.txt` is gitignored.

## Verifying

- Liveness: `GET /live` → `{"status":"ok"}` (used by the healthcheck).
- Readiness: `GET /ready` → `{"ready":true,"checks":{"db":"ok","eth_client":"ok"}}`.
- Versions served: `eth_supportedEntryPoints` returns all four EntryPoint
  addresses.
- Confirm the right key loaded: the startup log prints the derived
  `EOA Address` — it must match the bundler EOA you funded for gas.

The BadgerDB mempool persists in the named `bundler-data` volume across restarts
and redeploys.
