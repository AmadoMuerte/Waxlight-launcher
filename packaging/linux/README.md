# Waxlight Launcher — Linux Portable

This is the portable Linux build of Waxlight Launcher.

## Recommended first launch

Open a terminal in this folder and run:

```bash
./run.sh
```

`run.sh` checks whether the Linux runtime libraries required by Waxlight are installed. If anything is missing, it can offer to install the required packages using your system package manager and then start Waxlight.

Currently supported package managers:

- `apt` — Debian / Ubuntu and derivatives
- `pacman` — Arch Linux / Manjaro and derivatives
- `dnf` / `dnf5` — Fedora and derivatives
- `zypper` — openSUSE

Administrator authentication may be requested if packages need to be installed.

## Later launches

```bash
./waxlight
```

## Important

Do not rename or replace the `waxlight` executable inside this folder. Waxlight's portable Linux auto-updater expects the application executable to keep that exact name.

If automatic dependency installation is not available on your distribution, `run.sh` will print the missing libraries and suggested package names so they can be installed manually.
