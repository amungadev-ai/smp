#!/usr/bin/env bash
# M10 engine JSON backup/restore wrapper. Restore is OFFLINE-ONLY.
set -Eeuo pipefail
umask 077
ROOT=/var/lib/drevanta-engine
STATE="$ROOT/state.json"
DEST="$ROOT/backups"
BIN=/opt/drevanta/current/engine/drevanta-maint
LOCK=/run/lock/drevanta-m09-install.lock
usage(){
 cat <<'EOF'
Drevanta M10 state maintenance (root/admin operations):
  sudo ./deploy/state.sh backup [filename.zip]
  sudo ./deploy/state.sh verify filename.zip
  sudo ./deploy/state.sh restore filename.zip --confirm-offline

Restoring REQUIRES the Go engine stopped; never restore while running.
Backups contain state.json only, NOT the administrator password.
These are unencrypted: keep them private and copy off the VPS securely.
EOF
}
fail(){ echo "Drevanta state maintenance: $*" >&2;exit 2; }
[[ $# -ge 1 ]] || { usage;exit 2; }
mode="$1";shift
case "$mode" in
 --help|-h|help) usage;exit 0;;
 backup|verify|restore) ;;
 *)fail "Unknown operation $mode";;
esac
[[ $(id -u) -eq 0 ]] || fail 'Run as root for access to the private engine state'
for cmd in runuser systemctl flock curl;do command -v "$cmd" >/dev/null || fail "Missing $cmd";done
[[ -x "$BIN" ]] || fail 'Maintenance binary not installed. Run the audited M10 installer upgrade.'
[[ -f "$STATE" ]] || fail 'Engine state file does not exist.'
exec 9>"$LOCK"
flock -n 9 || fail 'A Drevanta installation or state operation is already running'
install -d -o drevanta -g drevanta -m 0700 "$DEST"
filename=''
case "$mode" in
 backup)
  [[ $# -le 1 ]] || fail 'Backup accepts at most one filename'
  filename="${1:-drevanta-state-$(date -u +%Y%m%dT%H%M%SZ)-$$.zip}"
  ;;
 verify)
  [[ $# -eq 1 ]] || fail 'Verify requires one filename'
  filename="$1"
  ;;
 restore)
  [[ $# -eq 2 && "$2" == --confirm-offline ]] || fail 'Restore requires filename.zip --confirm-offline'
  filename="$1"
  if systemctl is-active --quiet drevanta-engine.service;then fail 'Stop drevanta-engine.service before restore';fi
  if curl -fsS --connect-timeout 1 --max-time 2 http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1;then fail 'Engine still responds on localhost:8080';fi
  ;;
esac
[[ "$filename" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,126}[.]zip$ ]] || fail 'Use a short .zip filename without slashes'
archive="$DEST/$filename"
case "$mode" in
 backup)
  runuser -u drevanta -- "$BIN" backup --state "$STATE" --archive "$archive"
  ;;
 verify)
  runuser -u drevanta -- "$BIN" verify --archive "$archive"
  ;;
 restore)
  runuser -u drevanta -- "$BIN" restore --state "$STATE" --archive "$archive" --confirm-offline
  echo 'State restored. Start the engine explicitly after reviewing recovery output.'
  ;;
esac
