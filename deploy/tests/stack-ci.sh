#!/usr/bin/env bash
# Real production SvelteKit adapter-node -> Go engine, on CI loopback.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
TMP="$(mktemp -d)";GO_PID='';WEB_PID=''
cleanup(){ code=$?;trap - EXIT
 [[ -z "$WEB_PID" ]] || { kill "$WEB_PID" 2>/dev/null || true;wait "$WEB_PID" 2>/dev/null || true; }
 [[ -z "$GO_PID" ]] || { kill "$GO_PID" 2>/dev/null || true;wait "$GO_PID" 2>/dev/null || true; }
 if (( code!=0 ));then echo '--- Engine ---' >&2;cat "$TMP/go.log" >&2 || true;echo '--- Portal ---' >&2;cat "$TMP/node.log" >&2 || true;fi
 rm -rf -- "$TMP";exit "$code"
}
trap cleanup EXIT
cd "$ROOT/engine"
go build -o "$TMP/drevanta-engine" ./cmd/drevanta-engine
DRVT_ADMIN_PASSWORD='m09-disposable-ci-password' "$TMP/drevanta-engine" --mock --addr 127.0.0.1:8080 --data "$TMP/state" --web "$ROOT/engine/web" >"$TMP/go.log" 2>&1 &
GO_PID=$!
for ((i=0;i<40;i++));do
 if curl -fsS --max-time 2 http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1;then break;fi
 kill -0 "$GO_PID" 2>/dev/null || { echo 'Go engine exited' >&2;exit 1; }
 sleep .25
done
cd "$ROOT/frontend"
npm install --no-audit --no-fund
npm test
npm run check
npm run build
env NODE_ENV=production HOST=127.0.0.1 PORT=3000 ORIGIN=http://127.0.0.1:3000 DRVT_ENGINE_URL=http://127.0.0.1:8080 node build/index.js >"$TMP/node.log" 2>&1 &
WEB_PID=$!
for ((i=0;i<50;i++));do
 if "$ROOT/deploy/verify.sh" >/dev/null 2>&1;then break;fi
 kill -0 "$WEB_PID" 2>/dev/null || { echo 'Portal exited' >&2;exit 1; }
 sleep .25
done
"$ROOT/deploy/verify.sh"
login="$(curl --fail-with-body -fsS --max-time 8 -H 'Content-Type: application/json' --data-binary '{"password":"m09-disposable-ci-password"}' http://127.0.0.1:3000/api/v1/auth/login)"
[[ "$login" == *'"token"'* ]] || { echo 'Adapter-node auth proxy failed' >&2;exit 1; }
echo 'PASS M09 production SvelteKit + Go login through same-origin HTTP'
