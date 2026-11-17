# Drevanta — Server Management Platform

Self-hosted GUI-first hosting console. SvelteKit + TypeScript portal. Go engine integration is a separate milestone.

## Development
From `frontend`, with Node.js 22+:
```sh
npm install
npm run check
npm run build
npm run dev
```
Open http://localhost:5173. The native portal is a development preview; data and deployments are simulated. The original v0.16 prototype is available at /reference/v016.html and is the design reference for pending modules.

## Scope
Dashboard, sidebar, breadcrumbs, five palettes, projects, simulated container releases, logs and rollback, independent domain inventory, server and settings. Native database management is included (simulated). Native storage and file management are included (browser samples). Browser ZIP inspection and application detection are included. Native email management and standalone webmail are included (browser samples). Native website hosting and infrastructure workflows are included as simulations; real Git, cron, services, certificates, backups and VPS operations still require the Go engine. No privileged server operations are exposed.

Never commit secrets, real credentials or server state. This is not a production installer.

Run `npm test` from `frontend/` for database, file state, application detection, mail state, website and infrastructure tests. See `frontend/MIGRATION.md` for scope.
