# Native SvelteKit migration

The v0.16 design reference remains available under `frontend/static/reference/v016.html`. Native modules include projects, domains, databases, file manager, storage spaces, app detection, websites, email, standalone webmail and infrastructure previews. These modules remain simulations unless specifically noted.

The Engine page at `/engine` uses the authenticated Go API. Development Vite proxies `/api/v1` to localhost; **production adapter-node uses `src/hooks.server.ts`**, forwarding only required headers and streaming SSE without leaking cookies. The backend origin must be an explicit loopback HTTP URL (`DRVT_ENGINE_URL=http://127.0.0.1:8080`). Requests are capped at 1 MiB.

The M09 installer runs SvelteKit and Go as loopback-only services; see `docs/M09-INSTALL.md`. It does not enable live Docker or Caddy.
