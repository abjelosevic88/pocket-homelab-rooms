# Reverse proxy

Set `BASE_URL` to the public address and `SECURE_COOKIES=true` behind HTTPS. See [deploy/reverse-proxy/Caddyfile](../deploy/reverse-proxy/Caddyfile). Pictures (room photos, floor plans to trace) and imports come in one request each: if your proxy limits request bodies, allow at least `MAX_BODY_BYTES` (64 MB by default). `/api/*` is reachable for Homepage widgets, Shortcuts and linked apps; they send their own bearer token. The view-only links (`/s/…`) need no account.
