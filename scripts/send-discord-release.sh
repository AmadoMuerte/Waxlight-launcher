#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <release-tag> <prerelease: true|false>" >&2
  exit 2
fi

tag="$1"
prerelease="$2"

: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN is required}"
: "${DISCORD_RELEASE_WEBHOOK:?DISCORD_RELEASE_WEBHOOK is required}"

api_url="${GITHUB_API_URL:-https://api.github.com}"

windows_pattern="windows-amd64-installer.exe"
linux_pattern="linux-amd64.tar.gz"

max_attempts=30
attempt=0

windows_url=""
linux_url=""
release_url=""

echo "Querying assets for release ${tag}..."

while [[ $attempt -lt $max_attempts ]]; do
  attempt=$((attempt + 1))

  release_json="$(curl -sS \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "${api_url}/repos/${GITHUB_REPOSITORY}/releases/tags/${tag}")" || true

  windows_url="$(printf '%s' "$release_json" | jq -r --arg pat "$windows_pattern" \
    '[.assets[]? | select(.name | endswith($pat)) | .browser_download_url] | first // empty')" || true

  linux_url="$(printf '%s' "$release_json" | jq -r --arg pat "$linux_pattern" \
    '[.assets[]? | select(.name | endswith($pat)) | .browser_download_url] | first // empty')" || true

  release_url="$(printf '%s' "$release_json" | jq -r '.html_url // empty')" || true

  if [[ -n "$windows_url" && -n "$linux_url" ]]; then
    break
  fi

  [[ -z "$windows_url" ]] && echo "  attempt ${attempt}/${max_attempts}: Windows asset (${windows_pattern}) not found yet"
  [[ -z "$linux_url" ]] && echo "  attempt ${attempt}/${max_attempts}: Linux asset (${linux_pattern}) not found yet"

  if [[ $attempt -lt $max_attempts ]]; then
    sleep 10
  fi
done

if [[ -z "$windows_url" || -z "$linux_url" ]]; then
  echo "error: could not find the required release assets for ${tag}" >&2
  [[ -z "$windows_url" ]] && echo "  missing: Windows installer (name ends with ${windows_pattern})" >&2
  [[ -z "$linux_url" ]] && echo "  missing: Linux package (name ends with ${linux_pattern})" >&2
  echo "the GitHub Release is not affected; only the Discord notification failed." >&2
  exit 1
fi

if [[ "$prerelease" == "true" ]]; then
  headline="Waxlight Launcher ${tag} pre-release is now available! 🧪"
else
  headline="Waxlight Launcher ${tag} is now available! 🎉"
fi

content="$(cat << EOF
${headline}

**Download**

🪟 **Windows**
${windows_url}

🐧 **Linux**
${linux_url}

📦 **All downloads**
${release_url}

Enjoy the game! ❤️
EOF
)"

payload="$(jq -n --arg content "$content" '{content: $content}')"

response_file="$(mktemp)"
http_status="$(curl -sS \
  -o "$response_file" \
  -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  --data "$payload" \
  "$DISCORD_RELEASE_WEBHOOK")" || true

if [[ ! "$http_status" =~ ^2[0-9][0-9]$ ]]; then
  echo "error: Discord webhook returned HTTP ${http_status}" >&2
  if [[ -s "$response_file" ]]; then
    cat "$response_file" >&2
  fi
  rm -f "$response_file"
  exit 1
fi

rm -f "$response_file"

echo "Discord release notification sent for ${tag}."
