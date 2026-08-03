#!/usr/bin/env bash
set -euo pipefail

for required_command in dpkg-deb rpmbuild rpm tar install realpath; do
  if ! command -v "$required_command" >/dev/null 2>&1; then
    echo "required packaging command is unavailable: ${required_command}" >&2
    exit 1
  fi
done

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <version> <binary> <output-directory>" >&2
  exit 2
fi

release_version="${1#v}"
binary_path="$(realpath "$2")"
output_directory="$(mkdir -p "$3" && realpath "$3")"
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
asset_prefix="Waxlight-Launcher-v${release_version}-linux-amd64"
temporary_root="$(mktemp -d /tmp/waxlight-package-XXXXXX)"
trap 'rm -rf "$temporary_root"' EXIT

"${project_root}/scripts/check-version.sh" "$release_version"
if [[ ! -x "$binary_path" ]]; then
  echo "Linux binary is missing or not executable: ${binary_path}" >&2
  exit 1
fi

portable_root="${temporary_root}/${asset_prefix}"
install -Dm755 "$binary_path" "${portable_root}/waxlight"
install -Dm644 "${project_root}/README.md" "${portable_root}/README.md"
install -Dm644 "${project_root}/LICENSE" "${portable_root}/LICENSE"
install -Dm644 "${project_root}/NOTICE" "${portable_root}/NOTICE"
tar -C "$temporary_root" -czf "${output_directory}/${asset_prefix}.tar.gz" "$asset_prefix"

debian_root="${temporary_root}/debian"
install -Dm755 "$binary_path" "${debian_root}/usr/bin/waxlight"
install -Dm644 "${project_root}/packaging/linux/com.waxlight.launcher.desktop" "${debian_root}/usr/share/applications/com.waxlight.launcher.desktop"
install -Dm644 "${project_root}/packaging/linux/com.waxlight.launcher.svg" "${debian_root}/usr/share/icons/hicolor/scalable/apps/com.waxlight.launcher.svg"
install -Dm644 "${project_root}/LICENSE" "${debian_root}/usr/share/doc/waxlight/LICENSE"
install -Dm644 "${project_root}/NOTICE" "${debian_root}/usr/share/doc/waxlight/NOTICE"
install -Dm644 "${project_root}/README.md" "${debian_root}/usr/share/doc/waxlight/README.md"
mkdir -p "${debian_root}/DEBIAN"
sed "s/@VERSION@/${release_version}/g" "${project_root}/packaging/linux/debian-control.in" > "${debian_root}/DEBIAN/control"
dpkg-deb --root-owner-group --build "$debian_root" "${output_directory}/${asset_prefix}.deb"

rpm_top="${temporary_root}/rpmbuild"
mkdir -p "${rpm_top}/BUILD" "${rpm_top}/BUILDROOT" "${rpm_top}/RPMS" "${rpm_top}/SOURCES" "${rpm_top}/SPECS" "${rpm_top}/SRPMS"
rpmbuild -bb "${project_root}/packaging/linux/waxlight.spec" \
  --define "_topdir ${rpm_top}" \
  --define "waxlight_version ${release_version}" \
  --define "waxlight_binary ${binary_path}" \
  --define "waxlight_desktop ${project_root}/packaging/linux/com.waxlight.launcher.desktop" \
  --define "waxlight_icon ${project_root}/packaging/linux/com.waxlight.launcher.svg" \
  --define "waxlight_license ${project_root}/LICENSE" \
  --define "waxlight_notice ${project_root}/NOTICE" \
  --define "waxlight_readme ${project_root}/README.md"

rpm_path="$(find "${rpm_top}/RPMS" -type f -name '*.rpm' -print -quit)"
if [[ -z "$rpm_path" ]]; then
  echo "rpmbuild did not produce an RPM package" >&2
  exit 1
fi
cp "$rpm_path" "${output_directory}/${asset_prefix}.rpm"

dpkg-deb --info "${output_directory}/${asset_prefix}.deb" >/dev/null
rpm -qip "${output_directory}/${asset_prefix}.rpm" >/dev/null
tar -tzf "${output_directory}/${asset_prefix}.tar.gz" >/dev/null

echo "Linux packages written to ${output_directory}"
