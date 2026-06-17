# Deploying to Dokploy

This project deploys as **separate Dokploy Applications** plus a **managed
Postgres Database**. Each app is independently buildable and rollable, so the
stateless services can do zero-downtime / blue-green redeploys without ever
touching the database.

> Why not one Compose service? Blue-green runs the new stack beside the old one
> before switching traffic. A Postgres container inside that compose would be
> duplicated onto the same data volume (lock/corruption) and its migrations would
> mutate the schema the old version is still serving. The DB must stay outside
> the rotation — hence a managed Database + stateless Applications.

| Piece     | Dokploy type            | Source              | Routing on `app.example.com`        |
|-----------|-------------------------|---------------------|-------------------------------------|
| postgres  | **Database** (Postgres) | Dokploy-managed     | internal                            |
| core      | **Application** (Dockerfile) | `core/Dockerfile` | path `/api` (Strip Path) → :8080 |
| frontend  | **Application** (Dockerfile) | `frontend/Dockerfile` | path `/` → :80                  |
| lookout   | **Application** (Dockerfile) | `lookout/Dockerfile` | internal                        |

**Routing is single-origin via Traefik path rules on one shared domain:** `/api`
→ core, everything else → frontend. Because it's same-origin, **no CORS** is
needed. The SPA calls the API at the relative `/api` path.

> The `/api` route **must enable "Strip Path"** so core (whose routes live at the
> root: `/health`, `/accounts`, …) receives `/accounts`, not `/api/accounts`.

> ⚠️ **Build context = repo root for every app.** Dokploy defaults a Dockerfile
> app's build context to the *directory of the Dockerfile*. These Dockerfiles
> must build from the **repo root** — core/lookout pull in the shared `specs/`
> dir (and lookout the `core/` module), which live outside each service folder.
> So in every app's build settings set **Docker Context Path = `.`** while
> keeping **Docker File = `<service>/Dockerfile`**. If the context is wrong the
> build fails with `COPY … "not found"` (e.g. `/core/migrations`), sometimes only
> after a cache layer is invalidated — so clear the build cache when changing it.

## 1. Create the Postgres Database

1. **+ Create → Database → Postgres** (in the same Dokploy project).
2. Note its credentials and **internal connection host** (shown in the DB's UI).
   These feed `core`'s env in step 2.

## 2. Create the `core` Application

1. **+ Create → Application**, point at this repo.
2. **Build Type = Dockerfile.**
   - **Docker File:** `core/Dockerfile`
   - **Docker Context Path:** `.` (the repo root — the build needs the shared
     `specs/` dir, which lives outside `core/`).
3. **Environment:**
   ```
   APP_NAME=finance
   HTTP_PORT=8080
   LOG_LEVEL=info
   BASE_CURRENCY=UZS
   CORS_ORIGIN=                             # empty — same-origin, no CORS
   INGEST_TOKEN=<strong-shared-secret>      # bearer for lookout's ingest calls
   AUTH_USERNAME=<login>                    # HTTP Basic creds the UI logs in with
   AUTH_PASSWORD=<strong-password>
   POSTGRES_HOST=<managed-db-internal-host> # from step 1
   POSTGRES_PORT=5432
   POSTGRES_USER=<from step 1>
   POSTGRES_PASSWORD=<from step 1>
   POSTGRES_DB=<from step 1>
   POSTGRES_SSLMODE=disable
   ```
4. **Domains:** add host `app.example.com`, **Path `/api`**, **Strip Path = on**,
   container port **8080**.
5. Deploy. The container's ENTRYPOINT runs `migrate up` before starting the API,
   so the schema is created/updated automatically.
   Health check: `GET /health` → `{"status":"ok"}`.

## 3. Create the `frontend` Application

1. **+ Create → Application**, same repo. **Build Type = Dockerfile.**
   - **Docker File:** `frontend/Dockerfile`
   - **Docker Context Path:** `.`
2. **Build Arg** (baked at build time, not runtime) — relative, so the same image
   works on any domain:
   ```
   VITE_API_URL=/api
   ```
   (This is the Dockerfile default, so the arg is optional.)
3. **Domains:** add host `app.example.com`, **Path `/`**, container port **80**.
4. Deploy.

## 4. Create the `lookout` Application (Telegram userbot)

1. **+ Create → Application**, same repo. **Build Type = Dockerfile.**
   - **Docker File:** `lookout/Dockerfile`
   - **Docker Context Path:** `.`
2. **Environment:**
   ```
   TELEGRAM_API_ID=<from my.telegram.org>
   TELEGRAM_API_HASH=<from my.telegram.org>
   SOURCE_BOT=<bank bot @username>
   AUTH_MODE=qr
   POLL_INTERVAL=60s
   TRANSFER_PAIR_WINDOW=120s
   TRANSFER_HOLD_DURATION=5m
   TIMEZONE=Asia/Tashkent
   LOG_LEVEL=info
   FINANCE_API_URL=http://<core-app-name>:8080   # core's internal name on dokploy-network
   FINANCE_API_TOKEN=<same value as core's INGEST_TOKEN>
   ```
   - `<core-app-name>` is core's service name on the shared `dokploy-network`
     (shown in the core app's UI); this reaches core directly, bypassing Traefik
     so no `/api` prefix is involved. Or use the public `https://app.example.com/api`.
3. **Persistent storage:** add a **Volume Mount** so the Telegram session +
   watermark survive redeploys (without it, you'd re-scan the QR every deploy):
   - mount path **`/data`** (the container's working dir; `SESSION_FILE` and
     `STATE_FILE` are written there).
4. No domain needed (lookout only makes outbound calls).
5. Deploy. The container runs `lookout --wait-session`: with no session file yet
   it **idles**, logging `no session file yet; waiting for sign-in` every 10s
   (it does *not* crash-loop and does *not* attempt interactive auth — a deploy
   container has no TTY to type a 2FA password into).
6. Do the **one-time login** from an interactive shell into the container. In
   Dokploy open the `lookout` app's **Terminal** (or `docker exec -it <container>
   sh` on the host) and run:
   ```
   lookout --sign-in
   ```
   It prints a QR — in Telegram: **Settings → Devices → Link Desktop Device** and
   scan it, then (because the account has 2FA) **type the cloud password** at the
   prompt. On success it writes `session.json` to `/data` and exits. The
   `--wait-session` process then detects the file and starts the poll loop;
   later redeploys log in silently.

   > A real TTY is required for `--sign-in` (the QR is shown and the 2FA password
   > is read from stdin). Dokploy's Terminal and `docker exec -it` both provide one.
   >
   > Alternative: create `session.json` locally (`cd lookout && make run`) and
   > upload it into the `/data` volume.

## Zero-downtime redeploys

With this layout, enable Dokploy's rolling/zero-downtime setting on the
**`core`** and **`frontend`** apps. They are stateless, so a new version starts,
passes health checks, and takes over before the old one stops — the managed
Postgres is untouched.

> Migration caveat: because `core` runs migrations on boot, keep migrations
> **backward-compatible** (additive) so the old version keeps working during the
> overlap window. Avoid destructive column/table drops in the same deploy that
> ships code depending on the new shape.

## Local development

`docker-compose.yml` runs **only Postgres**. Start it with `docker compose up -d`
and run each service natively against it (`core`: `make run`, `frontend`:
`npm run dev`, `lookout`: `make run`), each reading its own `.env`. To verify a
production image build locally, build a service directly, e.g.
`docker build -f core/Dockerfile -t core .` from the repo root.

## Notes

- **Build context is the repo root** for every app — the Go services regenerate
  code from `specs/` at build time, and `lookout` also compiles against the
  `core` module (`replace finance => ../core`).
- **Generated code** isn't committed; it's recreated inside each image build.
- **State** lives only in the managed Database and lookout's `/data` volume —
  both survive app redeploys.
