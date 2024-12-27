build:
	CGO_ENABLED=0 go build -ldflags="-X main.commitHash=$(shell git rev-parse --short HEAD)" -o=pterodactylBackup main.go
