#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd -P)"
cd "$root"
bash -n deploy/preflight.sh
./deploy/preflight.sh --plan | grep -Fq 'NO live deployment'
./deploy/preflight.sh --check-local
if ./deploy/preflight.sh --check-live --domain test.example.com --expected-ip 8.8.8.8 >/dev/null 2>&1;then
 echo 'Unconfirmed live preflight accepted' >&2;exit 1
fi
if ./deploy/preflight.sh --check-live --confirm-disposable --domain bad..domain --expected-ip 8.8.8.8 >/dev/null 2>&1;then
 echo 'Invalid test hostname accepted' >&2;exit 1
fi
if ./deploy/preflight.sh --check-live --confirm-disposable --domain test.example.com --expected-ip 127.0.0.1 >/dev/null 2>&1;then
 echo 'Loopback IP accepted as public test IP' >&2;exit 1
fi
echo 'PASS M10 preflight is read-only and requires disposable-host confirmation'
