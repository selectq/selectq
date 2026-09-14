# Select Q website and Google sign-in

The public homepage is `/`. It includes a filterable portfolio, expandable project previews, studio introduction, team placeholders, office address and map link. `/login` is the private workspace entry. Existing accounting URLs remain available after authentication.

## Website content

Edit `frontend/src/app/landing/landing.content.ts` for projects, team roles, email and office address. Introductory marketing copy is in `landing.component.html`. Team names, biographies and portraits are pending; the page does not invent people or completed Select Q projects.

The office address was copied from the existing company profile: House No 189, Sector 31, Near Primary Govt School, Haryana 121003, India. This is a separate public copy. The accounting company-profile API remains private; changes in Settings do not automatically publish online.

The photographs in `frontend/src/assets/portfolio/` were downloaded from [Kay Lang + Associates](https://www.kaylangassocs.com/) at the user's request as temporary references. Visible labels and original watermarks identify the source. Replace these with Select Q photographs before publishing the portfolio as company work. Exact source URLs are in the asset README.

## Configure Google SSO

Create a **Web application** OAuth client in Google Cloud, following [Google's web-server OAuth documentation](https://developers.google.com/identity/protocols/oauth2/web-server). Configure the consent screen and add your account as a test user if the app is in testing mode. The application requests only `openid email profile`.

Register this exact authorized redirect URI for local use:

`http://localhost:8080/auth/google/callback`

For production, register `https://YOUR-DOMAIN/auth/google/callback` and use that HTTPS origin below. Supply environment variables to the Go server, never to Angular or source control:

```powershell
$env:APP_ORIGIN = 'http://localhost:8080'
$env:GOOGLE_CLIENT_ID = '<Google Web application client ID>'
$env:GOOGLE_CLIENT_SECRET = '<Google client secret>'
$env:AUTH_ALLOWED_EMAILS = 'ritugandhi@selectq.in'
go run .
```

Build the frontend first with `npm.cmd run build` in `frontend`, then open `http://localhost:8080`. The server reads environment variables directly; it does not load `.env` files. Restart after changing configuration. For frontend development, run `npm.cmd start` in `frontend`, use `http://localhost:4200` as APP_ORIGIN, and register its `/auth/google/callback` URI. The development proxy forwards `/api` and `/auth` to port 8080.

The default allowed account is **ritugandhi@selectq.in** when `AUTH_ALLOWED_EMAILS` is unset. An explicit comma-separated email list replaces the default. `AUTH_ALLOWED_DOMAINS` optionally allows entire Google Workspace domains; it is empty by default. Domain admission uses Google's verified `hd` claim, not the email suffix. An explicitly empty email list and no domains deny all access.

Without client credentials or any allowed accounts/domains, sign-in shows a setup message and all accounting APIs deny access. There is no development authentication bypass.

## Access controls

- Authorization-code exchange with a random, one-time, ten-minute state bound to an HttpOnly cookie and PKCE S256.
- Identity fetched server-side from [Google's UserInfo endpoint](https://developers.google.com/identity/openid-connect/openid-connect), requiring a subject, verified email and an allowed email or Workspace domain. Google access tokens never reach browser storage.
- Opaque random sessions, HttpOnly cookies, SameSite=Lax, Secure on HTTPS, eight-hour expiry. Sessions are stored in server memory; restarting signs everyone out. Multiple server instances require a shared session/attempt store before deployment.
- Every `/api` handler requires a session, including invoice/statement downloads and DB browser endpoints. Non-GET/HEAD requests also require the configured Origin to prevent cross-site writes. Angular guards improve navigation; the backend enforces access.
- Sign-out uses POST, verifies Origin and revokes the server session. API and session responses use `Cache-Control: no-store`.
- Session expiry on an API call sends the user back to login. Successful Google sign-in opens `/upload`.

Serve production behind HTTPS and proxy `/auth`, `/api`, and the frontend through the single configured APP_ORIGIN. All approved users currently have full existing workspace access; separate roles are not implemented.

## Validation

`go test ./...` covers accounting behavior and authentication. Auth tests use a fake Google HTTP transport to verify success, denied and unverified accounts, token endpoint failure, state replay/expiry, session expiry, API authorization and logout. A real Google sign-in requires configured OAuth credentials and a manual browser check with the approved account.
