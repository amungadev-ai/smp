#!/usr/bin/env bash
# Drevanta M08C acceptance script. Lab-only; live mode requires explicit opt-in.
set -euo pipefail
for t in curl jq; do command -v "$t" >/dev/null || { echo "Missing $t" >&2; exit 2; }; done
BASE="${DRVT_ENGINE_URL:-http://127.0.0.1:8080}"
MODE="${DRVT_SMOKE_MODE:-mock}"
PASS="${DRVT_ADMIN_PASSWORD:?Set DRVT_ADMIN_PASSWORD for your test engine}"
DOMAIN="${DRVT_TEST_DOMAIN:-}"
case "$BASE" in http://127.0.0.1:*|http://localhost:*) ;; *) echo "Engine API must be loopback-only" >&2; exit 2;; esac
if [[ "$MODE" == live ]]; then
 [[ "${DRVT_SMOKE_CONFIRM:-}" == "I_HAVE_A_DISPOSABLE_VPS" ]] || { echo "Explicit disposable VPS confirmation required" >&2; exit 2; }
 [[ "$DOMAIN" =~ ^[a-z0-9][a-z0-9.-]+[.][a-z]{2,24}$ ]] || { echo "Set DRVT_TEST_DOMAIN to a disposable DNS hostname" >&2; exit 2; }
elif [[ "$MODE" != mock ]]; then echo "DRVT_SMOKE_MODE must be mock or live" >&2; exit 2; fi
call(){ local method="$1" path="$2"; shift 2; curl --fail-with-body -sS --connect-timeout 5 --max-time 20 -X "$method" "$BASE/api/v1$path" "$@"; }
health="$(call GET /health)"
[[ "$(jq -r '.ok' <<<"$health")" == true ]] || { echo "Engine unhealthy" >&2; exit 1; }
mode="$(jq -r '.mock_mode' <<<"$health")"
if [[ "$MODE" == live && "$mode" != false || "$MODE" == mock && "$mode" != true ]]; then echo "Engine mode mismatch ($mode)" >&2; exit 1; fi
echo "PASS engine health ($MODE)"
login="$(call POST /auth/login -H 'Content-Type: application/json' --data-binary "$(jq -nc --arg password "$PASS" '{password:$password}')")"
TOKEN="$(jq -er '.token' <<<"$login")"
auth(){ local method="$1" path="$2"; shift 2; call "$method" "$path" -H "Authorization: Bearer $TOKEN" "$@"; }
auth GET /server/readiness | jq -e '.checks | type=="array"' >/dev/null
echo "PASS authenticated readiness"
if [[ "$MODE" == live ]]; then
 count="$(auth GET /projects | jq -r '.projects | length')"
 [[ "$count" == 0 ]] || { echo "Refusing live smoke on host with $count existing projects" >&2; exit 2; }
fi
NAME="drvt-smoke-$(date +%s)-$RANDOM"
NAME="${NAME:0:49}"
A="${DRVT_SMOKE_IMAGE_A:-nginx:alpine}"
B="${DRVT_SMOKE_IMAGE_B:-nginx:stable-alpine}"
if [[ "$MODE" == live ]]; then
 payload="$(jq -nc --arg name "$NAME" --arg image "$A" --arg domain "$DOMAIN" '{name:$name,image:$image,domain:$domain,port:80,https:true}')"
else
 payload="$(jq -nc --arg name "$NAME" --arg image "$A" '{name:$name,image:$image,port:80,https:false}')"
fi
start="$(auth POST /projects/deploy -H 'Content-Type: application/json' --data-binary "$payload")"
PROJECT="$(jq -er '.project.id' <<<"$start")"
wait_job(){ local id="$1" expect="$2" response status i
 for ((i=0;i<120;i++)); do
  response="$(auth GET "/jobs/$id")"; status="$(jq -r '.status' <<<"$response")"
  if [[ "$status" == succeeded || "$status" == failed ]]; then
   [[ "$status" == "$expect" ]] || { echo "Unexpected job outcome: $response" >&2; exit 1; }
   return
  fi
  sleep 1
 done
 echo "Job $id did not complete within the smoke-test budget" >&2; exit 1
}
wait_job "$(jq -er '.job.id' <<<"$start")" succeeded
first="$(auth GET "/projects/$PROJECT")"
ORIGINAL="$(jq -er '.current_release' <<<"$first")"
[[ "$(jq -r '.status' <<<"$first")" == running && "$(jq -r '.image' <<<"$first")" == "$A" ]] || { echo "First release is not active" >&2; exit 1; }
echo "PASS first release $ORIGINAL"
https_check(){ if [[ "$MODE" == live ]]; then
 curl --proto '=https' --tlsv1.2 -fsS --max-time 25 "https://$DOMAIN/" >/dev/null
 echo "PASS public HTTPS (certificate + content)"
fi; }
https_check
next="$(auth POST "/projects/$PROJECT/deploy" -H 'Content-Type: application/json' --data-binary "$(jq -nc --arg image "$B" '{image:$image}')")"
wait_job "$(jq -er '.job.id' <<<"$next")" succeeded
second="$(auth GET "/projects/$PROJECT")"; SECOND_RELEASE="$(jq -er '.current_release' <<<"$second")"
[[ "$SECOND_RELEASE" != "$ORIGINAL" && "$(jq -r '.image' <<<"$second")" == "$B" ]] || { echo "New image failed to become active" >&2; exit 1; }
echo "PASS blue/green switch to $SECOND_RELEASE"
https_check
if [[ "$MODE" == live ]]; then
 BAD="nginx:drevanta-smoke-intentionally-missing-$(date +%s)"
 failure="$(auth POST "/projects/$PROJECT/deploy" -H 'Content-Type: application/json' --data-binary "$(jq -nc --arg image "$BAD" '{image:$image}')")"
 wait_job "$(jq -er '.job.id' <<<"$failure")" failed
 stable="$(auth GET "/projects/$PROJECT")"
 [[ "$(jq -r '.current_release' <<<"$stable")" == "$SECOND_RELEASE" && "$(jq -r '.status' <<<"$stable")" == running ]] || { echo "Bad pull disturbed the previous release" >&2; exit 1; }
 echo "PASS failed image pull left active route untouched"
 https_check
else
 echo "SKIP failed image pull (mock mode does not pull images)"
fi
rollback="$(auth POST "/projects/$PROJECT/rollback" -H 'Content-Type: application/json' --data-binary '{}')"
wait_job "$(jq -er '.job.id' <<<"$rollback")" succeeded
after="$(auth GET "/projects/$PROJECT")"
[[ "$(jq -r '.image' <<<"$after")" == "$A" && "$(jq -r '.current_release' <<<"$after")" != "$SECOND_RELEASE" ]] || { echo "Rollback did not restore first image" >&2; exit 1; }
echo "PASS rollback"
https_check
auth GET "/releases?project=$PROJECT" | jq -e '.releases | length >= 3' >/dev/null
echo "PASS release history"
# Authenticated SSE logs: stream may time out normally, but must yield at least one frame.
set +e
logdata="$(curl --no-buffer -fsS --max-time 5 "$BASE/api/v1/projects/$PROJECT/logs/stream" -H "Authorization: Bearer $TOKEN" -H 'Accept: text/event-stream' 2>/dev/null)"
set -e
if grep -q '^data:' <<<"$logdata"; then
 echo "PASS authorized log stream"
elif [[ "$MODE" == mock ]]; then
 echo "Mock log stream produced no events" >&2; exit 1
else
 echo "WARNING no log events received during this short live smoke window" >&2
fi
echo "M08C $MODE acceptance completed for project $PROJECT"
