{ pkgs ? import ./nixpkgs.nix {} }:
with pkgs; buildGoModule rec {
  name = "gpgfs";
  src = nix-gitignore.gitignoreSourcePure [./../.gitignore] ./..;
  vendorSha256 = null;
  doCheck = false; # test requires network
}
