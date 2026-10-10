# AGENTS.md

pterodactylBackup is a Go CLI, usually run from cron. It creates a backup of every server on a Pterodactyl panel and rotates old backups so each server stays under its backup limit.

## Commands

The Go toolchain comes from devenv (`devenv.nix`). Run commands inside `devenv shell`, or use `.devenv/profile/bin/go` directly.

- `devenv tasks run app:test`: runs `go test -race ./...`.
- `devenv tasks run app:build`: runs `app:test` first, then builds a static binary to `out/pterodactylBackup` with the commit hash embedded. `out/` is git-ignored.
- `go vet ./...` and `gofmt` must be clean.

## Layout

- `main.go`: the entry point; sets up logging, loads the config, lists servers and runs the batch.
- `config.go`: `Config` and `loadConfig`.
- `backup.go`: the worker pool and the backup flow (`createBackupBatch`, `skipReason`, `backupWorker`, `newestBackupAge`, `backupServer`, `waitForBackup`), in call order.
- `pterodactylapi/adminapi`: the application API (`/api/application`). Only used to list servers.
- `pterodactylapi/clientapi`: the client API (`/api/client`) for backups, including `PruneBackups`.
- `pterodactylapi/structs`: JSON types, including the generic `Response[T]` for list responses.
- `internal/fakepanel`: an in-memory fake of the panel API, used by all tests.

## Configuration

The program reads `app.yaml` from the directory of the executable; see `app.example.yaml`. Unknown keys are an error, and environment variables are not read.

- `api_key`
- `base_url`
- `log_level`: `debug`, `info`, `warn` or `error`. Default `info`.
- `min_backup_interval_minutes`: default 720. 0 turns the check off.

The API key must be an account key (`ptlc_…`) of a root admin. That key works for both APIs, while an application key (`ptla_…`) is rejected by the client API.

## Backup behaviour (keep these invariants)

1. **Skip servers** that:
   - allow no backups (limit 0),
   - are suspended,
   - have a `status` set (installing, restoring and so on; the panel answers 409 for them),
   - or whose newest successful backup is younger than `min_backup_interval_minutes`.
   
   Skips are logged and are not failures.
2. **Keep one slot free:** the panel refuses new backups at the limit. Before creating, if the server is full, delete the oldest backup(s) down to `limit - 1`.
3. **Create the backup**, then poll until `completed_at` is set.
   - Temporary errors are retried until `backupTimeout`.
   - A 404 stops the wait at once.
4. **If the new backup failed**, delete it and count the server as failed.
5. **If it succeeded**, delete the oldest backups down to `max(limit - 1, 1)`. Never delete the new backup.
6. **Never delete** locked or in-progress backups. `PruneBackups` deletes nothing and returns `ErrNoRoom` when it can't reach its target.
   - Before creating, `ErrNoRoom` is a failure.
   - After a successful backup it only produces a warning.
7. **Failures:** one server failing must never stop the others or hang the run. The process exits 1 if any server failed.

## Pterodactyl API facts (checked against the panel source)

- **What counts toward the backup limit:** backups that are successful or still in progress (`completed_at` null). Failed backups don't count.
- **Failed backups** have `completed_at` set and `is_successful` false. In-progress backups have `completed_at` null.
- **Responses:** creating a backup at the limit returns 400 (`TooManyBackupsException`). Create returns 200 and delete returns 204.
- **Paging:**
  - Lists are paged; use `meta.pagination.total_pages`.
  - The backup list allows `per_page` up to 50 and is **not sorted**, so sort by `created_at` in code.
  - The server list is requested with `sort=id`.
- **Throttle:** the panel allows 2 backups per server per 10 minutes, and deleted backups still count toward it. A 429 is reported as a failure.
- **Nullable strings:** `external_id` and `limits.threads` can be strings or null, so they are `*string`.

## Conventions

- **Logging:** use `log/slog` only (text handler on stderr). Messages are short and lowercase, and details go in attributes. Use the existing keys:
  - `worker`, `server`, `server_id`, `backup`
  - `error`, `reason`
  - counts such as `succeeded`, `failed`, `skipped`
  
  Use Debug for progress noise, Info for actions, Warn for recoverable problems, and Error for failures.
- **Errors:** describe what was being done, lowercase, then wrap the cause: `fmt.Errorf("create backup: %w", err)`. Don't use "failed to" or "cannot" in errors. Log an error once, where it's handled. Errors after a backup is created include its UUID.
- **Tests:**
  - Use table-driven tests against `internal/fakepanel`, standard library only.
  - When the backup flow changes, assert the panel's ordered `Events()` and remaining `Backups()`.
  - Keep the fake true to the real panel: unsorted lists, paging, limit enforcement, locked backups, delayed completion.
  - Tests shorten `pollInterval` and `backupTimeout` in `TestMain`.
- **Git:** use [Conventional Commits](https://www.conventionalcommits.org/): `<type>(<optional scope>): <description>` in lowercase, e.g. `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `build:` or `chore:`. Mark breaking changes, such as config format changes, with `!` and a `BREAKING CHANGE:` footer.
