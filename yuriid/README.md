# YuriID operations

YuriID is a Pocket ID fork (branch `yuriid-branding`) that adds username/email + password sign-in, open
self-registration with email verification, forgot/reset/change password and the Yuri branding. Everything
upstream Pocket ID does (passkeys, OIDC provider, groups, API keys, audit log) still works.

| File | Purpose |
|---|---|
| `docker-compose.prod.yml` | production example: YuriID + PostgreSQL 17, healthchecks, named volumes, Docker secrets |
| `.env.example` | every YuriID-relevant env var with production defaults |
| `CUTOVER.md` | Authentik → YuriID cutover runbook (Forgejo, Kutt, DNS/proxy, rollback, user comms) |

## Configuration (env)
All configuration comes from env / app config. Nothing in the code assumes `localhost`: links in emails, the
OIDC issuer, cookies and CSRF origin all derive from `APP_URL`.

| Variable | Default | Notes |
|---|---|---|
| `APP_URL` | – | Public https URL. It is the OIDC **issuer**; changing it later breaks every client. |
| `TRUST_PROXY` | unset (no proxy trusted) | IPs/CIDRs of reverse proxies whose `X-Forwarded-For` is trusted, e.g. `172.16.0.0/12` for a proxy on the Docker host, or `true` for any. See below. |
| `TRUSTED_PLATFORM` | unset | Header with the real client IP set by a CDN, e.g. `CF-Connecting-IP` behind Cloudflare. |
| `PROXY_PROTOCOL` | unset | Accept PROXY protocol from these IPs/CIDRs (L4 load balancers). |
| `EMAIL_RECIPIENT_ALLOWLIST` | **empty = mail to everyone** | Comma-separated addresses or `@domain` entries. Mail to anyone else is dropped before SMTP. Test/staging only; a non-empty value logs a warning at every start. |
| `OIDC_RELEASE_UNVERIFIED_EMAIL` | `false` | YuriID withholds the `email` claim while the address is unverified (e.g. right after a user changes it), so apps that match accounts by email (Kutt, Forgejo auto-linking) cannot be tricked. |
| `OIDC_GROUPS_IN_PROFILE` | `false` | Also send `groups` for the `profile` scope, like Authentik's default profile mapping. |
| `ENCRYPTION_KEY(_FILE)` | – | Encrypts secrets at rest. **Back it up**; a database backup is useless without it. |
| `DB_PROVIDER` / `DB_CONNECTION_STRING(_FILE)` | sqlite | Use `postgres` in production. |

App config (Admin → Application Configuration, stored in the DB) that matters for YuriID:
`allowUserSignups=open` (public sign-up), SMTP host/port/TLS/user/password/from, `emailVerificationEnabled=true`
(lets users re-verify a changed address), `emailsVerified=false`.

### Real client IP (TRUST_PROXY)
Rate limits (sign-in, sign-up, password email, token endpoints) and audit-log IPs are **per client IP**.
Behind a reverse proxy without `TRUST_PROXY`, every request appears to come from the proxy: one person
hammering the sign-in form would rate-limit everyone. Set `TRUST_PROXY` to exactly the proxy's address
range (never `true` if the container port is reachable by anything other than the proxy) and make the proxy
overwrite (not append to) `X-Forwarded-For`. Verify after deploy: sign in once, then Admin → Audit Log must
show your real IP, not `172.x`.

## Backups
`pocket-id export` writes a consistent snapshot of **every** table, including YuriID's
`yuriid_user_passwords`, `yuriid_auth_tokens` and `yuriid_session_revocations`, plus uploaded images:
```sh
docker compose -f docker-compose.prod.yml exec -T yuriid /app/pocket-id export -p - > yuriid-$(date +%F).zip
chmod 600 yuriid-*.zip      # contains password hashes and encrypted secrets
```
Restore (stops the running instance's lease; needs the **same** `ENCRYPTION_KEY`):
```sh
docker compose -f docker-compose.prod.yml stop yuriid
docker compose -f docker-compose.prod.yml run --rm -T yuriid /app/pocket-id import -p - -y < yuriid-YYYY-MM-DD.zip
docker compose -f docker-compose.prod.yml up -d
```
Verified locally (2026-10-10): an export of the SQLite dev instance imported into the Postgres example
(15 users, 15 password rows, 2 clients, 3 groups); password sign-in and reset worked there, and its own
export again contained the `yuriid_*` tables. Additionally keep `pg_dump -Fc` of the database and a copy of
`secrets/encryption_key` off-host.

## Security notes (password auth)
* Hashes: argon2id (m=64 MiB, t=3, p=2). Imported Django/Authentik hashes (pbkdf2, bcrypt, argon2) are
  verified and upgraded to argon2id on first sign-in.
* Sign-in / sign-up / forgot / resend never reveal whether an account exists (same responses; a dummy hash
  is verified for unknown users; existing-account cases are told to the address owner by email instead).
* Email tokens: 256-bit random, only the SHA-256 is stored, single-use, 24 h (verify) / 1 h (reset); issuing a
  new one invalidates the previous.
* A password **reset signs the user out everywhere**: YuriID sessions issued before the reset are rejected and
  all OIDC refresh/access tokens of the user's apps are revoked. (Changing the password while signed in does
  not end other sessions.)
* CSRF: every state-changing request goes through Go's `http.CrossOriginProtection` (Sec-Fetch-Site/Origin),
  with `APP_URL` as the trusted origin; cookies are `Secure` + `SameSite=Lax`.
* Post-login `?redirect=` only accepts same-origin paths (`/…`), never `//host`, `/\host` or absolute URLs.
* Email headers: recipient/subject/display names are rejected if they contain CR/LF and are RFC 2047 encoded
  by the mailer; usernames are restricted to `[a-zA-Z0-9_.@-]`, emails are validated.
