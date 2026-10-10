# Authentik → YuriID cutover runbook

Scope: replace Authentik as the IdP for **Forgejo** (`git.yuri.llc`) and **Kutt** (`l.yuri.llc`).
Tooling lives on the Mac: `~/Projects/authentik-export` (dump + restore), `~/Projects/yuriid-migrate`
(`migrate.py`, outputs in `output/`, all secret files mode 600). Authentik stays **untouched and running**
until step 9, which is what makes every step reversible.

Placeholders: `<AK>` = Authentik's public host, `id.yuri.llc` = YuriID's public host (any name works; it
becomes the OIDC issuer and must not change later).

## 0. Decisions / prerequisites (days before)
- [ ] Host for YuriID (e.g. the Authentik VM). Deploy with `yuriid/docker-compose.prod.yml` (Postgres), image
      built on that host from a pinned commit of `hazelcaffe/pocket-id@yuriid-branding`.
- [ ] DNS: `id.yuri.llc` → reverse proxy (TTL 300 a day ahead). TLS via the proxy.
- [ ] Reverse proxy vhost, e.g. Caddy:
      ```
      id.yuri.llc {
          reverse_proxy 127.0.0.1:1411
      }
      ```
      nginx: `proxy_set_header X-Forwarded-For $remote_addr; proxy_set_header X-Forwarded-Proto https; proxy_set_header Host $host;`
- [ ] `.env`: `APP_URL=https://id.yuri.llc`, `TRUST_PROXY=<docker bridge range, e.g. 172.16.0.0/12>`,
      `OIDC_GROUPS_IN_PROFILE=true` (Authentik parity), `EMAIL_RECIPIENT_ALLOWLIST=hazel@yuri.llc,noc@yuri.llc`
      **for now** (it is removed in step 6).
- [ ] SMTP: Authentik reaches `mail.yuri.llc` through a private address (`extra_hosts` in its compose). Add the
      same `extra_hosts` entry to the `yuriid` service, or mail will fail.
- [ ] Back up `secrets/encryption_key` off-host (a DB backup is useless without it).
- [ ] Admin → Application Configuration: SMTP (from `noreply@yuri.llc`), `allowUserSignups=open`,
      `emailVerificationEnabled=true`, app name/branding/background (+ the illustration, see note at the end).
- [ ] Create the first admin with the setup page, then an **admin API key** for `migrate.py`.

## 1. Rehearsal on the new host (T-2 days)
Run steps 3–5 against the new instance with the allowlist still set, test both apps against a **copy**
(or skip app tests), then wipe the YuriID DB (`docker compose down -v`) so the real run starts clean.

## 2. User comms (T-2 days, and again at cutover)
> **Subject: Yuri ID is moving on <date>**
> On <date> around <time> ET, sign-in for git.yuri.llc and l.yuri.llc moves to the new Yuri ID at
> https://id.yuri.llc. Your username, email and **password stay the same**. Passkeys and authenticator-app
> codes from the old system can't be moved; after signing in you can add a new passkey under *Account*.
> You'll be signed out of both apps once and just sign in again. Problems: reply to this email / noc@yuri.llc.

Send it from your own mail client (YuriID itself is still allowlisted then).

## 3. Fresh export (cutover day, T-0) — Authentik read-only
```sh
cd ~/Projects/authentik-export
ssh debian@<authentik-host> 'cd /srv/docker/authentik && docker compose exec -T postgresql pg_dump -U authentik -d authentik -Fc' > authentik.dump
chmod 600 authentik.dump
docker rm -f ak-export-pg; docker run -d --name ak-export-pg -e POSTGRES_USER=authentik -e POSTGRES_DB=authentik \
  -e POSTGRES_PASSWORD=local -p 127.0.0.1:55432:5432 postgres:16-alpine
sleep 5; docker exec -i ak-export-pg pg_restore -U authentik -d authentik --no-owner < authentik.dump
python3 export_from_dump.py
```
From here on, ask users not to change passwords in Authentik (a later change would not be carried over).

## 4. Migrate into YuriID
```sh
cd ~/Projects/yuriid-migrate
export YURIID_URL=https://id.yuri.llc
python3 migrate.py --api-key-file <file with the prod admin API key> --dry-run
python3 migrate.py --api-key-file <…>        # users, groups, passwords, avatars, clients, logos, launch URLs
python3 migrate.py --api-key-file <…> --dry-run   # must report "api_writes": 0
```
Outputs (mode 600, keep on the Mac): `client-secrets.json` (new secret per app), `sub-mapping.csv`,
`forgejo-relink.sql` + `forgejo-relink-rollback.sql`, `login-codes.csv` (only users without a usable password).
The migration never sends email. Check `last-run-summary.json`: users, `passwords_imported`, skipped users.

## 5. Smoke test YuriID (still allowlisted)
- [ ] `https://id.yuri.llc/.well-known/openid-configuration` → `issuer` is `https://id.yuri.llc`
- [ ] Sign in as yourself with your Authentik password; Admin → Audit Log shows your **real IP** (TRUST_PROXY ok)
- [ ] Forgot password for hazel@yuri.llc arrives from noreply@yuri.llc; reset works and signs other sessions out
- [ ] *My Apps* shows Forgejo and Kutt with logos

## 6. Open mail to everyone
Remove `EMAIL_RECIPIENT_ALLOWLIST` from `.env` (or set it empty), `docker compose up -d`. The startup log must
**no longer** contain `EMAIL_RECIPIENT_ALLOWLIST is active`.

## 7. Switch Forgejo (`git.yuri.llc`)
Why the SQL: Authentik sent `sub = sha256("<pk>-<install id>")` (sub_mode `hashed_user_id`); YuriID sends the
user UUID. Forgejo stores the old `sub` in `external_login_user.external_id` and `user.login_name` for the
"Yuri ID" auth source; without the remap every existing user would look new.
1. Back up Forgejo (`forgejo dump` or a DB dump).
2. Pre-check that the mapping matches what Forgejo has stored (expect one row per linked user):
   ```sql
   SELECT u.name, e.external_id FROM external_login_user e JOIN "user" u ON u.id = e.user_id
   WHERE e.login_source_id = (SELECT id FROM login_source WHERE name = 'Yuri ID');
   ```
   Every `external_id` should appear in `sub-mapping.csv` → `authentik_sub` for the same username. If not,
   stop and use option B below.
3. Option A (recommended): run `output/forgejo-relink.sql` (one transaction) on the Forgejo DB.
   Option B (fallback): leave links alone and let users re-link: `[oauth2_client] ACCOUNT_LINKING = login`
   (default) makes Forgejo ask for the existing Forgejo password once. Avoid `auto` (links by email).
4. Site Administration → Authentication Sources → **Yuri ID** (keep the name, so the callback
   `https://git.yuri.llc/user/oauth2/Yuri%20ID/callback` stays valid):
   - OpenID Connect Auto Discovery URL: `https://id.yuri.llc/.well-known/openid-configuration`
   - Client ID: unchanged (`forgejo-03ee…`); Client Secret: from `client-secrets.json`
   - Additional scopes: `email profile` (add `groups` if you map groups). If you use *Group claim name* /
     admin or restricted groups: YuriID sends group **names (slugs)**, e.g. `authentik_admins`
     instead of Authentik's `authentik Admins` — update the values.
5. Test in a private window with a non-admin account and with your own: sign in lands in the **existing**
   Forgejo account (same repos), not a new one / not the "link account" page.

## 8. Switch Kutt (`l.yuri.llc`)
Kutt matches users by the `email` claim (no `sub` link), so existing accounts carry over as long as the email
is the same and verified (YuriID withholds unverified emails).
1. Kutt env: `OIDC_ISSUER=https://id.yuri.llc`, `OIDC_CLIENT_SECRET=<from client-secrets.json>`
   (client id unchanged, `OIDC_SCOPE="openid profile email"`, `OIDC_EMAIL_CLAIM=email`); restart Kutt.
2. Test: sign in → your existing links are there.

## 9. After both apps work
- [ ] Send the "done" note to users. Watch YuriID logs / audit log for a few days.
- [ ] Rotate nothing else needed: YuriID issued new client secrets; Authentik's old ones die with Authentik.
- [ ] Schedule backups (`pocket-id export` daily + `pg_dump`, copy off-host with the encryption key).
- [ ] After ~2 weeks without rollback: stop Authentik (keep the last dump + `.env` encrypted somewhere),
      remove its DNS name, then decommission.

## Rollback
| Failed at | Do |
|---|---|
| Steps 3–6 | Nothing user-visible changed. Fix and re-run (`migrate.py` is idempotent), or wipe YuriID's DB and start over. |
| Step 7 (Forgejo) | Run `forgejo-relink-rollback.sql`, point the "Yuri ID" source back at `https://<AK>/application/o/forgejo/.well-known/openid-configuration` with the **old** Authentik secret (still in Authentik). |
| Step 8 (Kutt) | Restore the previous `OIDC_ISSUER`/`OIDC_CLIENT_SECRET`, restart Kutt. |
| After users changed passwords in YuriID | Rolling back means those users must use their **old** Authentik password (or Authentik's recovery). Tell them. |

## What YuriID does not replace (known gaps)
- **Passkeys/WebAuthn (2) and TOTP (1)** from Authentik cannot be moved; users re-enroll passkeys. YuriID has
  **no TOTP/2FA for password sign-in** — password accounts are single-factor unless the user switches to a
  passkey. Authentik's TOTP user loses that second factor.
- **Policies/flows/stages, expression policies, enrollment/recovery flow customisation, outposts / forward
  auth (proxy provider), LDAP/SAML/RADIUS providers, Authentik "tokens" (9)** — none are migrated or provided.
  Only the two OIDC apps are in scope, and neither needs them.
- **Groups claim** values are slugs (`authentik_admins`), not Authentik's display names.
- **Logout**: Authentik had back-channel logout configured without a URL (inactive). YuriID supports RP-initiated
  logout (`end_session_endpoint`) and back-channel logout per client if the apps are configured for it;
  Forgejo/Kutt don't use either, so signing out of YuriID does not sign you out of the apps (same as before).
- **Changing your password** while signed in does not end other sessions (a **reset** does).
- Login illustration: make sure you have the rights to the artwork used as the login background.
