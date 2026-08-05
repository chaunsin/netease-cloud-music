# CLI Development

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
2. `PersistentPreRunE` loads and validates runtime configuration, applies CLI overrides, and initializes the logger. Load `references/configuration.md` through the skill routing table when changing this step.
3. The selected command validates arguments and runs its operation.
4. `Root.Execute` closes the logger after Cobra returns, including command-error paths. Keep critical root cleanup outside `PersistentPostRunE`, because Cobra skips post-run hooks when `RunE` returns an error.

`Root` owns address-stable logger storage before registering the command tree. Constructors receive that address, and `PersistentPreRunE` initializes the same storage after configuration and CLI overrides are final. Command methods log through their direct `c.l` field; do not copy a nil logger during construction, add package-level fallbacks, or reach through nested `root` fields to find the active logger. `log.Default` remains an alias for lower-level packages that still use package-level logging.

## Command map

| Command | Implementation | External effects and important dependencies |
| --- | --- | --- |
| `login phone` | `login_phone.go` | Sends SMS or submits a password, validates the resulting account, persists cookies |
| `login cookie` | `login_cookie.go` | Imports Netscape/JSON/header cookies, requires `MUSIC_U`, sets them on the persistent client, then validates the account |
| `login cookiecloud` | `login_cookiecloud.go` | Contacts CookieCloud, sets matching NetEase cookies on the persistent client, then validates the account |
| `login qrcode` | `login_qrcode.go` | Calls live login endpoints, writes a temporary `qrcode.png`, removes it after success |
| `logout` | `logout.go` | Calls the logout endpoint, then removes the default Cookie and XEAPI state; `--clear-anonymous-token` also removes the anonymous token |
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
defer closeAPIClient(ctx, cli, c.l)

request := weapi.New(cli)
```

For login-required work, verify authentication before the first account mutation. Add token refresh only where the command's control flow requires it, and make sure early returns do not skip cleanup. `closeAPIClient` triggers the final Cookie export and logs errors returned by `Client.Close`; prefer it over a bare ignored close. The Cookie exporter currently logs its own export failure internally, so do not promise stronger error propagation without changing and testing that contract.

## Scheduled tasks

`task` has two modes:

- With no selector, or with `--runAll`, it registers all three jobs.
- With any of `--sign`, `--partner`, or `--scrobble`, it registers only the selected jobs.

The scheduler creates fresh command instances and copies embedded option structs into them. When adding a scheduled option, update the `TaskOpts` embedding/fields, flag binding, validation, and command-copy path together. Keep cron parsing in validation and timezone loading before job registration.

`task` is long lived, but its current foreground wait is signal-only: `nohup.Daemon` does not observe the Cobra execution context. On SIGINT, SIGQUIT, or SIGTERM it calls `job.Stop()` but does not wait on the returned context, so in-flight jobs are not guaranteed to finish before deferred client/logger cleanup. Do not describe programmatic cancellation or graceful cron draining as implemented. Any lifecycle change needs deterministic cancellation, signal, in-flight-job, and cleanup-order tests.

## Concurrent file commands

`download`, `cloud`, and `ncm` use weighted semaphores and goroutines.

- Acquire before launching the goroutine and always release in a defer.
- Preserve the final wait or replace it with an equally explicit structured-concurrency mechanism.
- Do not share mutable per-file buffers between workers.
- Protect every other shared counter or state value with atomics, locks, or an existing synchronization abstraction.
- Close files and response bodies on every path, including checksum, tag, and rename errors.
- Keep partial output in a temporary file until validation succeeds.
- Return setup/cancellation errors; per-item failures may be logged and counted when the command intentionally continues.

## Command-specific traps

### `logout` state cleanup

After a successful remote logout, call `api.Client.Close` exactly once before deleting local state: `Close` synchronizes XEAPI, anonymous-token, and Cookie data, so a later deferred close can recreate files that were just removed. Always remove `cookie.json` and `xeapi.yaml` under `Network.HomeDir`; remove `anonymous_token` only when `--clear-anonymous-token` is set. Preserve `header.yaml`, logs, databases, and any custom Cookie path selected through configuration.

### `ncm --tag`

The existing flag is historically inverted: the default `false` writes tags, while passing `--tag` disables tag writing. Keep the documentation explicit. A future semantic cleanup is a behavior change and needs compatibility handling rather than a documentation-only rename.

### `download --tag`

The flag is retained for compatibility but tag writing is not implemented. Do not advertise it as functional until the implementation and focused metadata tests exist.

### `curl --method`

`--method` selects the exported Go API method name and overrides the positional method name; it is not an HTTP verb. The request type is obtained by reflection, JSON decoding rejects unknown fields, and unknown `--kind` values must fail rather than fall back to WEAPI.

### `crypto decrypt`

Direct input defaults to an EAPI request or an XEAPI response; HAR input defaults to both request and response. A direct XEAPI request must be a URL-encoded `B/S/R` form selected with `--target request`. Decrypt `R` unconditionally with the static key, decrypt `B` only with the explicitly supplied dynamic/session key, and strictly validate the outer `S` frame while preserving it without claiming it is recoverable: decryption requires the request's X25519 private key.

Keep dynamic/session keys explicit. Do not infer them from HAR response headers or persisted `xeapi.yaml`, and never log the value. A missing or incorrect key, incomplete request fields, or malformed XEAPI form is a per-request partial failure: retain recoverable `R` metadata, write the complete JSON, continue HAR response and later-entry processing, then return a non-zero aggregate error. Parse only the side selected by `--target`; HAR structure and response-decryption failures remain fail-fast.

Ciphertext in JSON must remain byte-recoverable. Preserve direct hex/base64 input in that representation, encode raw direct bytes and HAR response bytes as Base64, and set `ciphertextEncoding` on every populated `ciphertext` field. WEAPI still cannot be decrypted without its random client key, and the direct Linux/API branches are not implemented. Do not advertise accepted `--kind` strings as implemented capabilities in both subcommands.

### `proxy` XEAPI session sources

`proxy --xeapi-session-id` and `--xeapi-session-key` are a required pair. Limit the ID to 1024 bytes. Treat the key as raw ASCII bytes, require an AES length of 16, 24, or 32, never infer hex encoding, and keep the help warning that flags are visible in shell history and process arguments. `--xeapi-state-file` reads only an explicitly named regular canonical YAML file, bounded to 1 MiB, and requires a complete `session.id/key`; missing or expired public-key state is irrelevant to passive decryption. Do not discover the global-home `xeapi.yaml` implicitly.

Pass state-file seeds before command-line seeds so the latter replaces the same ID while different IDs coexist. Runtime response-header learning supersedes both and remains in memory only. Every explicit source validates independently; a valid flag pair must not hide an invalid file or vice versa. Do not print keys in startup output, validation errors, or diagnostics. These rules are proxy-specific and must not weaken the explicit-key-only contract of `crypto decrypt`.

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
