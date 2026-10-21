{
  description = "Waxlight Launcher — a modern, lightweight launcher for Vintage Story";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;

      # ── Go 1.27.3 ───────────────────────────────────────
      goToolchain = pkgs: pkgs.stdenvNoCC.mkDerivation {
        pname = "go";
        version = "1.27.3";

        # buildGoModule reads GOOS/GOARCH/CGO_ENABLED and meta from the Go
        # derivation; nixpkgs' go exposes them and this hand-rolled toolchain must
        # too. Values mirror the host platform (this flake is x86_64-linux only).
        GOOS = pkgs.stdenv.hostPlatform.go.GOOS;
        GOARCH = pkgs.stdenv.hostPlatform.go.GOARCH;
        # cgo is required for mattn/go-sqlite3.
        CGO_ENABLED = "1";

        src = pkgs.fetchurl {
          url = "https://go.dev/dl/go1.27.3.linux-amd64.tar.gz";
          hash = "sha256-Ze+8gP3B+izSYxytW0gXd6aOhyXzTon1D8ascU43m0A=";
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
      # Waxlight's frontend linter is pinned to Bun 1.4.4, which is only
      # published on the canary release channel today. Once 1.4.4 ships as a
      # stable release, point `url` at
      #   https://github.com/oven-sh/bun/releases/download/bun-v1.4.4/bun-linux-x64.zip
      # and re-pin `hash` for that artifact.
      #
      # Bun is a dynamically linked binary with the generic
      # /lib64/ld-linux-x86-64.so.2 interpreter, which does not exist inside the
      # Nix sandbox; autoPatchelfHook rewrites it to the stdenv loader so the
      # same derivation works for the devShell and for the sandboxed app build.
      bunToolchain = pkgs: pkgs.stdenvNoCC.mkDerivation {
        pname = "bun";
        version = "1.4.4";

        src = pkgs.fetchurl {
          url = "https://github.com/oven-sh/bun/releases/download/canary/bun-linux-x64.zip";
          hash = "sha256-+mF39r5y9kqG+qasQIFF7bVyMILUPWGxiTJ6nUkchtA=";
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
