# CLI Development Reference

This reference maps ncmctl's public commands to their implementation and development constraints. For the current flag tables and user examples, read the repository-root `skills/ncmctl/references/commands.md`; do not duplicate those tables here.

## Contents

- [Root lifecycle](#root-lifecycle)
- [Command map](#command-map)
- [Adding or changing a command](#adding-or-changing-a-command)
- [API client lifecycle](#api-client-lifecycle)
- [Scheduled tasks](#scheduled-tasks)
- [Concurrent file commands](#concurrent-file-commands)
- [Command-specific traps](#command-specific-traps)
- [Testing](#testing)
- [Documentation checklist](#documentation-checklist)

## Root lifecycle

`internal/ncmctl/ncmctl.go` owns the root Cobra command and top-level command registration. Nested commands are registered by their nearest parent constructor, such as `login.go` and `crypto.go`.

Execution order:

1. Cobra parses persistent flags (`--debug`, `--config`, and `--home`).
2. `PersistentPreRunE` uses the embedded default config or reads the exact complete `--config` path, replaces `${HOME}`, validates required sections and nested network/log settings, and initializes the logger.
3. The selected command validates arguments and runs its operation.
4. On a successful command path, `PersistentPostRunE` closes the logger. Cobra skips post-run hooks when `RunE` returns an error, so critical cleanup cannot rely on this hook.

Without `--config`, the program does not auto-discover `~/.ncmctl/config.yaml`; it uses the embedded `config/config.yaml`.

## Command map

| Command | Implementation | External effects and important dependencies |
| --- | --- | --- |
| `login phone` | `login_phone.go` | Sends SMS or submits a password, validates the resulting account, persists cookies |
| `login cookie` | `login_cookie.go` | Imports Netscape/JSON/header cookies, requires `MUSIC_U`, validates in memory, then persists |
| `login cookiecloud` | `login_cookiecloud.go` | Contacts a CookieCloud server, validates matching NetEase cookies in memory, then persists |
| `login qrcode` | `login_qrcode.go` | Calls live login endpoints, writes a temporary `qrcode.png`, removes it after success |
| `logout` | `logout.go` | Calls the logout endpoint, then removes `<home>/.ncmctl/cookie.json` |
| `task` | `task.go` | Registers `sign`, `partner`, and/or `scrobble` in a long-running cron service |
| `sign` | `sign.go` | Performs YunBei and VIP account actions; optional automatic reward claims |
| `partner` | `partner.go` | Reports plays and submits music-partner evaluations after randomized waits |
| `scrobble` | `scrobble.go` | Sends play logs and writes Badger dedup/daily-counter records |
| `download` | `download.go` | Resolves resources, downloads media, and verifies MD5; the compatibility `--tag` flag currently has no effect |
| `cloud` | `cloud.go` | Reads local audio, uploads it, submits metadata, polls transcoding, and publishes it |
| `ncm` | `ncm.go` | Reads local NCM files, decodes audio, and writes MP3/FLAC output |
| `crypto` | `crypto*.go` | Local protocol encryption/decryption; HAR input may contain secrets |
| `curl` | `curl.go` | Reflectively invokes an API wrapper method and may contact live services |
| `proxy` | `proxy.go`, `internal/proxy/` | Starts an HTTP(S) proxy, manages a CA, and emits redacted captures |

Treat the account-changing and live-network boundaries above as part of tests and documentation. A command being callable without a login check does not imply that its selected API method is anonymous or side-effect free.

## Adding or changing a command

Follow the nearest existing command rather than requiring one rigid shape. Most top-level commands use this flow:

1. Define an options struct and command struct in `internal/ncmctl/<command>.go`.
2. Construct a `*cobra.Command` with accurate `Use`, `Short`, and `Example` text.
3. Bind flags in `addFlags` and validate all cross-field constraints before side effects.
4. Use `RunE` so errors reach the root command and produce a non-zero exit.
5. Create only the clients and resources the command needs.
6. Register a top-level command in `Root.New()`; register a nested command in the nearest parent constructor.
7. Add focused tests for parsing, validation, output, cancellation, and error propagation.
8. Build the binary and compare `ncmctl <command> --help` with the user documentation.

Use `RunE` so command and scheduler failures remain machine-visible. `partner` follows this contract; preserve it when changing its execution path.

## API client lifecycle

Runtime commands should use the error-returning constructor:

```go
cli, err := api.NewClient(c.root.Cfg.Network, c.l)
if err != nil {
	return fmt.Errorf("NewClient: %w", err)
}
defer closeAPIClient(ctx, cli)

request := weapi.New(cli)
```

For login-required work, verify authentication before the first account mutation. Add token refresh only where the command's control flow requires it, and make sure early returns do not silently skip required cleanup. `closeAPIClient` already records a final Cookie flush error and should be preferred over a bare ignored `cli.Close(ctx)` inside this package.

## Scheduled tasks

`task` has two modes:

- With no selector, or with `--runAll`, it registers all three jobs.
- With any of `--sign`, `--partner`, or `--scrobble`, it registers only the selected jobs.

The scheduler creates fresh command instances and copies embedded option structs into them. When adding a scheduled option, update the `TaskOpts` embedding/fields, flag binding, validation, and command-copy path together. Keep cron parsing in validation and timezone loading before job registration.

`task` is long lived. Changes must preserve context cancellation, cron shutdown, logger/client cleanup, and the no-duplicate-registration behavior.

## Concurrent file commands

`download`, `cloud`, and `ncm` use weighted semaphores and goroutines.

- Acquire before launching the goroutine and always release in a defer.
- Preserve the final wait or replace it with an equally explicit structured-concurrency mechanism.
- Do not share mutable per-file buffers between workers.
- Close files and response bodies on every path, including checksum, tag, and rename errors.
- Keep partial output in a temporary file until validation succeeds.
- Return setup/cancellation errors; per-item failures may be logged and counted when the command intentionally continues.

## Command-specific traps

### `ncm --tag`

The existing flag is historically inverted: the default `false` writes tags, while passing `--tag` disables tag writing. Keep the documentation explicit. A future semantic cleanup is a behavior change and needs compatibility handling rather than a documentation-only rename.

### `download --tag`

The flag is retained for compatibility but tag writing is not implemented. Do not advertise it as functional until the implementation and focused metadata tests exist.

### `curl --method`

`--method` selects the exported Go API method name and overrides the positional method name; it is not an HTTP verb. The request type is obtained by reflection, JSON decoding rejects unknown fields, and unknown `--kind` values must fail rather than fall back to WEAPI.

### `crypto decrypt`

Direct request decryption currently supports EAPI. WEAPI cannot be decrypted without its random client key, and the direct Linux/API branches are not implemented. Do not advertise accepted `--kind` strings as implemented decrypt capabilities.

### Credential flags

Phone password and CookieCloud UUID/password are command-line flags. There is no built-in interactive password prompt or dedicated credential environment variable for these ncmctl subcommands. Do not document one unless the implementation is added and tested.

## Testing

Safe starting points for CLI mechanics:

```bash
go test ./internal/ncmctl
go test -run TestName ./internal/ncmctl
go build -o /tmp/ncmctl-doc-check ./cmd/ncmctl
/tmp/ncmctl-doc-check COMMAND --help
```

Do not execute login, daily-task, upload, download, `curl`, or live API tests as a smoke test. Tests under `api/weapi` and `api/eapi` make real requests without an integration tag; tests under `example/` require `-tags=integration` and are also live.

## Documentation checklist

When syntax, defaults, output, errors, persistence paths, or safety/side-effect boundaries change, update the affected surfaces:

- `README.md`
- `skills/ncmctl/SKILL.md` when the quick reference or safety guidance changes
- `skills/ncmctl/references/commands.md`
- `skills/ncmctl/references/install-and-login.md` for setup/authentication changes
- this file only for implementation or lifecycle contracts that future development must preserve

Verify examples against a freshly built binary rather than relying on remembered Cobra output.
