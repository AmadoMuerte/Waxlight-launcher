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
# The explicit experimental features make the flake invocation independent of
# the user's nix configuration (nix-command/flakes).
build_derivation() {
    (cd "$repository_root" && nix build . --no-link --extra-experimental-features 'nix-command flakes' 2>&1)
}

if build_output="$(build_derivation)"; then
    echo "vendorHash is already correct"
    exit 0
fi

# A stale vendorHash fails the fixed-output derivation that fetches the Go
# modules and prints the real hash on the `got:` line of its mismatch block:
#   error: hash mismatch in fixed-output derivation '/nix/store/...':
#     wanted: sha256-XXXXXXXXX=
#     got:    sha256-XXXXXXXXX=
# Only take `got:` from the first hash-mismatch block so unrelated output
# (evaluation errors, other failed derivations) cannot be mistaken for the
# vendor hash.
new_hash="$(
    printf '%s\n' "$build_output" |
        awk '
            /error: hash mismatch/ { block = 1; next }
            block && /got:[[:space:]]+sha256-/ { print; exit }
        ' |
        sed -n 's/.*got:[[:space:]]*\(sha256-[A-Za-z0-9+/=]*\)[[:space:]]*$/\1/p'
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

# Keep the original file so a failed or interrupted post-update build can
# restore it instead of leaving nix/waxlight.nix modified with an unverified
# hash. The restore is idempotent: it runs on every exit and signal, but only
# acts while the backup exists, i.e. before the update has been verified.
backup_file="$(mktemp)"
cp "$nix_file" "$backup_file"

restore_original() {
    if [[ -n "${backup_file:-}" && -f "$backup_file" ]]; then
        cp "$backup_file" "$nix_file"
        rm -f "$backup_file"
    fi
}
trap restore_original EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP

echo "updating vendorHash to $new_hash in $nix_file"
sed -i "s/vendorHash = \"sha256-[A-Za-z0-9+/=]*\"/vendorHash = \"$new_hash\"/" "$nix_file"

if build_output="$(build_derivation)"; then
    rm -f "$backup_file"
    echo "vendorHash updated successfully"
    exit 0
fi

echo "error: nix build failed after updating vendorHash; restoring the original file:" >&2
printf '%s\n' "$build_output" >&2
exit 1
