#!/usr/bin/env bash
# Tests real offline snapshot/verify/restore tooling without root/systemd.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
(cd "$ROOT/engine" && go build -o "$tmp/drevanta-maint" ./cmd/drevanta-maint)
mkdir -p "$tmp/state"
printf '{"version":1,"projects":[{"id":"before","name":"Before"}]}\n' >"$tmp/state/state.json"
chmod 0600 "$tmp/state/state.json"
"$tmp/drevanta-maint" backup --state "$tmp/state/state.json" --archive "$tmp/first.zip"
"$tmp/drevanta-maint" verify --archive "$tmp/first.zip"
if "$tmp/drevanta-maint" backup --state "$tmp/state/state.json" --archive "$tmp/first.zip" >/dev/null 2>&1;then
 echo 'Backup overwrite unexpectedly allowed' >&2;exit 1
fi
printf '{"version":1,"projects":[{"id":"after","name":"After"}]}\n' >"$tmp/state/state.json"
if "$tmp/drevanta-maint" restore --state "$tmp/state/state.json" --archive "$tmp/first.zip" >/dev/null 2>&1;then
 echo 'Unconfirmed restoration allowed' >&2;exit 1
fi
"$tmp/drevanta-maint" restore --state "$tmp/state/state.json" --archive "$tmp/first.zip" --confirm-offline
grep -Fq '"id":"before"' "$tmp/state/state.json" || { echo 'Original state not restored' >&2;exit 1; }
grep -rlq '"id":"after"' "$tmp/state"/.drevanta-pre-restore-* || { echo 'Prior state was not retained' >&2;exit 1; }
ln -s "$tmp/first.zip" "$tmp/link.zip"
if "$tmp/drevanta-maint" verify --archive "$tmp/link.zip" >/dev/null 2>&1;then
 echo 'Symlink backup accepted' >&2;exit 1
fi
printf 'not-a-zip' >"$tmp/invalid.zip"
if "$tmp/drevanta-maint" verify --archive "$tmp/invalid.zip" >/dev/null 2>&1;then
 echo 'Corrupt archive accepted' >&2;exit 1
fi
echo 'PASS M10 state backup, checksums, offline restore and recovery copy'
