{pkgs, ...}: {
  packages = with pkgs; [gnumake];

  languages.go = {
    enable = true;
    version = "1.27.1";
  };

  tasks = {
    "app:build".exec = "CGO_ENABLED=0 go build -ldflags=\"-X main.commitHash=$(git rev-parse --short HEAD)\" -o=pterodactylBackup main.go";
  };
}
