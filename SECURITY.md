# Security configuration (AcornDrive / FileBrowser fork)

## Secrets — never commit these; supply via environment / Key Vault

| Env var | Purpose |
|---|---|
| `FILEBROWSER_JWT_TOKEN_SECRET` | Signs all session JWTs and encrypts stored Azure tokens. 32+ random bytes. If unset, a random key is generated and persisted on first init. **Set this in prod to control rotation.** |
| `FILEBROWSER_CHAINFS_CLIENT_SECRET` | Azure AD B2C client secret (only for a confidential client). |
| `FILEBROWSER_CHAINFS_ISSUER_URL` | B2C ID-token issuer. When set, ID-token signature + `iss` + `aud` are verified (recommended). Must equal the exact `iss` claim of B2C ID tokens. |
| `FILEBROWSER_ACORN_TOOLS_SECRET` | API key for the acorn.tools internal subscription endpoint. |
| `ACORN_DRIVE_API_SECRET` | Auth for the internal delete-user endpoint (≥16 chars). |
| `FILEBROWSER_ONLYOFFICE_SECRET` / `integrations.onlyoffice.secret` | Verifies OnlyOffice callback JWTs. Required to prevent callback forgery/SSRF. |

Config files (`config*.yaml`) and `acorndrive-prod.yaml` must contain **no** secret values — only `secretRef` / env references.

## Rotation after the leaked-key incident
The previous `auth.key` and B2C `clientSecret` were committed to git history and **must be rotated**:

1. Generate a new key: `openssl rand -base64 32`.
2. Store it in Key Vault and reference it via `FILEBROWSER_JWT_TOKEN_SECRET` (see `acorndrive-prod.yaml` secretRefs).
3. Rotate the B2C client secret in the Azure portal; update `FILEBROWSER_CHAINFS_CLIENT_SECRET`.
4. Rotate `FILEBROWSER_ACORN_TOOLS_SECRET` (shared with acorn.tools) on both sides.
5. Purge the old values from git history (`git filter-repo` / BFG) and force-push — coordinate with the team first.
6. Rotating the JWT key invalidates existing sessions; users simply re-authenticate via B2C.

## Hardening enabled in code
- Path/scope/symlink containment on all file resolution (`SafeScopedJoin`, source-root guard in `GetRealPath`).
- OnlyOffice callback JWT signature verification + outbound-fetch host allowlist (SSRF guard).
- Open-redirect protection on the B2C login `redirect` parameter.
- HSTS + baseline CSP; constant-time secret comparisons; per-share password brute-force throttling; trusted-proxy XFF handling.
