# M10 — State recovery and session hardening

M10 extends the mock-only installer. **No live VPS or public hosting has been validated.**

## Authentication

The Go engine now issues unpredictable, 256-bit, one-time bearer values. Only SHA-256 token digests are retained in server memory. Tokens expire after eight hours; at most 128 active sessions are retained. `POST /api/v1/auth/logout` revokes the presented token immediately. Engine restart invalidates every session. Browser log streams still use authorization headers instead of query-string tokens.

This does **not** add multi-user login, permissions, CSRF-safe cookie sessions, MFA, public HTTPS or production account recovery. The portal stays loopback-only, accessed via an SSH tunnel.

## Verified engine state backups

A new `drevanta-maint` command and `deploy/state.sh` wrapper provide bounded (16 MiB), checksummed ZIP snapshots of `state.json` and guarded offline restoration. A backup has two entries: `state.json` and `manifest.json` containing the version, size and SHA-256 hash.

The administrator password, runtime containers, web assets, mailboxes and database storage are **not** included. A SHA-256 checksum detects corruption, but does **not authenticate the origin** of a backup. ZIP files are **not encrypted**; keep the private 0600 backups in a restricted directory, copy off-server securely and arrange encrypted storage independently.

From the audited repository on the **test VPS**, after upgrading to M10:

```bash
sudo ./deploy/state.sh backup
sudo ./deploy/state.sh verify drevanta-state-EXAMPLE.zip
```

The backup command prints the created filename. Replace the example name with that filename for verification.

For restoration, ensure you have a second copy of the current state and stop the Go engine:

```bash
sudo systemctl stop drevanta-engine.service
sudo ./deploy/state.sh restore drevanta-state-EXAMPLE.zip --confirm-offline
sudo systemctl start drevanta-engine.service
./deploy/install.sh --verify
```

The wrapper refuses restoration while the engine service is active or while the local engine health endpoint responds. It executes maintenance as the `drevanta` runtime user, preserving owner-only state permissions. A private `.drevanta-pre-restore-*` copy of the replaced state is retained for recovery. The wrapper uses the same installer lock to avoid concurrent installer operations; it does not provide a substitute for freezing all externally initiated writes.

Restoration is intentionally **state.json only**, and an offline operation. It cannot recover running containers, Caddy certificates, volume data or secrets. Production backup and recovery must cover those separately.

## Validation

GitHub Actions runs Go unit/race tests, frontend checks and a command-line backup/verify/restore smoke test. The snapshot tests include corruption detection, unsafe archive entries, oversize rejection, original state preservation and no overwrite of existing backup files. Real VPS restoration after service restart still needs a disposable test host.

See [M09 installation](M09-INSTALL.md) and [M08C live acceptance](../engine/docs/M08C-ACCEPTANCE.md).

## Disposable-VPS preflight (read-only)

M10 also includes `deploy/preflight.sh`, which checks Linux/Go/Node prerequisites and, with explicit disposable-VPS acknowledgement, checks Docker availability, DNS A record targeting and the **empty dedicated Caddy admin instance** at 127.0.0.1:2021. It never calls Caddy `/load`, starts containers, requests certificates or enables live mode.

```bash
./deploy/preflight.sh --plan
./deploy/preflight.sh --check-local
# On a disposable VPS only, after configuring dedicated Caddy and DNS:
./deploy/preflight.sh --check-live --domain app.test-example.com \\
  --expected-ip YOUR_PUBLIC_IPV4 --confirm-disposable
```

Passing preflight does not establish public HTTPS, zero downtime, real rollback or production readiness. Complete [M08C acceptance](../engine/docs/M08C-ACCEPTANCE.md) on a disposable VPS and preserve test logs before reconsidering live mode.
