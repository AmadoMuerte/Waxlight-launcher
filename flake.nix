{
  description = "Waxlight Launcher — a modern, lightweight launcher for Vintage Story";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (system: {
        default = nixpkgs.legacyPackages.${system}.callPackage ./nix/waxlight.nix { };
      });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/waxlight";
        };
      });

      checks = forAllSystems (system: {
        default = self.packages.${system}.default;
      });

      devShells = forAllSystems (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          name = "waxlight-dev";
          packages = with nixpkgs.legacyPackages.${system}; [
            go
            nodejs_22
            pkg-config
            gtk3
            webkitgtk_4_1
          ];
        };
      });
    };
}