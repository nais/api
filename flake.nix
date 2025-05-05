{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs =
    { self, ... }@inputs:
    inputs.flake-utils.lib.eachSystem
      [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ]
      (
        system:
        let
          pkgs = import inputs.nixpkgs {
            localSystem = { inherit system; };
            overlays = [
              (
                final: prev:
                let
                  version = "1.24.2";
                  newerGoVersion = prev.go.overrideAttrs (old: {
                    inherit version;
                    src = prev.fetchurl {
                      url = "https://go.dev/dl/go${version}.src.tar.gz";
                      hash = "sha256-ncd/+twW2DehvzLZnGJMtN8GR87nsRnt2eexvMBfLgA=";
                    };
                  });
                  nixpkgsVersion = prev.go.version;
                  newVersionNotInNixpkgs = -1 == builtins.compareVersions nixpkgsVersion version;
                in
                {
                  go = if newVersionNotInNixpkgs then newerGoVersion else prev.go;
                  buildGoModule = prev.buildGoModule.override { go = final.go; };
                }
              )
            ];
          };
          gqlgen = pkgs.buildGoModule rec {
            pname = "gqlgen";
            version = "0.17.68";
            doCheck = false; # TODO: Actually run tests
            src = pkgs.fetchFromGitHub {
              owner = "99designs";
              repo = "gqlgen";
              rev = "v${version}";
              hash = "sha256-zu9Rgxua19dZNLUeJeMklKB0C95E8UVWGu/I5Lkk66E=";
            };
            vendorHash = "sha256-B3RiZZee6jefslUSTfHDth8WUl5rv7fmEFU0DpKkWZk=";
          };
        in
        {
          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              go
              go-tools
              gopls
              gotools

              gofumpt
              protobuf
              protoc-gen-go
              protoc-gen-go-grpc
              sqlc
              # Mise dependencies
              helm
              nodejs_22

              mise
              nodePackages.prettier
            ] ++ [ gqlgen ];
          };

          formatter = pkgs.nixfmt-rfc-style;
        }
      );
}
