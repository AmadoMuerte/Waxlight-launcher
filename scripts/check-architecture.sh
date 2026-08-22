#!/usr/bin/env bash
# Checks the Waxlight backend architecture:
#
#  1. Feature packages must not import the composition root, the Wails
#     transport, or platform adapters (SQLite, filesystem, OS, network).
#  2. The Wails transport must not import SQLite or platform adapters.
#  3. The legacy global hierarchy (internal/application, internal/domain,
#     internal/infrastructure, internal/presentation) must not exist and no
#     god service/store may be reintroduced under those names.
#  4. Generated Wails bindings must not expose credential fields.
#
# Run with `make architecture` or directly.

set -euo pipefail

cd "$(dirname "$0")/.."

FEATURES="accounts downloads events gamelog instances language launching mods mutations news operations publishers recovery servers sessions settings snapshots statistics telemetry updates version versions"
FORBIDDEN_FROM_FEATURES="waxlight-launcher/internal/app waxlight-launcher/internal/transport waxlight-launcher/internal/platform"
LEGACY_PACKAGES="internal/application internal/domain internal/infrastructure internal/presentation"
BINDINGS_DIR="frontend/src/wailsjs/go"
PROHIBITED_NAMES="password totpcode prelogintoken sessionkey sessionsignature credentialstorepath"

failures=0

note_failure() {
  echo "architecture: $1" >&2
  failures=$((failures + 1))
}

for package in $FEATURES; do
  imports="$(go list -f '{{join .Imports "\n"}}' "./internal/$package" 2>/dev/null)"
  for forbidden in $FORBIDDEN_FROM_FEATURES; do
    if echo "$imports" | grep -q "^$forbidden"; then
      note_failure "$package imports forbidden dependency $forbidden"
    fi
  done
done

transport_imports="$(go list -f '{{join .Imports "\n"}}' ./internal/transport/wails)"
for forbidden in $FORBIDDEN_FROM_FEATURES; do
  if echo "$transport_imports" | grep -q "^$forbidden"; then
    note_failure "internal/transport/wails imports forbidden dependency $forbidden"
  fi
done

for package in $LEGACY_PACKAGES; do
  if [ -d "$package" ]; then
    note_failure "legacy package $package still exists"
  fi
done

if find internal -maxdepth 1 -type d -name 'application' >/dev/null 2>&1; then
  :
fi

for binding in "$BINDINGS_DIR"/models.ts "$BINDINGS_DIR"/go/wails/*.d.ts; do
  if [ -f "$binding" ]; then
    lowered="$(tr '[:upper:]' '[:lower:]' < "$binding")"
    for prohibited in $PROHIBITED_NAMES; do
      if echo "$lowered" | grep -q "$prohibited"; then
        note_failure "$binding exposes prohibited name $prohibited"
      fi
    done
  fi
done

if [ "$failures" -ne 0 ]; then
  echo "architecture: $failures check(s) failed" >&2
  exit 1
fi

echo "architecture: all checks passed"
