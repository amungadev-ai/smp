# Drevanta — Server Management Platform

Self-hosted, GUI-first VPS hosting console. SvelteKit + TypeScript portal; Go engine planned for real server operations.

Development milestone: native Svelte migration of the v0.16 design reference. UI interactions and sample data are not live server operations.

## Portal

From `frontend/`, use Node.js 22+, `npm install`, `npm run check`, `npm run build`, and `npm run dev`.

Never commit secrets, real credentials, or server state. Do not expose privileged runtime sockets or development endpoints.
