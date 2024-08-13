{
  description = "Example Go development environment for Zero to Nix";

  # Flake inputs
  inputs = { nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable"; };

  # Flake outputs
  outputs = { self, nixpkgs }:
    let
      # Systems supported
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];

      # Helper to provide system-specific attributes
      forAllSystems = f:
        nixpkgs.lib.genAttrs allSystems (system:
          f {
            pkgs = import nixpkgs {
              inherit system;
              overlays = [
                (final: prev: {
                  go = prev.go_1_22.overrideAttrs (old:
                    old // {
                      version = "1.22.6";
                      src = prev.fetchurl {
                        url = "https://go.dev/dl/go1.22.6.src.tar.gz";
                        hash =
                          "sha256-/tcgZ45yinyjC6jR3tHKr+J9FgKPqwIyuLqOIgCPt4Q=";
                      };
                    });

                })
              ];
              # crossSystem = { config = "aarch64-unknown-linux-gnu"; };
            };
          });
    in {
      # Development environment output
      devShells = forAllSystems ({ pkgs }: {
        default = pkgs.mkShell {
          # The Nix packages provided in the environment
          packages = with pkgs; [
            go
            gotools # Go tools like goimports, godoc, and others
            gopls
            buf-language-server
          ];
        };
      });
    };
}
