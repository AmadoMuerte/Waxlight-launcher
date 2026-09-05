#!/usr/bin/env bash

set -euo pipefail

script_directory="$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    pwd
)"

repository_root="$(
    cd "$script_directory/.."
    pwd
)"

nix_file="$repository_root/nix/waxlight.nix"

if ! command -v nix >/dev/null 2>&1; then
    echo "error: nix is required but not installed or not in PATH" >&2
    exit 1
fi

if [[ ! -f "$nix_file" ]]; then
    echo "error: nix derivation not found: $nix_file" >&2
    exit 1
fi

# build_derivation runs nix build and is only called under `if`, so a failed
# build does not trip `set -e`; the output is captured for hash extraction.
build_derivation() {
    (cd "$repository_root" && nix build . --no-link 2>&1)
}

if build_output="$(build_derivation)"; then
    echo "vendorHash is already correct"
    exit 0
fi

# A hash mismatch prints the actual hash on the `got:` line, e.g.
#   got:    sha256-XXXXXXXXX=
new_hash="$(
    printf '%s\n' "$build_output" |
        sed -n 's/.*got:[[:space:]]*\(sha256-[A-Za-z0-9+/=]*\)[[:space:]]*$/\1/p' |
        head -n 1
)"

if [[ -z "$new_hash" ]]; then
    echo "error: nix build failed but no vendorHash mismatch ('got: sha256-...') was found in the output:" >&2
    printf '%s\n' "$build_output" >&2
    exit 1
fi

if ! grep -q 'vendorHash = "sha256-' "$nix_file"; then
    echo "error: no vendorHash to update in $nix_file" >&2
    exit 1
fi

echo "updating vendorHash to $new_hash in $nix_file"
sed -i "s/vendorHash = \"sha256-[A-Za-z0-9+/=]*\"/vendorHash = \"$new_hash\"/" "$nix_file"

if build_output="$(build_derivation)"; then
    echo "vendorHash updated successfully"
    exit 0
fi

echo "error: nix build failed after updating vendorHash:" >&2
printf '%s\n' "$build_output" >&2
exit 1