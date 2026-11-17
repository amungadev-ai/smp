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
Dashboard, sidebar, breadcrumbs, five palettes, projects, simulated container releases, logs and rollback, independent domain inventory, server and settings. ZIP analysis, databases, storage, email and other resource modules still require migration, then API integration. No privileged server operations are exposed.

Never commit secrets, real credentials or server state. This is not a production installer.
