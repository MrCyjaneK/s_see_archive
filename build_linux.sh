#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
# shellcheck source=../linux-gnu.sh
source "$SCRIPT_DIR/../linux-gnu.sh"
linux_go_shared_module archive
