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

      # ── Go 1.27.2 ───────────────────────────────────────
      goToolchain = pkgs: pkgs.stdenvNoCC.mkDerivation {
        pname = "go";
        version = "1.27.2";

        # buildGoModule reads GOOS/GOARCH/CGO_ENABLED and meta from the Go
        # derivation; nixpkgs' go exposes them and this hand-rolled toolchain must
        # too. Values mirror the host platform (this flake is x86_64-linux only).
        GOOS = pkgs.stdenv.hostPlatform.go.GOOS;
        GOARCH = pkgs.stdenv.hostPlatform.go.GOARCH;
        # cgo is required for mattn/go-sqlite3.
        CGO_ENABLED = "1";

        src = pkgs.fetchurl {
          url = "https://go.dev/dl/go1.27.2.linux-amd64.tar.gz";
          hash = "sha256-7LrbmQkaP0bjH1+TSwaLGGTq+nmVIRs56t33aZYEX+U=";
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
    in
    {
      packages = forAllSystems (
        system: {
          default =
            nixpkgs.legacyPackages.${system}.callPackage ./nix/waxlight.nix {
              go = goToolchain nixpkgs.legacyPackages.${system};
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

          # ── Wails 2.12.0 ─────────────────────────────────────
          wails_2_12_0 = pkgs.buildGoModule rec {
            pname = "wails";
            version = "2.12.0";

            src = pkgs.fetchFromGitHub {
              owner = "wailsapp";
              repo = "wails";
              rev = "v${version}";
              hash = "sha256-XngfbEbXhPRRKbNp/aaVCleISABTs90d5JjmwIq7nsk=";
            };

            modRoot = "v2";

            vendorHash = "sha256-zQZ6bHiJhRSDaaZjgD31wcD+6vN1cOKt9LD42c6zbes=";

            subPackages = [
              "cmd/wails"
            ];
          };
        in
        {
          default = pkgs.mkShell {
            name = "waxlight-dev";

            packages = [
              go
              pkgs.nodejs_22
              pkgs.pkg-config
              pkgs.gtk3
              pkgs.webkitgtk_4_1

              wails_2_12_0
            ];

            shellHook = ''
              echo
              echo "Waxlight development environment"
              echo "────────────────────────────────"
              echo "Go:    $(go version)"
              echo "Node:  $(node --version)"
              echo "npm:   $(npm --version)"
              echo "Wails: $(wails version 2>/dev/null | head -n1)"
              echo
            '';
          };
        }
      );
    };
}
