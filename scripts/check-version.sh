#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

release_version="${1#v}"
if [[ ! "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid release version: $1" >&2
  exit 2
fi

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
root_version="$(node -p "require('${project_root}/wails.json').info.productVersion")"
command_version="$(node -p "require('${project_root}/cmd/waxlight/wails.json').info.productVersion")"

if [[ "$root_version" != "$release_version" || "$command_version" != "$release_version" ]]; then
  echo "release version ${release_version} does not match both wails.json files (${root_version}, ${command_version})" >&2
  exit 1
fi

echo "Waxlight release version: ${release_version}"
