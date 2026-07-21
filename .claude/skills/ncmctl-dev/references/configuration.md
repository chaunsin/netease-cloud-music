# Configuration and Persistence

Use this reference for config loading, runtime path substitution, Cookie persistence, CookieCloud, logging, and database storage.

## Contents

- [Configuration ownership](#configuration-ownership)
- [Loading and precedence](#loading-and-precedence)
- [Runtime paths](#runtime-paths)
- [Cookie persistence](#cookie-persistence)
- [Database and logs](#database-and-logs)
- [Change checklist](#change-checklist)
- [Testing](#testing)

## Configuration ownership

`config.Config` composes package-owned settings:

```go
type Config struct {
	Version  string
	Log      *log.Config
	Network  *api.Config
	Database *database.Config
}
```

Keep field validation with the package that owns the field. `config.Config.Validate` checks required sections and delegates log and network validation; `database.New` validates the selected driver when opening storage.

Treat `config/config.yaml` as the complete default schema and path source. Do not infer configuration from README examples or invent top-level fields.

## Loading and precedence

Preserve these runtime rules:

- Without `--config`, use the embedded `config/config.yaml`; do not auto-discover `~/.ncmctl/config.yaml`.
- With `--config <file>`, read that exact complete YAML file through Viper.
- On the explicit-file path, apply `NCMCTL_` environment overrides after reading the file. Replace nested separators with underscores, such as `NCMCTL_LOG_LEVEL` and `NCMCTL_NETWORK_TIMEOUT`. The embedded-default path currently bypasses Viper and therefore does not apply these overrides.
- Reject unknown keys through `UnmarshalExact`. Missing `log`, `network`, or `database` sections fail validation; omitted sections do not merge from embedded defaults.
- Apply `--home` by replacing `${HOME}` after loading, then validate and apply `--debug` overrides before logger creation.

`config.GetDefault` returns the package-owned default pointer, not a clone. `ReplaceMagicVariables` mutates its log, Cookie, and database paths and only replaces placeholders still present. Do not assume repeated root-command construction starts from a fresh default; add isolation coverage before changing this behavior.

For a custom file, copy the full current `config/config.yaml`, edit it, and pass it explicitly.

## Runtime paths

Default runtime paths resolve under the selected `<home>`:

| Data | Default path | Owner |
| --- | --- | --- |
| Cookie Jar | `<home>/.ncmctl/cookie.json` | `pkg/cookie`, configured by `api.Config` |
| Rotating log | `<home>/.ncmctl/log/ncm.log` | `pkg/log` |
| Badger data | `<home>/.ncmctl/database/badger/` | `pkg/database` |
| Generated proxy CA | `<home>/.ncmctl/proxy/ca.crt` and `ca.key` | `internal/ncmctl/proxy.go`, `internal/proxy` |

When adding a path containing `${HOME}`, update `ReplaceMagicVariables`, the default YAML, focused tests, and affected user documentation together.

## Cookie persistence

The Cookie Jar is URL-scoped. Pass an explicit parsed URL to both public helpers:

```go
musicURL, err := url.Parse("https://music.163.com")
if err != nil {
	return fmt.Errorf("parse music URL: %w", err)
}

cli.SetCookies(musicURL, cookies)
stored := cli.GetCookies(musicURL)
```

Preserve these behaviors:

- A positive sync interval starts periodic export; a non-positive interval exports when cookies change.
- `Client.Close` calls `Cookie.Close` for a final export. The current exporter logs export failures internally and returns `nil`; do not document caller-visible propagation unless the implementation and tests change.
- An empty configured Cookie filepath makes `api.NewClient` omit `WithFilePath`, so the jar falls back to `./cookie.json`. Treat changes to this working-directory write as a safety-sensitive behavior change.
- Missing parent directories are created with `0700`, and a newly created Cookie file uses `0600` on POSIX. Existing permissions are not repaired by `os.WriteFile`.
- `GetDeviceId` searches `deviceId` and `sDeviceId` across the music, interface, and interface3 domains for XEAPI.
- CookieCloud credentials, imported cookies, and `MUSIC_U` values must stay out of logs, fixtures, and errors.
- The current CookieCloud HTTP client disables TLS certificate verification. Treat this as a security defect, do not describe HTTPS peer identity as authenticated, and do not copy the setting into new clients.

Cookie and CookieCloud login commands currently set cookies on the configured persistent client before account validation. Depending on the interval, persistence can occur immediately or during deferred close even when validation fails. Do not claim validation-before-persistence unless the implementation and fake-transport regression coverage establish it.

## Database and logs

`pkg/database.Database` exposes `Get`, `Set`, `Exists`, `Increment`, `Del`, and `Close`. `database.New` currently supports Badger only. Preserve scrobble's per-account key construction, daily counters, TTL behavior, and close-error reporting when changing storage.

The root command creates `pkg/log.Logger` after config and CLI overrides. Close it at the command boundary; remember that Cobra skips `PersistentPostRunE` when `RunE` fails, so resources created inside a command need their own cleanup path.

## Change checklist

When changing configuration or persistence:

1. Update the owning struct, validation, embedded YAML, and `${HOME}` replacement together.
2. Cover exact-file loading, unknown and missing fields, environment overrides, invalid values, and path substitution.
3. Cover initialization, periodic or final flush, restrictive creation modes, and cleanup errors for persisted data.
4. Update user documentation only when schema, flags, defaults, paths, errors, or safety boundaries change.

## Testing

Use offline, focused checks:

```bash
go test ./config ./pkg/cookie ./pkg/cookiecloud ./api ./pkg/database ./pkg/log
go test ./internal/ncmctl
go test -race ./pkg/cookie # when changing periodic sync or close behavior
```

Use temporary directories and `t.Setenv`; never read or overwrite the user's real Cookie, log, database, or proxy CA paths in tests.
