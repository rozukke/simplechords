{
  description = "Development environment for simplechords";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.fluidsynth
              pkgs.soundfont-fluid
            ];

            # The application also honours a user-supplied SOUNDFONT value.
            shellHook = ''
              export SOUNDFONT="''${SOUNDFONT:-${pkgs.soundfont-fluid}/share/soundfonts/FluidR3_GM2-2.sf2}"
            '';
          };
        });
    };
}
