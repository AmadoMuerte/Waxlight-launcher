#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "usage: $0 VERSION [OUTPUT_DIRECTORY]" >&2
    echo "example: $0 0.2.0-beta.1 release" >&2
    exit 1
fi

release_version="${1#v}"
output_directory="${2:-release}"

if [[ ! "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "error: invalid semantic version: $release_version" >&2
    exit 1
fi

script_directory="$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    pwd
)"

repository_root="$(
    cd "$script_directory/.."
    pwd
)"

if [[ "$output_directory" != /* ]]; then
    output_directory="$repository_root/$output_directory"
fi

base_version="${release_version%%-*}"
prerelease=""

if [[ "$release_version" == *-* ]]; then
    prerelease="${release_version#*-}"
fi

# Debian sorts 0.2.0~beta.1 before 0.2.0.
if [[ -n "$prerelease" ]]; then
    deb_version="${base_version}~${prerelease}"
else
    deb_version="$base_version"
fi

# RPM does not allow '-' in Version.
# Pre-release versions must therefore be represented through Release.
rpm_version="$base_version"

if [[ -n "$prerelease" ]]; then
    rpm_release="0.${prerelease//-/.}"
else
    rpm_release="1"
fi

architecture="amd64"
package_name="waxlight"
application_name="Waxlight Launcher"

build_directory="$repository_root/build"
build_bin_directory="$build_directory/bin"
application_binary="$build_bin_directory/waxlight"

staging_root="$build_directory/package-linux"
portable_root="$staging_root/portable"
deb_root="$staging_root/deb"
rpm_root="$staging_root/rpm"

portable_name="Waxlight-Launcher-v${release_version}-linux-${architecture}"
portable_archive="$output_directory/${portable_name}.tar.gz"
deb_package="$output_directory/${portable_name}.deb"
rpm_package="$output_directory/${portable_name}.rpm"

echo "Waxlight release version: $release_version"
echo "Debian version:           $deb_version"
echo "RPM version:              $rpm_version"
echo "RPM release:              $rpm_release"
echo "Output directory:         $output_directory"

find_first_file() {
    local pattern="$1"
    shift

    local directory
    local result

    for directory in "$@"; do
        [[ -d "$directory" ]] || continue

        result="$(
            find "$directory" \
                -type f \
                -name "$pattern" \
                -print \
                -quit
        )"

        if [[ -n "$result" ]]; then
            printf '%s\n' "$result"
            return 0
        fi
    done

    return 1
}

install_optional_file() {
    local source="$1"
    local destination="$2"
    local mode="$3"

    if [[ -n "$source" && -f "$source" ]]; then
        install -Dm"$mode" "$source" "$destination"
    fi
}

rm -rf "$staging_root"
mkdir -p "$output_directory"
mkdir -p "$build_bin_directory"

echo
echo "Building Linux application..."
echo "-----------------------------"

(
    cd "$repository_root/cmd/waxlight"

    CGO_ENABLED=1 \
    wails build \
        -clean \
        -platform linux/amd64 \
        -trimpath \
        -ldflags="-s -w"
)

if [[ ! -s "$application_binary" ]]; then
    echo "error: Wails binary was not created: $application_binary" >&2
    exit 1
fi

chmod 0755 "$application_binary"

desktop_file="$(
    find_first_file \
        "*.desktop" \
        "$repository_root/packaging/linux" \
        "$repository_root/build/linux" \
        2>/dev/null || true
)"

icon_file="$(
    find_first_file \
        "*.png" \
        "$repository_root/packaging/linux" \
        "$repository_root/cmd/waxlight/build" \
        "$repository_root/build" \
        2>/dev/null || true
)"

license_file=""

if [[ -f "$repository_root/LICENSE" ]]; then
    license_file="$repository_root/LICENSE"
fi

echo
echo "Creating portable archive..."
echo "----------------------------"

mkdir -p "$portable_root/$portable_name"

install \
    -Dm755 \
    "$application_binary" \
    "$portable_root/$portable_name/waxlight"

install_optional_file \
    "$desktop_file" \
    "$portable_root/$portable_name/waxlight.desktop" \
    0644

install_optional_file \
    "$icon_file" \
    "$portable_root/$portable_name/waxlight.png" \
    0644

install_optional_file \
    "$license_file" \
    "$portable_root/$portable_name/LICENSE" \
    0644

tar \
    -C "$portable_root" \
    -czf "$portable_archive" \
    "$portable_name"

echo
echo "Creating Debian package..."
echo "--------------------------"

mkdir -p "$deb_root/DEBIAN"
mkdir -p "$deb_root/usr/bin"
mkdir -p "$deb_root/usr/share/applications"
mkdir -p "$deb_root/usr/share/icons/hicolor/256x256/apps"
mkdir -p "$deb_root/usr/share/doc/$package_name"

install \
    -Dm755 \
    "$application_binary" \
    "$deb_root/usr/bin/waxlight"

install_optional_file \
    "$desktop_file" \
    "$deb_root/usr/share/applications/waxlight.desktop" \
    0644

install_optional_file \
    "$icon_file" \
    "$deb_root/usr/share/icons/hicolor/256x256/apps/waxlight.png" \
    0644

install_optional_file \
    "$license_file" \
    "$deb_root/usr/share/doc/$package_name/LICENSE" \
    0644

cat >"$deb_root/DEBIAN/control" <<EOF
Package: $package_name
Version: $deb_version
Section: games
Priority: optional
Architecture: $architecture
Maintainer: AmadoMuerte
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Description: Waxlight Launcher for Vintage Story
 A modern, lightweight and cross-platform launcher for Vintage Story.
EOF

chmod 0755 "$deb_root/DEBIAN"
chmod 0644 "$deb_root/DEBIAN/control"

dpkg-deb \
    --root-owner-group \
    --build \
    "$deb_root" \
    "$deb_package"

echo
echo "Creating RPM package..."
echo "-----------------------"

rpm_architecture="x86_64"

mkdir -p "$rpm_root/BUILD"
mkdir -p "$rpm_root/BUILDROOT"
mkdir -p "$rpm_root/RPMS"
mkdir -p "$rpm_root/SOURCES"
mkdir -p "$rpm_root/SPECS"
mkdir -p "$rpm_root/SRPMS"

rpm_source_root="$rpm_root/SOURCES/waxlight-${rpm_version}"

mkdir -p "$rpm_source_root/usr/bin"
mkdir -p "$rpm_source_root/usr/share/applications"
mkdir -p "$rpm_source_root/usr/share/icons/hicolor/256x256/apps"
mkdir -p "$rpm_source_root/usr/share/licenses/$package_name"

install \
    -Dm755 \
    "$application_binary" \
    "$rpm_source_root/usr/bin/waxlight"

install_optional_file \
    "$desktop_file" \
    "$rpm_source_root/usr/share/applications/waxlight.desktop" \
    0644

install_optional_file \
    "$icon_file" \
    "$rpm_source_root/usr/share/icons/hicolor/256x256/apps/waxlight.png" \
    0644

install_optional_file \
    "$license_file" \
    "$rpm_source_root/usr/share/licenses/$package_name/LICENSE" \
    0644

tar \
    -C "$rpm_root/SOURCES" \
    -czf "$rpm_root/SOURCES/waxlight-${rpm_version}.tar.gz" \
    "waxlight-${rpm_version}"

cat >"$rpm_root/SPECS/waxlight.spec" <<EOF
Name:           $package_name
Version:        $rpm_version
Release:        $rpm_release
Summary:        Waxlight Launcher for Vintage Story
License:        GPL-3.0-only
URL:            https://github.com/AmadoMuerte/Waxlight-launcher
Source0:        %{name}-%{version}.tar.gz
BuildArch:      $rpm_architecture

Requires:       gtk3
Requires:       webkit2gtk4.1

%description
Waxlight Launcher is a modern, lightweight and cross-platform
launcher for Vintage Story.

%prep
%setup -q

%build

%install
rm -rf %{buildroot}
cp -a usr %{buildroot}/

%files
%license /usr/share/licenses/$package_name/LICENSE
/usr/bin/waxlight
/usr/share/applications/waxlight.desktop
/usr/share/icons/hicolor/256x256/apps/waxlight.png

%changelog
* Tue Aug 04 2026 AmadoMuerte - $rpm_version-$rpm_release
- Waxlight Launcher $release_version
EOF

rpmbuild \
    --define "_topdir $rpm_root" \
    --define "_build_id_links none" \
    -bb \
    "$rpm_root/SPECS/waxlight.spec"

built_rpm="$(
    find "$rpm_root/RPMS" \
        -type f \
        -name "*.rpm" \
        -print \
        -quit
)"

if [[ -z "$built_rpm" || ! -s "$built_rpm" ]]; then
    echo "error: RPM package was not created" >&2
    exit 1
fi

cp "$built_rpm" "$rpm_package"

expected_assets=(
    "$portable_archive"
    "$deb_package"
    "$rpm_package"
)

echo
echo "Verifying Linux release assets..."
echo "---------------------------------"

for asset in "${expected_assets[@]}"; do
    if [[ ! -s "$asset" ]]; then
        echo "error: missing or empty release asset: $asset" >&2
        exit 1
    fi
done

ls -lh "${expected_assets[@]}"

echo
echo "Linux release packages created successfully."
