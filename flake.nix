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
    in
    {
      packages = forAllSystems (
        system: {
          default =
            nixpkgs.legacyPackages.${system}.callPackage ./nix/waxlight.nix { };
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

          # ── Go 1.25.13 ───────────────────────────────────────
          go_1_25_13 = pkgs.stdenvNoCC.mkDerivation {
            pname = "go";
            version = "1.25.13";

            src = pkgs.fetchurl {
              url = "https://go.dev/dl/go1.25.13.linux-amd64.tar.gz";
              hash = "sha256-OQQqB46pzuvj7NpKcYjw9bluFKBx0nkjun9AtFboWuM=";
            };

            sourceRoot = ".";

            installPhase = ''
              runHook preInstall

              mkdir -p $out
              cp -r go/* $out/

              runHook postInstall
            '';
          };

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
              go_1_25_13
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
