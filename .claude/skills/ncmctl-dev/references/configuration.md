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
- Apply `--home` to `Network.HomeDir` and replace `${HOME}` after loading, then validate and apply `--debug` overrides before logger creation. API-managed state is rooted at `<home>/.ncmctl/`.

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

- A positive sync interval starts periodic export. A non-positive interval attempts an export after every `SetCookies` call accepted before shutdown, even when the underlying Jar ignores the update.
- `Client.Close` calls `Cookie.Close` for a final export and returns that export error. If its context is canceled first, the shared close continues in the background; a later `Close` call can wait for the same final result.
- An empty configured Cookie filepath makes `api.NewClient` omit `WithFilePath`, so the jar falls back to `./cookie.json`. Treat changes to this working-directory write as a safety-sensitive behavior change.
- The persistent Jar uses `publicsuffix.List` by default. Callers must explicitly pass `WithPublicSuffixList(nil)` to opt out of public-suffix protection; the lower-level in-memory `New(nil)` retains the standard-library nil-list behavior.
- Missing parent directories are created with `0700`. Exports write and sync a same-directory temporary file, then replace the target with `os.Rename`. Same-directory rename is atomic on Unix; Windows follows the standard library's platform semantics and does not promise atomicity. POSIX targets use mode `0600`, repairing an existing Cookie file's broader mode.
- Loading requires every field emitted by the original persistence format; only `Quoted` may be absent for backward compatibility. It validates each persisted entry and its containing bucket, including the entry ID, canonical domain, absolute path, HostOnly/IP relationship, session expiration, and public-suffix scope.
- Valid origin-derived buckets required by an explicit nil or safely isolated custom Public Suffix List are preserved. Former nil-PSL buckets migrate when HostOnly makes the origin explicit or the current list is the default `publicsuffix.List`; ambiguous custom-list collisions remain in their original bucket. Migration collisions and other invalid files fail initialization without rewriting the source. Sequence numbers are compacted deterministically, and expired persistent cookies are discarded.
- Session cookies intentionally survive process restarts. This is the persistence extension's explicit difference from an in-memory browser session; do not drop them during load or export.
- A Cookie file has a single-writer contract. Goroutines sharing one `Cookie` are synchronized, and the final export includes `SetCookies` calls accepted before shutdown marks the jar as closing; later calls are ignored. Separate processes do not lock or merge the same file, so the last completed replacement wins.
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
