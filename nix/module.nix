{ config, lib, pkgs, ... }:
with builtins;
with lib;
let
  cfg = config.system;

  decToOct = let
    toOct = q: a:
      if q > 0
      then toOct
        (q / 8)
        ((toString (mod q 8)) + a)
      else if a == "" then "0" else a;
  in v: toOct v "";

  definedAttrs = filterAttrs (n: v: v != null);
in {
  options.system = with types; {
    secretsSource = mkOption {
      type = path;
      description = "Directory where all GPG encrypted files are stored as a tree on the filesystem of builder";
    };

    secretsTarget = mkOption {
      type = str;
      default = "/run/secrets";
      description = "Temporary directory where gpgfs fuse filesystem should be mounted on the target host";
    };

    secrets = mkOption {
      type = attrsOf (submodule ({ ... }: {
        options = {
          # for more information see 'st_*' in:
          # https://www.gnu.org/software/libc/manual/html_node/Attribute-Meanings.html

          atime = mkOption {
            type = nullOr int;
            default = null;
            description = "Absolute atime attribute of the file";
          };
          ctime = mkOption {
            type = nullOr int;
            default = null;
            description = "Absolute creation time of the file";
          };
          mtime = mkOption {
            type = nullOr int;
            default = null;
            description = "Absolute modification time of the file";
          };
          uid = mkOption {
            type = nullOr int;
            default = null;
            description = "User id which owns the file";
          };
          gid = mkOption {
            type = nullOr int;
            default = null;
            description = "Group id which owns the file";
          };
          mode = mkOption {
            type = nullOr int;
            default = null;
            description = "Decimal (will be converted to octal) mode for the file";
          };
        };
      }));
      default = {};
      description = "Named secrets and their attributes";
    };
  };

  config = let
    store = derivation {
      name = "secrets";
      system = currentSystem;
      builder = with pkgs; writeScript "builder.sh" ''
        #!${pkgs.bash}/bin/bash -e
        export PATH=${lib.makeBinPath [coreutils]}
        mkdir -p $out
        ${concatMapStringsSep "\n"
          (value: ''
            target="$out/${dirOf value}"
            mkdir -p "$target"
            cp ${cfg.secretsSource}/${value}.gpg "$target"
            cp ${writeText "${baseNameOf value}.yml" (toJSON (definedAttrs cfg.secrets.${value}))} "$target/${baseNameOf value}.yml"
          '')
          (attrNames cfg.secrets)}
      '';
    };
  in mkIf (length (attrNames cfg.secrets) > 0) {
    systemd.tmpfiles.rules = ["d ${cfg.secretsTarget} 1700 root root -"];
    services.gpgfs = {
      enable = true;
      source = toString store;
      target = cfg.secretsTarget;
    };
  };
}
