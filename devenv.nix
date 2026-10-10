{...}: {
  languages.go = {
    enable = true;
    version = "1.27.1";
  };

  tasks = {
    "app:test".exec = "go test -race ./...";
    "app:build" = {
      exec = "CGO_ENABLED=0 go build -ldflags=\"-X main.commitHash=$(git rev-parse --short HEAD)\" -o=out/pterodactylBackup main.go";
      after = ["app:test"];
    };
  };
}
