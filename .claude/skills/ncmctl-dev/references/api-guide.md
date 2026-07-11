# API Development Guide

## Table of Contents

- [Architecture](#architecture)
- [API Client Core](#api-client-core)
- [Adding a New API Endpoint](#adding-a-new-api-endpoint)
- [Encryption Details](#encryption-details)
- [Cookie Management](#cookie-management)
- [Configuration System](#configuration-system)
- [Database Layer](#database-layer)
- [Error Handling Patterns](#error-handling-patterns)

## Architecture

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│  CLI Layer   │────▶│  API Layer  │────▶│  NetEase API │
│ (cobra/cmd)  │     │ (weapi/etc) │     │  (remote)    │
└──────────────┘     └─────────────┘     └──────────────┘
                            │
                     ┌──────┴──────┐
                     │  api.Client │
                     │  (core)     │
                     └──────┬──────┘
                            │
              ┌─────────────┼─────────────┐
              │             │             │
        ┌─────┴────┐ ┌─────┴────┐ ┌─────┴────┐
        │  crypto  │ │  cookie  │ │  resty   │
        │ (encrypt)│ │(persist) │ │ (http)   │
        └──────────┘ └──────────┘ └──────────┘
```

## API Client Core

The `api.Client` in `api/api.go` is the central HTTP client handling:

- Request dispatch via `resty`
- Automatic parameter encryption/decryption based on `CryptoMode`
- Cookie jar with periodic disk persistence
- Response parsing into typed structs

### Creating a Client

```go
cfg := &config.Network{
    Timeout: 60 * time.Second,
    Retry:   3,
    Cookie: config.Cookie{
        Filepath: "${HOME}/.ncmctl/cookie.json",
        Interval: 3 * time.Second,
    },
}
cli, err := api.NewClient(cfg, logger)
if err != nil { /* handle */ }
defer cli.Close(ctx) // ensures cookie flush
```

### Making API Calls

```go
weapiClient := weapi.New(cli)
resp, err := weapiClient.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
if err != nil { /* handle */ }
if resp.Code != 200 { /* handle API error */ }
```

## Adding a New API Endpoint

### Step 1: Create the API file

Create a new file in `api/weapi/` (or `api/eapi/`):

```go
package weapi

type MyNewFeatureReq struct {
    Id string `json:"id"`
}

type MyNewFeatureResp struct {
    Code int    `json:"code"`
    Data string `json:"data"`
}

func (a *Api) MyNewFeature(ctx context.Context, req *MyNewFeatureReq) (*MyNewFeatureResp, error) {
    var resp MyNewFeatureResp
    _, err := a.client.Request(ctx, "https://music.163.com/weapi/my/new/feature", req, &resp,
        api.WithCryptoMode(api.CryptoModeWEAPI),
    )
    if err != nil {
        return nil, fmt.Errorf("MyNewFeature: %w", err)
    }
    return &resp, nil
}
```

### Step 2: Choose the right CryptoMode

| Mode | When to use |
|------|-------------|
| `CryptoModeWEAPI` | Web/mini-program endpoints (most common) |
| `CryptoModeEAPI` | PC/mobile endpoints (uses `/eapi/` prefix) |
| `CryptoModeLinux` | Linux client endpoints |
| `CryptoModeAPI` | No encryption needed |

### Step 3: Use in CLI command

```go
cli, err := api.NewClient(c.root.Cfg.Network, c.l)
defer cli.Close(ctx)
request := weapi.New(cli)
if request.NeedLogin(ctx) { return fmt.Errorf("need login") }
resp, err := request.MyNewFeature(ctx, &weapi.MyNewFeatureReq{Id: "123"})
```

## Encryption Details

### weapi Encryption (`pkg/crypto/crypto.go`)

1. Generate random 16-byte `secKey` and `aesKey`
2. Double-encrypt params with AES-CBC:
   - First encryption: AES-CBC(aesKey, iv=0102030405060708, plaintext)
   - Second encryption: AES-CBC(presetKey, iv=0102030405060708, firstResult)
3. Encrypt `aesKey` with RSA (no padding, reverse bytes)
4. POST with `params` (encrypted data) and `encSecKey` (encrypted key)

### eapi Encryption

1. AES-ECB encrypt params with eapi key
2. Add MD5 signature header
3. Response can be decrypted with `EApiDecrypt()`

### Key Constants

All encryption keys are defined in `pkg/crypto/crypto.go`:
- `presetKey`: `0CoJUm6Qyw8W8jud`
- `iv`: `0102030405060708`
- `publicKey`: RSA public key for weapi
- `eapiKey`: AES key for eapi
- `linuxKey`: AES key for linux api

## Cookie Management

### Storage

- Cookie jar in `pkg/cookie/` implements `http.CookieJar`
- Auto-persisted to JSON file at configurable interval (default 3s)
- File format: JSON array of cookie entries

### CookieCloud

`pkg/cookiecloud/` provides:
- Encrypted cookie sync with CookieCloud server
- AES-GCM decryption of synced data
- Automatic extraction of NetEase music cookies

### Usage in API Client

```go
// Set cookies (e.g., after login)
cli.SetCookies(cookies)

// Get cookies
cookies := cli.GetCookies()

// Cookie persistence is automatic via interval flush
// cli.Close(ctx) ensures final flush
```

## Configuration System

### Config Structure (`config/config.go`)

```go
type Config struct {
    Version  string
    Log      LogConfig
    Network  NetworkConfig
    Database DatabaseConfig
}
```

### Features

- **Viper-based**: Supports YAML, env vars, flags
- **Env var override**: Prefix `NCMCTL_`, e.g., `NCMCTL_LOG_LEVEL=debug`
- **Magic variables**: `${HOME}` replaced at runtime with `config.ReplaceMagicVariables()`
- **Validation**: `config.Validate()` checks all fields
- **Defaults**: `config.GetDefault()` returns sensible defaults

### Default Paths

| Item | Path |
|------|------|
| Config | `~/.ncmctl/config.yaml` |
| Cookie | `~/.ncmctl/cookie.json` |
| Database | `~/.ncmctl/database/badger/` |
| Log | `~/.ncmctl/log/ncm.log` |

## Database Layer

### Interface (`pkg/database/database.go`)

```go
type Database interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
    Close(ctx context.Context) error
}
```

### Badger Implementation (`pkg/database/badger/badger.go`)

- Uses Badger v4 as the storage backend
- Supports TTL for automatic key expiry
- Used by scrobble for dedup records and daily counters

### Key Patterns

```
scrobble:record:{uid}:{songId}    # Song play record
scrobble:today:{uid}              # Daily scrobble counter
```

## Error Handling Patterns

### API Response Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 405 | Already completed (partner) |
| 703 | Not a music partner |
| 8821 | Need behavior verification (risk control) |

### Common Error Patterns

```go
// API call with error check
resp, err := request.SomeMethod(ctx, req)
if err != nil {
    return fmt.Errorf("SomeMethod: %w", err)
}
if resp.Code != 200 {
    return fmt.Errorf("SomeMethod: %+v", resp)
}

// Login check
if request.NeedLogin(ctx) {
    return fmt.Errorf("need login")
}

// Token refresh (always defer after login check)
defer func() {
    refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
    if err != nil || refresh.Code != 200 {
        log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
    }
}()
```
