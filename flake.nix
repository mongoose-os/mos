{
  description = "Mongoose OS command line tool";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let

      # to work with older version of flakes
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

      # Generate a user-friendly version number.
      version = builtins.substring 0 8 lastModifiedDate;

      # System types to support.
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });

    in
    {

      # Provide some binary packages for selected system types.
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.buildGoModule {
            pname = "mos";
            inherit version;
            # In 'nix develop', we don't need a copy of the source tree
            # in the Nix store.
            src = ./.;

            # This hash locks the dependencies of this package. It is
            # necessary because of how Go requires network access to resolve
            # VCS.  See https://www.tweag.io/blog/2021-03-04-gomod2nix/ for
            # details. Normally one can build with a fake hash and rely on native Go
            # mechanisms to tell you what the hash should be or determine what
            # it should be "out-of-band" with other tooling (eg. gomod2nix).
            # To begin with it is recommended to set this, but one must
            # remember to bump this hash when your dependencies change.
            # vendorHash = pkgs.lib.fakeHash;

            vendorHash = "sha256-PpzclrsG7HYwbowGCbMQFj6+/w+svyMzkxYh2VpR18Y=";

            preBuild = ''
              echo "package version
              const (
                Version = \"${version}\"
                BuildId = \"${self.rev or self.dirtyRev}\"
                BuildTimestamp = \"${lastModifiedDate}\"
              )" > version/version.go
            '';

            nativeBuildInputs = with pkgs; [ pkg-config ];

            buildInputs = with pkgs; [ libusb1 libftdi1 ];
          };
        });

      apps = forAllSystems (system: {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/cli";
          };
          instance = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/instance";
          };
          manager = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/manager";
          };
        }
      );

      # Add dependencies that are only needed for development
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = [ self.packages.${system}.default pkgs.esptool ];
            shellHook = ''
              alias mos=cli
            '';
          };
        });
    };
}
