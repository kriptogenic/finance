# Security Audit

**Scope:** `core` (Go API), `lookout` (Telegram userbot), `Mullajiringnotifier` (Android SMS notifier), `frontend` (Vue SPA).
**Date:** 2026-06-26
**Method:** Manual source review of auth, input handling, SQL access, secret storage, transport, container/build config, and client storage. No dynamic testing.
**Threat model:** Single-user, self-hosted personal-finance system. There is one human principal; there is no multi-tenant isolation to break, so the focus is *authentication of write paths, secret handling, and transport*, not per-object authorization.

## Severity summary

| # | Component | Finding | Severity |
|---|-----------|---------|----------|
| C1 | core | Ingest endpoints are unauthenticated when `INGEST_TOKEN` is unset (auth fails open) | **High** |
| C2 | core | No brute-force protection / rate limiting on Basic auth | Medium |
| C3 | core | Database connection defaults to `sslmode=disable` (incl. prod docs) | Medium |
| C4 | core | SSRF via stored push-subscription endpoint | Low |
| C5 | core | Excel formula injection in transaction export | Low/Info |
| L1 | lookout | Ingest token optional + delivered over plaintext HTTP to core | Medium |
| L2 | lookout | Telegram session stored as plaintext JSON on disk | Low |
| N1 | notifier | `allowBackup="true"` with default (empty) backup rules | Low |
| N2 | notifier | No explicit `network_security_config`; URL scheme not validated | Low |
| F1 | frontend | Basic-auth credentials persisted in `localStorage` (reversible) | Medium |
| F2 | frontend | No security headers (CSP / X-Frame-Options / nosniff) in nginx | Medium |

---

## core

### C1 — Ingest endpoints unauthenticated when `INGEST_TOKEN` is empty (High)
`core/internal/http/middlewares/main.go:51-53`

```go
func authenticateBearer(req *http.Request, token string) error {
    if token == "" {
        return nil          // <-- auth disabled when no token configured
    }
    ...
}
```

`INGEST_TOKEN` defaults to empty (`core/config/config.go:58`, `env-default:""`). When unset, the bearer check returns `nil` for *every* request, so `/ingest/transactions` and `/ingest/balances` (the only `IngestAuth`-protected paths, `specs/api.yaml:514,547`) accept fully anonymous writes. An attacker who can reach the core service can forge arbitrary transactions and balance snapshots, corrupting net-worth/spending data and driving false reconciliation alerts.

This is a fail-open default. `DEPLOY.md` tells the operator to set a strong token, but nothing enforces it.

**Recommendation:** Fail closed. Either make `INGEST_TOKEN` `env-required:"true"`, or reject ingest requests when no token is configured (return an error instead of `nil`). Log loudly at startup if ingest auth is disabled.

### C2 — No brute-force protection on Basic auth (Medium)
`core/internal/http/middlewares/main.go:39-49`

The whole API is gated by a single global Basic credential with no rate limiting, throttling, or lockout. Online password guessing is unbounded. The credential compare itself is constant-time (good — SHA-256 + `subtle.ConstantTimeCompare`), but the auth attempt rate is not constrained.

**Recommendation:** Add rate limiting / temporary lockout on failed auth (at the app or reverse-proxy layer, e.g. Traefik middleware / fail2ban). Use a long, high-entropy password.

### C3 — Database TLS disabled by default and in production docs (Medium)
`core/config/config.go:46` (`POSTGRES_SSLMODE env-default:"disable"`), `DEPLOY.md:65` (`POSTGRES_SSLMODE=disable`).

DB credentials and all financial data traverse the network in cleartext. Even on an internal Dokploy network this is weak defense-in-depth; a compromised neighbor or misrouted traffic exposes everything.

**Recommendation:** Default to and document `require` (ideally `verify-full`) for managed Postgres.

### C4 — SSRF via push subscription endpoint (Low)
`core/internal/http/handlers/push.go:16-37`, `core/pkg/webpush/webpush.go:63`

`SubscribePush` stores a client-supplied `endpoint` URL verbatim; the server later issues outbound POSTs to it (`Sender.Send`). An authenticated caller can make core send requests to arbitrary hosts/ports. Impact is limited because it is single-user and authenticated, and the payload is encrypted, but it is a server-side request to an attacker-chosen URL.

**Recommendation:** Validate the endpoint against the known push-service hosts (FCM/Mozilla/Apple) or at least restrict scheme to `https` and block private/loopback ranges.

### C5 — Excel formula injection in export (Low/Info)
`core/internal/http/handlers/transactions_export.go:82`

User-controlled `note` (and category name) are written into `.xlsx` cells. The merchant/note text originates from ingested bank messages, which are externally influenced. `excelize` writes these as string-typed cells (not formulas), so the common risk is mitigated, but a value beginning with `=`, `+`, `-`, or `@` can still be interpreted as a formula by some spreadsheet apps on open.

**Recommendation:** Prefix cells whose value starts with a formula trigger character with a leading apostrophe / zero-width guard before writing.

**Positives:** All SQL uses parameterized queries / placeholders, including the dynamic filter builder in `transaction_repository.List` (no SQL injection). Money is integer minor units. The container runs as non-root (`USER app`, uid 10001). CORS is locked to a single configured origin with no credential wildcard. `chi` `Recoverer` is installed.

---

## lookout

### L1 — Optional ingest token sent over plaintext HTTP (Medium)
`lookout/internal/config/config.go:29` (`FINANCE_API_TOKEN env-default:""`), `lookout/internal/delivery/delivery.go:49-53`, `DEPLOY.md:103-104`.

The bearer header is only attached when the token is non-empty, so an unconfigured token means no auth — which, paired with C1, yields an entirely unauthenticated ingest pipeline. The deployment guide also points lookout at `http://<core>:8080` (plaintext), so the bearer token and financial data are sent in cleartext over the internal network.

**Recommendation:** Require `FINANCE_API_TOKEN`; prefer HTTPS for the core call, or explicitly document and isolate the internal trust boundary.

### L2 — Telegram session stored as plaintext JSON (Low)
`lookout/internal/telegram/telegram.go:68` (`telegram.FileSessionStorage{Path: SessionFile}`)

The gotd session file holds the account auth key in cleartext on the `/data` volume. Anyone with read access to that file gains full control of the linked Telegram account (the account also has 2FA, but the live session bypasses it).

**Recommendation:** Restrict file/volume permissions (0600, dedicated user), and treat the volume as a secret store.

**Positives:** Non-banking SMS / non-matching messages are dropped by the parser before any network use; no secrets are logged; delivery uses bounded exponential backoff; the container runs as non-root (`USER app`). The atomic state writer (`store.go`) is implemented safely (temp file + fsync + rename).

---

## notifier (Mullajiringnotifier — Android)

### N1 — `allowBackup="true"` with default empty backup rules (Low)
`app/src/main/AndroidManifest.xml:14`; `res/xml/backup_rules.xml` and `res/xml/data_extraction_rules.xml` are the unmodified empty templates.

With backups enabled and no include/exclude rules, the app's files — including the `EncryptedSharedPreferences` blob holding the ingest token and base URL — are eligible for cloud / `adb backup` extraction. Impact is reduced because the AES master key lives in the Android Keystore (device-bound, not backed up), so a restored blob cannot be decrypted on another device — but exporting the secret store at all is unnecessary exposure.

**Recommendation:** Set `android:allowBackup="false"`, or explicitly `<exclude>` the `notifier_secure_prefs` shared-prefs file in both backup rule files.

### N2 — No `network_security_config`; endpoint scheme unvalidated (Low)
`app/src/main/AndroidManifest.xml` (no `android:networkSecurityConfig`), `config/AppConfig.kt:16-18`, `net/IngestClient.kt:35-39`

The ingest base URL is free-text user input and is concatenated into the request URL with no scheme check. The platform default (targetSdk 35) blocks cleartext HTTP, which is good, but there is no explicit policy pinning that, and nothing rejects an `http://` URL up front.

**Recommendation:** Add an explicit `network_security_config` that forbids cleartext, and validate that `ingestBaseUrl` begins with `https://` when saved.

**Positives:** Token, URL, and card mapping are held in Keystore-backed `EncryptedSharedPreferences` (`AppConfig.kt`). `SmsReceiver` is exported but guarded by the system-only `android.permission.BROADCAST_SMS`. Idempotency key is SHA-256 of body+sender. OTP/non-matching SMS are dropped per design; no analytics/crash SDKs.

---

## frontend

### F1 — Basic-auth credentials persisted in `localStorage` (Medium)
`frontend/src/api/auth.ts:18-20`

```js
localStorage.setItem(STORAGE_KEY, btoa(`${username}:${password}`))
```

The real username/password are stored base64-encoded (trivially reversible) in `localStorage`: readable by any JavaScript in the origin, persistent across sessions, and equal to the live API password (not a revocable token). Any XSS — or a malicious dependency — yields permanent, full credential theft.

**Recommendation:** At minimum use `sessionStorage`; better, move to a server-issued session token / HttpOnly cookie so the raw password is never retained client-side and can be revoked.

### F2 — Missing security headers in nginx (Medium)
`frontend/nginx.conf`

No `Content-Security-Policy`, `X-Frame-Options` / `frame-ancestors`, `X-Content-Type-Options: nosniff`, or `Referrer-Policy`. Absent a CSP, any injected script runs with full privileges — which directly amplifies F1 (credential exfiltration). Absent frame-ancestors/X-Frame-Options, the app is framable (clickjacking).

**Recommendation:** Add a strict CSP (default-src 'self', no inline scripts), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: no-referrer`.

**Positives:** No XSS sinks found — no `v-html`, `innerHTML`, or `eval` usage; Vue's default output escaping is relied upon. A global 401 handler clears credentials. The production single-origin layout (Traefik HTTPS, relative `/api`) avoids CORS exposure.

---

## Cross-cutting recommendations (priority order)

1. **Fix fail-open ingest auth (C1) and require the token on both sides (L1).** This is the single highest-impact change: today a default deployment has an unauthenticated write path into financial data.
2. **Enforce TLS** for the DB connection (C3) and core↔lookout traffic (L1).
3. **Harden the browser surface:** add security headers (F2) and stop persisting raw credentials (F1).
4. **Add brute-force throttling** on Basic auth (C2).
5. **Tighten the Android secret store exposure** (N1/N2) and the lookout session file (L2).

No SQL injection, secret logging, or server-side XSS sinks were identified. The money core, parameterized data access, non-root containers, and Keystore-backed mobile storage are solid foundations; the gaps are concentrated in *default-open authentication* and *transport/secret-at-rest hygiene*.
