{ pkgs ? import <nixpkgs> { } }:
pkgs.buildGoModule rec {
        meta = {
                description = "Flogo";
                homepage = "https://github.com/Gleipnir-Technology/flogo";
        };
        pname = "flogo";
        src = ./.;
        subPackages = [];
        version = "0.0.1";
        # Needs to be updated after every modification of go.mod/go.sum
        vendorHash = "sha256-aaJnH258H1LkXvb22rR3Clg7fKzA/HSmBZUkh1E8aaa=";
}
