# Installing the Drevanta engine

Use the [M09 localhost-only VPS installer](../../docs/M09-INSTALL.md). The old M01 engine-only installation procedure is obsolete. Do not embed administrator credentials into a systemd unit, expose the engine API publicly, or allow the default shared Caddy admin endpoint at port 2019 to be overwritten.

M09 runs the SvelteKit portal and Go engine as separate systemd services on loopback. The engine is **forced into mock mode** and does not access Docker or Caddy. Live Docker/HTTPS acceptance on a disposable VPS is described in [M08C acceptance](M08C-ACCEPTANCE.md).
