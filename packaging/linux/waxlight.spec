Name:           waxlight
Version:        %{waxlight_version}
Release:        1%{?dist}
Summary:        Unofficial launcher for Vintage Story
License:        GPL-3.0-or-later
URL:            https://github.com/AmadoMuerte/Waxlight-launcher
BuildArch:      x86_64
Requires:       gtk3
Requires:       webkit2gtk4.1

%description
Waxlight Launcher is an unofficial desktop launcher for Vintage Story with
isolated instances, version management, accounts, mods, and playtime tracking.

%prep

%build

%install
install -Dm755 %{waxlight_binary} %{buildroot}/usr/bin/waxlight
install -Dm644 %{waxlight_desktop} %{buildroot}/usr/share/applications/com.waxlight.launcher.desktop
install -Dm644 %{waxlight_icon} %{buildroot}/usr/share/icons/hicolor/scalable/apps/com.waxlight.launcher.svg
install -Dm644 %{waxlight_license} %{buildroot}/usr/share/licenses/waxlight/LICENSE
install -Dm644 %{waxlight_notice} %{buildroot}/usr/share/doc/waxlight/NOTICE
install -Dm644 %{waxlight_readme} %{buildroot}/usr/share/doc/waxlight/README.md

%files
/usr/bin/waxlight
/usr/share/applications/com.waxlight.launcher.desktop
/usr/share/icons/hicolor/scalable/apps/com.waxlight.launcher.svg
/usr/share/licenses/waxlight/LICENSE
/usr/share/doc/waxlight/NOTICE
/usr/share/doc/waxlight/README.md

%changelog
* Mon Aug 03 2026 Waxlight contributors <noreply@waxlight.local> - %{waxlight_version}-1
- Automated Waxlight Launcher release.
