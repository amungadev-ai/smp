#!/usr/bin/env bash
# Build the Drevanta engine, install the binary + systemd service.
# Usage: sudo bash install.sh [--force-mock]
set -euo pipefail

step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root: sudo bash install.sh"
command -v go >/dev/null || die "Go 1.22+ required (apt install golang-go)"

step "Building engine"
go build -o /usr/local/bin/drevanta-engine ./cmd/drevanta-engine

step "Deploying files"
mkdir -p /var/lib/drevanta-engine /opt/drevanta-engine
cp -r web /opt/drevanta-engine/
cp deploy/drevanta-engine.service /etc/systemd/system/

if grep -qs "^Environment=DRVT_ADMIN_PASSWORD=$" /etc/systemd/system/drevanta-engine.service; then
    PW="$(head -c 15 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 20)"
    sed -i "s|^Environment=DRVT_ADMIN_PASSWORD=$|Environment=DRVT_ADMIN_PASSWORD=$PW|" \
        /etc/systemd/system/drevanta-engine.service
    chmod 600 /etc/systemd/system/drevanta-engine.service
    printf 'Admin password: %s\n' "$PW"
fi

step "Starting service"
systemctl daemon-reload
systemctl enable --now drevanta-engine
sleep 1
curl -fsS http://127.0.0.1:8080/api/v1/health && echo && echo "Engine is live."
