{ pkgs ? import <nixpkgs> {}, ... }:
with pkgs;
buildGoModule rec {
  name = "gpgfs";
  version = "master";
  src = nix-gitignore.gitignoreSourcePure [./../.gitignore] ./..;
  vendorSha256 = null;

  buildInputs = [makeWrapper fuse];
  ldflags = [
    "-X" "git.backbone/corpix/gpgfs/pkg/meta.Version=${version}"
  ];

  postInstall = ''
    wrapProgram $out/bin/${name} --prefix PATH : ${lib.makeBinPath [fuse]}
  '';
}
