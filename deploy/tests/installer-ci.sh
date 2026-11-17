#!/usr/bin/env bash
# Non-root installer policy tests. Never invokes --install.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
cd "$ROOT"
for script in deploy/install.sh deploy/verify.sh deploy/tests/installer-ci.sh deploy/tests/stack-ci.sh deploy/state.sh deploy/tests/state-ci.sh deploy/preflight.sh deploy/tests/preflight-ci.sh engine/install.sh;do bash -n "$script";done
plan="$(./deploy/install.sh --plan)"
[[ "$plan" == *'--mock'* && "$plan" == *'127.0.0.1:3000'* && "$plan" == *'127.0.0.1:8080'* ]] || { echo 'Unsafe installer plan' >&2;exit 1; }
legacy="$(./engine/install.sh --plan)"
[[ "$legacy" == *'M09 installation plan'* ]] || { echo 'Old entrypoint did not delegate' >&2;exit 1; }
if ./deploy/install.sh --bogus >/dev/null 2>&1;then echo 'Unexpected option accepted' >&2;exit 1;fi
engine=deploy/systemd/drevanta-engine.service;portal=deploy/systemd/drevanta-portal.service
grep -Fq 'ExecStart=/opt/drevanta/current/engine/drevanta-engine --mock --addr 127.0.0.1:8080' "$engine"
grep -Fxq 'Environment=HOST=127.0.0.1' "$portal"
grep -Fxq 'Environment=DRVT_ENGINE_URL=http://127.0.0.1:8080' "$portal"
for unit in "$engine" "$portal";do
 if [[ "$unit" == "$portal" ]];then grep -Fxq 'User=drevanta-portal' "$unit";else grep -Fxq 'User=drevanta' "$unit";fi
 grep -Fxq 'NoNewPrivileges=true' "$unit"
 grep -Fxq 'ProtectSystem=strict' "$unit"
 ! grep -q -- '--live\|--ack-disposable-vps' "$unit"
done
grep -Fq 'PORTAL_USER=drevanta-portal' deploy/install.sh
grep -Fq 'BUILD_USER=drevanta-build' deploy/install.sh
grep -Fq 'go build -o drevanta-maint ./cmd/drevanta-maint' deploy/install.sh
grep -Fq 'runuser -u drevanta -- "$BIN" restore' deploy/state.sh
grep -Fq 'systemctl is-active --quiet drevanta-engine.service' deploy/state.sh
grep -Fq 'chmod -R a+rX "$RELEASES/$RELEASE"' deploy/install.sh
echo 'PASS M09 CLI syntax, preview policy and service unit checks'
