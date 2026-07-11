---
name: ncmctl-dev
description: >
  NetEase Cloud Music CLI tool (ncmctl) development guide. Use this skill when working with
  the netease-cloud-music codebase, including ncmctl commands, API integration, crypto,
  NCM file decryption, daily tasks, music download, cloud upload, HTTP(S) proxy monitoring, or any Go code in this
  repository. Trigger on mentions of ncmctl, 网易云音乐 CLI, NetEase Cloud Music API,
  weapi/eapi encryption, .ncm file format, cloud music daily tasks (sign/partner/scrobble),
  or when modifying, debugging, or extending any part of this project.
---
# ncmctl Development Guide

ncmctl is a Go CLI tool for NetEase Cloud Music providing login, daily tasks, music download, cloud upload, NCM file decryption, and HTTP(S) API monitoring.

## Project Structure

```
cmd/ncmctl/main.go       # CLI entry point
internal/ncmctl/          # CLI command implementations (cobra commands)
internal/proxy/           # goproxy MITM, capture, protocol parsing, redaction
api/                      # API client layer
  ├── api.go              # Core Client: HTTP, encryption, cookie persistence
  ├── weapi/              # Web/Mini-program API (recommended, most complete)
  ├── eapi/               # PC/Mobile API
  ├── linux/              # Linux client API
  └── types/              # Shared request/response types
pkg/
  ├── crypto/             # AES-CBC/ECB, RSA, weapi/eapi encryption
  ├── cookie/             # Cookie persistence and sync
  ├── cookiecloud/        # CookieCloud browser extension support
  ├── ncm/                # NCM file decryption + audio tag handling
  ├── database/           # Badger key-value store wrapper
  ├── log/                # Structured logging (lumberjack rotation)
  └── utils/              # General utilities
config/                   # Config structs + default config.yaml
```

## Build & Test

```bash
make build                # Build binary → ./ncmctl
make install              # Install to $GOPATH/bin
go test -v ./...          # Run all tests
go test -v -run TestName ./example/  # Run single test
make build-image          # Build Docker image
```

Requires Go >= 1.25.0. Tests needing login require cookie file or should be skipped.

## CLI Commands

| Command      | Login | Description                            |
| ------------ | ----- | -------------------------------------- |
| `login`    | No    | Phone/Cookie/CookieCloud/QR code login |
| `logout`   | No    | Clear stored credentials               |
| `task`     | Yes   | Run all daily tasks on cron schedule   |
| `sign`     | Yes   | YunBei + VIP daily check-in            |
| `partner`  | Yes   | Music partner auto-evaluation          |
| `scrobble` | Yes   | Scrobble 300 songs daily               |
| `download` | Yes   | Download songs/albums/playlists        |
| `cloud`    | Yes   | Upload music to cloud disk             |
| `ncm`      | No    | Decrypt .ncm → .mp3/.flac             |
| `crypto`   | No    | Encrypt/decrypt API parameters         |
| `curl`     | No    | Invoke API methods directly            |
| `proxy`    | No    | Monitor NetEase HTTP(S) API traffic    |

## Adding a New CLI Command

1. Create `internal/ncmctl/<command>.go` implementing a struct with `root`, `cmd`, `opts`, `l` fields
2. Implement `New<Command>(root *Root, l *log.Logger)` constructor
3. Define cobra command with `Use`, `Short`, `Example`
4. Add flags via `addFlags()` method
5. Implement `validate()` and `execute(ctx, args)` methods
6. For login-required commands: create API client, check `request.NeedLogin(ctx)`, defer `request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})`
7. Register in `internal/ncmctl/ncmctl.go`: `c.Add(New<Command>(c, c.l).Command())`

Pattern for login-required commands:

```go
cli, err := api.NewClient(c.root.Cfg.Network, c.l)
if err != nil { return fmt.Errorf("NewClient: %w", err) }
defer cli.Close(ctx)
request := weapi.New(cli)
if request.NeedLogin(ctx) { return fmt.Errorf("need login") }
defer func() {
    refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
    if err != nil || refresh.Code != 200 {
        log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
    }
}()
```

## Adding a New API Endpoint

1. Create file in `api/weapi/` or `api/eapi/`
2. Define request/response structs (request fields use json tags)
3. Implement method on API struct calling `a.client.Request(ctx, url, req, resp, opts...)`
4. Set correct `CryptoMode` via option: `api.WithCryptoMode(api.CryptoModeWEAPI)`
5. Default crypto mode is weapi; eapi uses `CryptoModeEAPI`

API call flow:

```go
cli := api.New(cfg)
weapiClient := weapi.New(cli)
resp, err := weapiClient.SomeMethod(ctx, &weapi.SomeMethodReq{...})
cli.Close(ctx)
```

## Encryption Modes

| Mode                | Algorithm                    | Use Case         |
| ------------------- | ---------------------------- | ---------------- |
| `CryptoModeWEAPI` | AES-CBC double encrypt + RSA | Web/Mini-program |
| `CryptoModeEAPI`  | AES-ECB                      | PC/Mobile        |
| `CryptoModeLinux` | AES-ECB                      | Linux client     |
| `CryptoModeAPI`   | None                         | Basic API        |

Core functions in `pkg/crypto/crypto.go`: `WeApiEncrypt()`, `EApiEncrypt()`, `LinuxApiEncrypt()`, `EApiDecrypt()`.

## Configuration

- Config file: `~/.ncmctl/config.yaml` (optional, uses defaults if absent)
- Cookie storage: `~/.ncmctl/cookie.json` (auto-persisted, 3s interval)
- Database: `~/.ncmctl/database/badger/` (scrobble dedup records)
- Logs: `~/.ncmctl/log/ncm.log`
- Proxy CA: `~/.ncmctl/proxy/ca.crt` and `ca.key`
- Env var prefix: `NCMCTL_` (e.g., `NCMCTL_NETWORK_DEBUG=true`)
- Magic variable: `${HOME}` replaced at runtime

## Download Quality Levels

| Level    | Aliases | Format  |
| -------- | ------- | ------- |
| standard | 128     | 128kbps |
| higher   | 192     | 192kbps |
| exhigh   | HQ, 320 | 320kbps |
| lossless | SQ      | FLAC    |
| hires    | HR      | Hi-Res  |

## Key Dependencies

| Package                 | Purpose           |
| ----------------------- | ----------------- |
| `spf13/cobra`         | CLI framework     |
| `elazarl/goproxy`     | HTTP(S) MITM proxy |
| `go-resty/resty/v2`   | HTTP client       |
| `dgraph-io/badger/v4` | Local KV database |
| `robfig/cron/v3`      | Cron scheduling   |
| `spf13/viper`         | Config management |

## Important Notes

- Cookie persistence is interval-based (3s); unclean shutdown may lose recent cookies
- Scrobble dedup data in `~/.ncmctl/database/` should not be deleted
- Directory depth limit is 3 for cloud upload and NCM decryption
- Cloud upload max file size: 500MB
- Download parallelism max: 20; Cloud upload parallelism max: 10
- The `task` command runs as a long-lived service; use Ctrl+C to stop
- The `proxy` command runs until SIGINT/SIGTERM; capture blocks go to stdout and startup/errors go to stderr
- Proxy CA private keys must remain mode `0600` on POSIX and use a current-user protected ACL on Windows; never use goproxy's public built-in CA key
- Proxy observation failures must not alter forwarded requests or responses; unstructured/non-UTF-8 bodies fail closed unless sensitive output is explicitly enabled, and a full output queue must report `CAPTURE_DROPPED` instead of blocking traffic
- Passive WEAPI/XEAPI requests may be marked unsupported because their client-side/session keys are unavailable
- Sign-in reward auto-claim (`--sign.automatic`) has ban risk, disabled by default
- Scrobble (刷歌) currently has high risk of account ban due to strict risk control

For detailed command usage and API reference, read `references/commands.md` and `references/api-guide.md`.
