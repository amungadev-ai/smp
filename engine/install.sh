#!/usr/bin/env bash
# Compatibility entry point — no longer installs the legacy insecure unit.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
exec "$HERE/deploy/install.sh" "$@"
