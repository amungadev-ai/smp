#!/usr/bin/env bash
# Verifies that the mock engine and SvelteKit portal are alive and same-origin.
set -euo pipefail
ENGINE_URL="http://127.0.0.1:8080"
PORTAL_URL="http://127.0.0.1:3000"
fail(){ printf 'M09 verification failed: %s\n' "$*" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || fail "curl is required"
api="$(curl --fail-with-body -fsS --connect-timeout 3 --max-time 8 "$ENGINE_URL/api/v1/health")" || fail "Go engine unreachable"
[[ "$api" == *'"ok":true'* && "$api" == *'"mock_mode":true'* ]] || fail "engine is not in safe mock mode"
curl --fail-with-body -fsS --connect-timeout 3 --max-time 8 -o /dev/null "$PORTAL_URL/engine" || fail "SvelteKit /engine route unreachable"
proxied="$(curl --fail-with-body -fsS --connect-timeout 3 --max-time 8 "$PORTAL_URL/api/v1/health")" || fail "SvelteKit production API proxy unavailable"
[[ "$proxied" == *'"ok":true'* && "$proxied" == *'"mock_mode":true'* ]] || fail "portal did not reach a mock engine"
code="$(curl -sS --connect-timeout 3 --max-time 8 -o /dev/null -w '%{http_code}' "$PORTAL_URL/api/v1/server")" || fail "auth probe failed"
[[ "$code" == 401 ]] || fail "protected engine API did not return HTTP 401 through portal (got $code)"
echo 'PASS M09: mock Go API, SvelteKit /engine, same-origin proxy and authentication boundary'
