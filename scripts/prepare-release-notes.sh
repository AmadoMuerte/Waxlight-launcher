#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [auto]" >&2
  exit 2
fi

version="${1#v}"
auto_mode="${2:-}"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "error: invalid release version: $1" >&2
  exit 2
fi

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
notes_file="${project_root}/releases/v${version}.md"
relative_file="releases/v${version}.md"

mkdir -p "$(dirname "$notes_file")"

echo "Waxlight Release v${version}"
echo

if [[ -n "$auto_mode" ]]; then
  if ! command -v gh >/dev/null 2>&1; then
    echo "error: gh is required for AUTO=1 release notes" >&2
    exit 1
  fi
  remote_url="$(git -C "$project_root" remote get-url origin 2>/dev/null || true)"
  owner_repo="$(printf '%s' "$remote_url" | sed -E 's#(^https?://[^/]+/|^git@[^:]+:)([^/]+)/([^/.]+)(\.git)?$#\2/\3#')"
  if [[ -z "$owner_repo" || "$owner_repo" == *"://"* ]]; then
    echo "error: could not determine the GitHub repository from the origin remote" >&2
    exit 1
  fi
  branch="$(git -C "$project_root" branch --show-current 2>/dev/null || true)"
  branch="${branch:-main}"
  echo "Generating release notes from commit history..."
  generated_body="$(gh api \
    -X POST \
    "repos/${owner_repo}/releases/generate-notes" \
    -f "tag_name=v${version}" \
    -f "target_commitish=${branch}" \
    -q '.body')"
  if [[ -z "$generated_body" ]]; then
    echo "error: could not generate automatic release notes" >&2
    exit 1
  fi
  printf '%s\n' "$generated_body" > "$notes_file"
  echo "Generated release notes automatically:"
  echo "  ${relative_file}"
  echo
  echo "Release notes validated."
  exit 0
fi

if [[ -f "$notes_file" ]]; then
  echo "Release notes already exist:"
  echo "  ${relative_file}"
  echo
  echo "Using the existing file."
else
  cat > "$notes_file" << EOF
# Waxlight Launcher v${version}


Enjoy the game! ❤️

❤️ [Support Waxlight](https://hipolink.net/amadomuerte)
EOF
  echo "Created release notes:"
  echo "  ${relative_file}"
fi
echo

ensure_footer() {
  if ! grep -q "^Enjoy the game" "$notes_file" ||
    ! grep -q "^❤️ \[Support Waxlight\]" "$notes_file"; then
    printf '\nEnjoy the game! ❤️\n\n❤️ [Support Waxlight](https://hipolink.net/amadomuerte)\n' >> "$notes_file"
    echo "Restored the release notes footer."
    echo
  fi
}

validate_notes() {
  local body
  body="$(sed \
    -e '/^# Waxlight Launcher v/d' \
    -e '/^Enjoy the game/,$d' \
    "$notes_file")"
  [[ -n "$(printf '%s' "$body" | tr -d '[:space:]')" ]]
}

while :; do
  echo "Edit the release notes now:"
  echo "  ${relative_file}"
  echo
  echo "When you are finished, return here and press Enter to continue the release."
  read -r _
  echo

  ensure_footer

  if validate_notes; then
    echo "Release notes validated."
    echo
    break
  fi

  echo "Release notes are empty."
  echo
  echo "Edit:"
  echo "  ${relative_file}"
  echo
  echo "Then press Enter again."
done
