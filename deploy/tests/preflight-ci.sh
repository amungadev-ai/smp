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
invalid_domain="$(./deploy/preflight.sh --check-live --confirm-disposable --domain bad..domain --expected-ip 8.8.8.8 2>&1)" && { echo 'Invalid test hostname accepted' >&2;exit 1; }
[[ "$invalid_domain" == *'valid dedicated test hostname'* ]] || { echo 'Domain was not rejected during validation' >&2;exit 1; }
invalid_ip="$(./deploy/preflight.sh --check-live --confirm-disposable --domain test.example.com --expected-ip 127.0.0.1 2>&1)" && { echo 'Loopback IP accepted as public test IP' >&2;exit 1; }
[[ "$invalid_ip" == *'private, reserved or non-public'* ]] || { echo 'IP was not rejected during validation' >&2;exit 1; }
echo 'PASS M10 preflight is read-only and requires disposable-host confirmation'
