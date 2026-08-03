#!/usr/bin/env bash
set -euo pipefail

for required_command in node npm wails; do
  if ! command -v "$required_command" >/dev/null 2>&1; then
    echo "required build command is unavailable: ${required_command}" >&2
    exit 1
  fi
done

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [output-directory]" >&2
  exit 2
fi

release_version="${1#v}"
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_directory="${2:-${project_root}/release}"
build_config_directory="$(mktemp -d /tmp/waxlight-build-config-XXXXXX)"
trap 'rm -rf "$build_config_directory"' EXIT

"${project_root}/scripts/check-version.sh" "$release_version"
mkdir -p "$output_directory"

npm ci --prefix "${project_root}/frontend"
npm --prefix "${project_root}/frontend" run build

(
  cd "${project_root}/cmd/waxlight"
  XDG_CONFIG_HOME="$build_config_directory" wails build -clean -platform linux/amd64
)

binary_path="${project_root}/build/bin/waxlight"
if [[ ! -x "$binary_path" ]]; then
  echo "Wails did not produce ${binary_path}" >&2
  exit 1
fi

"${project_root}/scripts/package-linux.sh" "$release_version" "$binary_path" "$output_directory"
