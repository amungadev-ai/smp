#!/usr/bin/env bash
# Run the real Go HTTP API in mock mode and exercise it over localhost.
set -euo pipefail
cd "$(dirname "$0")/.."
for cmd in curl jq; do command -v "$cmd" >/dev/null || exit 2; done
tmp="$(mktemp -d)"
export DRVT_ADMIN_PASSWORD='m08c-ci-lab-password'
cleanup(){ if [[ -n "$pid" ]]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi; rm -rf "$tmp"; }
pid=''
trap cleanup EXIT
./drevanta-engine --mock --addr 127.0.0.1:18080 --data "$tmp/state" --web "$PWD/web" >"$tmp/engine.log" 2>&1 &
pid=$!
for ((i=0;i<50;i++)); do
 if curl -fsS --connect-timeout 1 http://127.0.0.1:18080/api/v1/health >/dev/null 2>&1; then break; fi
 if ! kill -0 "$pid" 2>/dev/null; then cat "$tmp/engine.log" >&2; exit 1; fi
 sleep 0.2
done
DRVT_SMOKE_MODE=mock DRVT_ENGINE_URL=http://127.0.0.1:18080 ./scripts/smoke.sh
