{
	description = "Flogo";

	inputs = {
		nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
		flake-utils.url = "github:numtide/flake-utils";
	};

	outputs = { self, nixpkgs, flake-utils }:
		flake-utils.lib.eachDefaultSystem (system:
			let
				pkgs = nixpkgs.legacyPackages.${system};
				package = import ./default.nix { inherit pkgs; };
			in
			{
				packages.default = package;
				packages.postgrid-test = package;

				# Development shell configuration
				devShells.default = pkgs.mkShell {
					buildInputs = [
						pkgs.air
						pkgs.go
						pkgs.gotools
						pkgs.lefthook
					];
				};
			}
		);
}
