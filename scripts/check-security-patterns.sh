#!/usr/bin/env bash
set -euo pipefail

if rg -n 'InsecureSkipVerify\s*:\s*true' --glob '*.go' .; then
  echo 'InsecureSkipVerify=true is prohibited' >&2
  exit 1
fi

if rg -n 'NewFileStore|type FileStore struct' internal --glob '*.go'; then
  echo 'Production plaintext credential stores are prohibited' >&2
  exit 1
fi

if rg -n 'json:"(password|totpcode|prelogintoken|sessionkey|sessionsignature)' internal/presentation frontend/src/wailsjs; then
  echo 'Secret-bearing public DTO or generated binding detected' >&2
  exit 1
fi

legacy_occurrences="$(rg -l 'account-secrets\.json' internal --glob '*.go' --glob '!**/*_test.go' || true)"
if [[ -n "$legacy_occurrences" && "$legacy_occurrences" != "internal/platform/credentials/migration.go" ]]; then
  echo "Legacy plaintext filename used outside migration: $legacy_occurrences" >&2
  exit 1
fi

echo 'Security pattern checks passed'
