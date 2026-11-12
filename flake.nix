{
  description = "Waxlight Launcher — a modern, lightweight launcher for Vintage Story";

  inputs = {
    # Pinned to the exact nixpkgs revision the release is built and tested
    # against. GL libraries (mesa) are injected at runtime by the host system
    # via /run/opengl-driver, so the closure's glibc must match the system's:
    # mesa 26.x requires GLIBC_2.43+, and an older-glibc closure fails to load
    # the EGL vendor library, which leaves the WebKit window blank.
    nixpkgs.url = "github:NixOS/nixpkgs/1d060086d60cb76a40cf37109f979d88a5bd6396";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;

      # ── Go 1.27.4 ───────────────────────────────────────
      goToolchain = pkgs: pkgs.stdenvNoCC.mkDerivation {
        pname = "go";
        version = "1.27.4";

        # buildGoModule reads GOOS/GOARCH/CGO_ENABLED and meta from the Go
        # derivation; nixpkgs' go exposes them and this hand-rolled toolchain must
        # too. Values mirror the host platform (this flake is x86_64-linux only).
        GOOS = pkgs.stdenv.hostPlatform.go.GOOS;
        GOARCH = pkgs.stdenv.hostPlatform.go.GOARCH;
        # cgo is required for mattn/go-sqlite3.
        CGO_ENABLED = "1";

        src = pkgs.fetchurl {
          url = "https://go.dev/dl/go1.27.4.linux-amd64.tar.gz";
          hash = "sha256-ciZzbCM5cZzaKPvzpU1xqmO87+N26YzvAANCaNqySEA=";
        };

        sourceRoot = ".";

        installPhase = ''
          runHook preInstall

          mkdir -p $out
          cp -r go/* $out/

          runHook postInstall
        '';

        meta.platforms = pkgs.lib.platforms.linux;
      };

      # ── Bun 1.4.4 ───────────────────────────────────────
      # Pinned to the stable bun-v1.4.4 release to match the frontend
      # toolchain pin; the canary channel re-publishes its zip on every
      # build, which breaks the fetchurl hash each time.
      #
      # Bun is a dynamically linked binary with the generic
      # /lib64/ld-linux-x86-64.so.2 interpreter, which does not exist inside the
      # Nix sandbox; autoPatchelfHook rewrites it to the stdenv loader so the
      # same derivation works for the devShell and for the sandboxed app build.
      bunToolchain = pkgs: pkgs.stdenvNoCC.mkDerivation {
        pname = "bun";
        version = "1.4.4";

        src = pkgs.fetchurl {
          url = "https://github.com/oven-sh/bun/releases/download/bun-v1.4.4/bun-linux-x64.zip";
          hash = "sha256-v4zL+ysLgBWeD5ptFcVTF63YB3U028YTCj7bUvzLOVw=";
        };

        # The release zip unpacks to a single bun-linux-x64/ directory.
        sourceRoot = "bun-linux-x64";

        nativeBuildInputs = [
          pkgs.unzip
          pkgs.autoPatchelfHook
        ];

        dontConfigure = true;
        dontBuild = true;

        installPhase = ''
          runHook preInstall

          install -Dm755 bun $out/bin/bun
          ln -s $out/bin/bun $out/bin/bunx

          runHook postInstall
        '';

        meta.platforms = pkgs.lib.platforms.linux;
      };
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.callPackage ./nix/waxlight.nix {
            go = goToolchain pkgs;
            bun = bunToolchain pkgs;
          };
        }
      );

      apps = forAllSystems (
        system: {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/waxlight";
          };
        }
      );

      checks = forAllSystems (
        system: {
          default = self.packages.${system}.default;
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          go = goToolchain pkgs;
          bun = bunToolchain pkgs;
        in
        {
          default = pkgs.mkShell {
            name = "waxlight-dev";

            packages = [
              go
              bun
              pkgs.pkg-config
              pkgs.gtk3
              pkgs.webkitgtk_4_1

              pkgs.wails
            ];

            shellHook = ''
              echo
              echo "Waxlight development environment"
              echo "────────────────────────────────"
              echo "Go:    $(go version)"
              echo "Bun:   $(bun --version)"
              echo "Wails: $(wails version 2>/dev/null | head -n1)"
              echo
            '';
          };
        }
      );
    };
}
