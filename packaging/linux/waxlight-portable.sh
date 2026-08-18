#!/usr/bin/env bash

set -uo pipefail

script_directory="$(
    cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
    pwd
)"
application_binary="$script_directory/waxlight"

if [[ ! -x "$application_binary" ]]; then
    echo "Waxlight portable binary is missing or is not executable: $application_binary" >&2
    exit 1
fi

if ! command -v ldd >/dev/null 2>&1; then
    echo "Waxlight could not check Linux runtime dependencies because 'ldd' is unavailable." >&2
    echo "Install GTK3 and WebKitGTK 4.1 for your distribution, then run Waxlight again." >&2
    exit 1
fi

missing_libraries() {
    LC_ALL=C ldd "$application_binary" 2>/dev/null |
        awk '/=> not found/ { print $1 }'
}

mapfile -t missing < <(missing_libraries)

if (( ${#missing[@]} == 0 )); then
    exec "$application_binary" "$@"
fi

echo "Waxlight is missing required Linux runtime libraries:" >&2
printf '  - %s\n' "${missing[@]}" >&2
echo >&2

run_as_root() {
    if (( EUID == 0 )); then
        "$@"
        return
    fi

    if [[ -t 0 ]] && command -v sudo >/dev/null 2>&1; then
        sudo "$@"
        return
    fi

    if command -v pkexec >/dev/null 2>&1; then
        pkexec "$@"
        return
    fi

    if command -v sudo >/dev/null 2>&1; then
        sudo "$@"
        return
    fi

    echo "Waxlight needs administrator privileges to install runtime dependencies." >&2
    echo "Neither sudo nor pkexec is available." >&2
    return 1
}

install_runtime_dependencies() {
    if command -v apt-get >/dev/null 2>&1; then
        echo "Installing GTK3 and WebKitGTK 4.1 with apt..."
        run_as_root apt-get update &&
            run_as_root apt-get install -y libgtk-3-0 libwebkit2gtk-4.1-0
        return
    fi

    if command -v pacman >/dev/null 2>&1; then
        echo "Installing GTK3 and WebKitGTK 4.1 with pacman..."
        run_as_root pacman -S --needed --noconfirm gtk3 webkit2gtk-4.1
        return
    fi

    if command -v dnf5 >/dev/null 2>&1; then
        echo "Installing GTK3 and WebKitGTK 4.1 with dnf5..."
        run_as_root dnf5 install -y gtk3 webkit2gtk4.1
        return
    fi

    if command -v dnf >/dev/null 2>&1; then
        echo "Installing GTK3 and WebKitGTK 4.1 with dnf..."
        run_as_root dnf install -y gtk3 webkit2gtk4.1
        return
    fi

    if command -v zypper >/dev/null 2>&1; then
        echo "Installing GTK3 and WebKitGTK 4.1 with zypper..."
        run_as_root zypper --non-interactive install libgtk-3-0 libwebkit2gtk-4_1-0
        return
    fi

    echo "Waxlight could not detect a supported package manager." >&2
    echo >&2
    echo "Install the GTK3 and WebKitGTK 4.1 runtime packages manually:" >&2
    echo "  Debian/Ubuntu: libgtk-3-0 libwebkit2gtk-4.1-0" >&2
    echo "  Arch/Manjaro:  gtk3 webkit2gtk-4.1" >&2
    echo "  Fedora 40+:    gtk3 webkit2gtk4.1" >&2
    echo "  openSUSE:      libgtk-3-0 libwebkit2gtk-4_1-0" >&2
    return 1
}

if [[ -t 0 ]]; then
    printf 'Install the required packages automatically now? [Y/n] '
    read -r answer

    case "${answer:-y}" in
        y|Y|yes|YES|Yes)
            ;;
        *)
            echo "Dependency installation cancelled." >&2
            exit 1
            ;;
    esac
else
    echo "Waxlight will try to install the required runtime packages automatically."
    echo "Your system may display an administrator authentication prompt."
fi

if ! install_runtime_dependencies; then
    echo >&2
    echo "Automatic dependency installation failed." >&2
    echo "Install the packages shown above and run Waxlight again." >&2
    exit 1
fi

mapfile -t missing < <(missing_libraries)

if (( ${#missing[@]} != 0 )); then
    echo >&2
    echo "Waxlight still has unresolved runtime libraries after installation:" >&2
    printf '  - %s\n' "${missing[@]}" >&2
    echo "Your distribution may require different package names." >&2
    exit 1
fi

exec "$application_binary" "$@"
