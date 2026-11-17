#!/usr/bin/env bash
# Drevanta M09 installer — mock-only and loopback-only by construction.
# Run from an audited checkout. No automatic apt installation or public ports.
set -Eeuo pipefail
umask 077
SOURCE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
ROOT=/opt/drevanta
RELEASES="$ROOT/releases"
STATE=/var/lib/drevanta-engine
SERVICE_USER=drevanta
PORTAL_USER=drevanta-portal
BUILD_USER=drevanta-build
UNITS=/etc/systemd/system
MODE=''
ACK=0
OLD=''
STAGE=''
PROMOTED=0
RELEASE=''
CURRENT_TARGET=''
say(){ printf '\n==> %s\n' "$*"; }
die(){ printf 'ERROR: %s\n' "$*" >&2; exit 1; }
usage(){
 cat <<'EOF'
Drevanta M09 — preview installer
  --plan        Show installation plan (no root, no changes)
  --install     First mock-only localhost installation
  --upgrade     Build and activate a new preview release
  --rollback    Switch to previously installed release
  --verify      Verify installed mock engine + portal
  --accept-mock-only  Required with --install or --upgrade

The installer never enables Docker or Caddy live mode.
The portal is available only on 127.0.0.1:3000 (use an SSH tunnel).
EOF
}
while (($#)); do
 case "$1" in
  --plan|--install|--upgrade|--rollback|--verify) [[ -z "$MODE" ]] || die 'Choose exactly one operation';MODE="$1";;
  --accept-mock-only)ACK=1;;
  --help|-h)usage;exit 0;;
  *)die "Unknown option: $1";;
 esac
 shift
done
[[ -n "$MODE" ]] || { usage;exit 2; }
if [[ "$MODE" == --plan ]]; then
 cat <<EOF
M09 installation plan — no changes made
Source: $SOURCE
Release root: $RELEASES
Engine state: $STATE (preserved across upgrades)
Services: drevanta-engine.service (127.0.0.1:8080, --mock)
          drevanta-portal.service (127.0.0.1:3000, production API proxy)
Builds: Go 1.22+, Node.js 22+, npm
Service users: engine=$SERVICE_USER, portal=$PORTAL_USER (no Docker group)
No Caddy changes, no firewall changes, no public ports, no database server.
EOF
 exit 0
fi
if [[ "$MODE" == --verify ]]; then exec "$SOURCE/deploy/verify.sh"; fi
[[ $(id -u) -eq 0 ]] || die 'Installation, upgrade and rollback require root'
exec 9>/run/lock/drevanta-m09-install.lock
flock -n 9 || die 'Another Drevanta installer is running'
command -v systemctl >/dev/null || die 'systemd required (Ubuntu/Debian VM)'
[[ "$(uname -s)" == Linux ]] || die 'Linux required'
case "$(uname -m)" in x86_64|aarch64) ;; *)die 'Only x86_64 and aarch64 are supported for now';; esac
for unit in drevanta-engine.service drevanta-portal.service;do
 if [[ -f "$UNITS/$unit" ]] && ! grep -q 'Managed by Drevanta M09 installer' "$UNITS/$unit";then
  die "$UNITS/$unit was not installed by M09; refusing to replace a pre-existing service"
 fi
done
[[ ! -e "$ROOT/current" || -L "$ROOT/current" ]] || die 'Existing current path is not a symlink'
[[ ! -e "$ROOT/previous" || -L "$ROOT/previous" ]] || die 'Existing previous path is not a symlink'
if [[ -L "$ROOT/current" ]]; then
 CURRENT_TARGET="$(readlink "$ROOT/current")"
 [[ "$CURRENT_TARGET" == releases/* && -d "$ROOT/$CURRENT_TARGET" ]] || die 'Current release symlink is unsafe or missing'
fi
if [[ "$MODE" == --rollback ]]; then
 [[ -L "$ROOT/previous" ]] || die 'No previous release is available'
 candidate="$(readlink "$ROOT/previous")"
 [[ "$candidate" == releases/* && -d "$ROOT/$candidate" ]] || die 'Previous release symlink is unsafe or missing'
 [[ -n "$CURRENT_TARGET" ]] || die 'Current release missing'
 say "Rolling back to $candidate"
 ln -s "$candidate" "$ROOT/current.next"
 mv -Tf "$ROOT/current.next" "$ROOT/current"
 systemctl restart drevanta-engine.service drevanta-portal.service
 if ! "$SOURCE/deploy/verify.sh";then
  ln -s "$CURRENT_TARGET" "$ROOT/current.next"
  mv -Tf "$ROOT/current.next" "$ROOT/current"
  systemctl restart drevanta-engine.service drevanta-portal.service || true
  die 'Rollback health check failed; original release restored'
 fi
 ln -s "$CURRENT_TARGET" "$ROOT/previous.next"
 mv -Tf "$ROOT/previous.next" "$ROOT/previous"
 say 'Previous release restored and health-checked'
 exit 0
fi
[[ "$ACK" == 1 ]] || die 'Explicit --accept-mock-only required'
if [[ "$MODE" == --install && -n "$CURRENT_TARGET" ]];then die 'Existing install detected; use --upgrade';fi
if [[ "$MODE" == --upgrade && -z "$CURRENT_TARGET" ]];then die 'Nothing to upgrade; use --install';fi
for cmd in go node npm curl runuser tar mktemp mv ln flock;do
 command -v "$cmd" >/dev/null || die "Missing command: $cmd"
done
node -e 'process.exit(Number(process.versions.node.split(".")[0])>=22?0:1)' || die 'Node.js >=22 required'
govers="$(go env GOVERSION)"
if [[ "$govers" =~ ^go([0-9]+)\.([0-9]+) ]];then
 major="${BASH_REMATCH[1]}";minor="${BASH_REMATCH[2]}"
 (( major > 1 || (major == 1 && minor >= 22) )) || die 'Go >=1.22 required'
else die 'Could not determine Go version';fi
[[ -f "$SOURCE/frontend/package.json" && -f "$SOURCE/engine/go.mod" ]] || die 'Run from a complete Drevanta repository checkout'
if ! id "$SERVICE_USER" >/dev/null 2>&1;then
 useradd --system --home-dir "$STATE" --shell /usr/sbin/nologin "$SERVICE_USER"
fi
if ! id "$PORTAL_USER" >/dev/null 2>&1;then
 useradd --system --home-dir /var/lib/drevanta-portal --shell /usr/sbin/nologin "$PORTAL_USER"
fi
if ! id "$BUILD_USER" >/dev/null 2>&1;then
 useradd --system --home-dir /var/lib/drevanta-build --shell /usr/sbin/nologin "$BUILD_USER"
fi
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0700 "$STATE"
install -d -o "$BUILD_USER" -g "$BUILD_USER" -m 0700 /var/lib/drevanta-build
install -d -o "$PORTAL_USER" -g "$PORTAL_USER" -m 0700 /var/lib/drevanta-portal
install -d -o root -g root -m 0755 "$ROOT" "$RELEASES" "$ROOT/staging"
STAGE="$(mktemp -d "$ROOT/staging/.build.XXXXXXXX")"
chmod 0711 "$STAGE"
OLD="$CURRENT_TARGET"
cleanup(){
 code=$?
 trap - EXIT
 if (( code != 0 && PROMOTED == 1 ));then
  say 'Activation failed; restoring previous service units and release'
  set +e
  if [[ -n "$OLD" ]];then
   ln -s "$OLD" "$ROOT/current.next" && mv -Tf "$ROOT/current.next" "$ROOT/current"
  else
   rm -f "$ROOT/current"
  fi
  for unit in drevanta-engine.service drevanta-portal.service;do
   if [[ -f "$STAGE/backup-$unit" ]];then
    cp "$STAGE/backup-$unit" "$UNITS/$unit"
   else
    rm -f "$UNITS/$unit"
   fi
  done
  systemctl daemon-reload
  if [[ -n "$OLD" ]];then
   systemctl restart drevanta-engine.service drevanta-portal.service
  else
   systemctl disable --now drevanta-portal.service drevanta-engine.service
  fi
  set -e
 fi
 [[ -z "$STAGE" || ! -d "$STAGE" ]] || rm -rf -- "$STAGE"
 exit "$code"
}
trap cleanup EXIT
install -d -o "$BUILD_USER" -g "$BUILD_USER" -m 0750 "$STAGE/app" "$STAGE/app/engine" "$STAGE/app/portal"
say 'Staging audited sources'
tar -C "$SOURCE/engine" --exclude=./data --exclude=./drevanta-engine --exclude='.env' --exclude='.env.*' -cf - . | tar -C "$STAGE/app/engine" -xf -
tar -C "$SOURCE/frontend" --exclude=./node_modules --exclude=./build --exclude=./.svelte-kit --exclude='.env' --exclude='.env.*' -cf - . | tar -C "$STAGE/app/portal" -xf -
chown -R "$BUILD_USER:$BUILD_USER" "$STAGE/app"
say 'Building and testing Go as an unprivileged user'
runuser -u "$BUILD_USER" -- env PATH="$PATH" bash -e -c 'cd "$1"; go test ./...; go vet ./...; go build -o drevanta-engine ./cmd/drevanta-engine' _ "$STAGE/app/engine"
say 'Building and testing SvelteKit as an unprivileged user'
runuser -u "$BUILD_USER" -- env PATH="$PATH" bash -e -c 'cd "$1"; npm install --no-audit --no-fund; npm test; npm run check; npm run build' _ "$STAGE/app/portal"
[[ -f "$STAGE/app/portal/build/index.js" && -f "$STAGE/app/engine/drevanta-engine" ]] || die 'Build output missing'
RELEASE="$(date -u +%Y%m%dT%H%M%SZ)-$$"
say "Activating release $RELEASE"
mv "$STAGE/app" "$RELEASES/$RELEASE"
chown -R root:root "$RELEASES/$RELEASE"
chmod -R a+rX "$RELEASES/$RELEASE"
for unit in drevanta-engine.service drevanta-portal.service;do
 if [[ -f "$UNITS/$unit" ]];then cp "$UNITS/$unit" "$STAGE/backup-$unit";fi
done
ln -s "releases/$RELEASE" "$ROOT/current.next"
mv -Tf "$ROOT/current.next" "$ROOT/current"
PROMOTED=1
for unit in drevanta-engine.service drevanta-portal.service;do
 install -o root -g root -m 0644 "$SOURCE/deploy/systemd/$unit" "$UNITS/$unit"
done
systemctl daemon-reload
systemctl enable drevanta-engine.service drevanta-portal.service
systemctl restart drevanta-engine.service drevanta-portal.service
say 'Verifying local-only mock installation'
healthy=0
for ((i=0;i<30;i++));do
 if "$SOURCE/deploy/verify.sh" >/dev/null 2>&1;then healthy=1;break;fi
 sleep 1
done
[[ "$healthy" == 1 ]] || die 'Both services must pass the health and API boundary checks'
"$SOURCE/deploy/verify.sh"
if [[ -n "$OLD" ]];then
 ln -s "$OLD" "$ROOT/previous.next"
 mv -Tf "$ROOT/previous.next" "$ROOT/previous"
fi
PROMOTED=0
say 'Drevanta mock installer completed (localhost only)'
printf 'Release: %s\nEngine credentials: %s/admin-password (0600; do not disclose)\n' "$RELEASE" "$STATE"
printf 'Access remotely with: ssh -L 3000:127.0.0.1:3000 user@your-vps\nThen open http://127.0.0.1:3000/engine\n'
