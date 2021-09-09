{ config, lib, pkgs, ... }:
with builtins;
with lib;

let
  name = "gpgfs";
  cfg = config.services."${name}";
in {
  options = with types; {
    services."${name}" = {
      enable = mkEnableOption "Gpgfs";

      user = mkOption {
        default = name;
        type = str;
        description = "User name to run service from";
      };
      group = mkOption {
        default = name;
        type = str;
        description = "Group name to run service from";
      };

      source = mkOption {
        type = str;
        description = "Source directory with encrypted files";
      };
      target = mkOption {
        type = str;
        description = "Target directory where decrypted filesystem should be mounted";
      };

      config = mkOption {
        type = attrs;
        default = {};
        description = "Gpgfs raw configuration";
      };
    };
  };

  config = mkIf cfg.enable {
    users = {
      extraUsers = mkIf (name == cfg.user)
        {
          ${name} = {
            name = cfg.user;
            group = cfg.group;
            isSystemUser = true;
          };
        };

      extraGroups = optionalAttrs (name == cfg.group)
        { ${name}.name = cfg.group; };
    };

    systemd.services.${name} = {
      enable = true;
      wantedBy = ["multi-user.target"];

      serviceConfig = {
        Type       = "simple";
        Restart    = "on-failure";
        RestartSec = 30;

        User  = cfg.user;
        Group = cfg.group;

        ExecStart = "${pkgs.gpgfs}/bin/${name} -c ${pkgs.writeText "config.yml" (toJSON cfg.config)} mount --source ${cfg.source} --target ${cfg.target}";
      };
    };
  };
}
