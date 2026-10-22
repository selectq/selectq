# HTTP and HTTPS configuration

Add these fields to `auth.config.json`, preserving the Google credentials and allowlists:

```json
{
  "app_origin": "https://YOUR-DOMAIN",
  "tls_enabled": true,
  "tls_cert_file": "/etc/letsencrypt/live/YOUR-DOMAIN/fullchain.pem",
  "tls_key_file": "/etc/letsencrypt/live/YOUR-DOMAIN/privkey.pem"
}
```

This is a configuration fragment, not a replacement for your authentication settings.
The process must have permission to read both files and bind port 443.
`TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables override the JSON paths,
including when using systemd LoadCredential as described in the deployment instructions.

`tls_enabled` defaults to false (HTTP on port 8080). When true, the app serves HTTPS
on port 443 and requires both certificate paths. Optional `listen_address` overrides
the default address, for example `":8443"` or `"127.0.0.1:8080"`.
Relative certificate paths are resolved from the process working directory.

`app_origin` is the browser-facing origin and does not choose the listener protocol.
For direct TLS, use an HTTPS origin and the matching Google redirect URI ending in
`/auth/google/callback`. Behind a TLS proxy, app_origin can be HTTPS while tls_enabled
is false. Direct HTTP traffic and session cookies are unencrypted.

Restart after changing configuration or renewing certificates. The server does not
automatically reload certificates or redirect HTTP to HTTPS. AUTH_CONFIG_FILE can
select a different configuration file. No private keys are included in builds.
