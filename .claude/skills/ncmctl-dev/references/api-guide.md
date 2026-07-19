# API Development Guide

Use this reference for endpoint wrappers, `api.Client`, crypto modes, XEAPI state, cookies, configuration, and database work.

## Contents

- [Architecture](#architecture)
- [Constructing a client](#constructing-a-client)
- [Adding an endpoint](#adding-an-endpoint)
- [Request options](#request-options)
- [Crypto details](#crypto-details)
- [Cookie management](#cookie-management)
- [Configuration](#configuration)
- [Database layer](#database-layer)
- [Error handling](#error-handling)
- [Testing strategy](#testing-strategy)

## Architecture

```text
internal/ncmctl command or external caller
                  |
                  v
api/weapi, api/eapi, api/api, api/linux
                  |
                  v
             api.Client
       /          |           \
 pkg/crypto   pkg/cookie   resty/http
       |
 api/xeapi.go coordinates XEAPI key and session state
```

`api.Client` owns transport, retry/timeout settings, the persistent Cookie Jar, crypto-mode dispatch, response decoding, and XEAPI state. Endpoint packages define typed request/response contracts and select options.

## Constructing a client

Use `api.NewClient` in code that can return an initialization error. `api.New` panics and is mainly retained for callers or tests that intentionally accept that behavior.

```go
cfg := &api.Config{
	Timeout: 60 * time.Second,
	Retry:   3,
	Cookie: cookie.Config{
		Filepath: "/path/to/cookie.json",
		Interval: 3 * time.Second,
	},
}

cli, err := api.NewClient(cfg, logger)
if err != nil {
	return fmt.Errorf("create API client: %w", err)
}
defer func() {
	if closeErr := cli.Close(ctx); closeErr != nil {
		logger.Logger().Error("close API client", "error", closeErr)
	}
}()
```

Inside `internal/ncmctl`, use the existing `closeAPIClient` helper instead of repeating the defer.

## Adding an endpoint

Create the method in the package matching the wire protocol. A WEAPI endpoint follows this shape:

```go
package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
)

type FeatureReq struct {
	ID string `json:"id"`
}

type FeatureResp struct {
	Code int64 `json:"code"`
	Data any   `json:"data"`
}

func (a *Api) Feature(ctx context.Context, req *FeatureReq) (*FeatureResp, error) {
	var (
		endpoint = "https://music.163.com/weapi/example/feature"
		reply    FeatureResp
		opts     = api.NewOptions()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return &reply, nil
}
```

Keep transport failure and business failure separate: the wrapper returns request/decode errors, while a command or service checks `reply.Code` according to that endpoint's contract.

Before inventing a shared response type, search `api/types` and neighboring endpoints. Do not move an endpoint-specific shape into `api/types` solely to shorten one file.

## Request options

`api.NewOptions()` defaults to POST plus `CryptoModeWEAPI`. Options are mutable setters, not functional options.

```go
opts := api.NewOptions().SetCryptoModeEAPI()
opts.SetMethod(http.MethodPost)
opts.SetHeader("key", "value")
opts.SetCookies(cookies...)
```

Available modes:

| Mode | Request behavior | Response behavior |
| --- | --- | --- |
| `CryptoModeWEAPI` | Double AES-CBC plus RSA-wrapped random key | Plain JSON |
| `CryptoModeEAPI` | Path-bound digest envelope encrypted with AES-ECB | `Client.Request` currently accepts plain JSON only; it does not transparently decrypt encrypted `e_r=true` responses |
| `CryptoModeLinux` | AES-ECB `eparams` wrapper | Linux decrypt path |
| `CryptoModeAPI` | No encryption, but the generic request layer currently does not serialize `req` into query or form parameters | Plain response |
| `CryptoModeXEAPI` | XEAPI envelope, public-key/session state, `B`/`S`/`R` form | XEAPI response decrypt path and session-header update |

There is no `api.WithCryptoMode`. Select modes with `SetCryptoModeWEAPI`, `SetCryptoModeEAPI`, `SetCryptoModeLinux`, `SetCryptoModeAPI`, or `SetCryptoModeXEAPI`.

`api.Client.Request` currently supports GET and POST. Do not set another verb without implementing its transport branch and tests. Check whether an existing GET endpoint actually serializes the request fields before copying its pattern.

## Crypto details

### WEAPI

`pkg/crypto.WeApiEncrypt`:

1. Marshals the request to JSON.
2. Generates one random 16-byte base62 secret key.
3. Encrypts JSON with the preset key using AES-CBC and the protocol IV.
4. Encrypts that base64 result with the random secret key using AES-CBC.
5. Reverses and RSA-encrypts the random key without padding.
6. Returns `params` and `encSecKey`.

A passive proxy does not know the random secret key, so it cannot truthfully decrypt an observed WEAPI request. Preserve this limitation in proxy and user documentation.

### EAPI

`pkg/crypto.EApiEncrypt` normalizes the first `eapi` path segment to `api`, combines the route, JSON body, and protocol MD5 digest, then AES-ECB encrypts the envelope. The route is authenticated input; passing the wrong path produces a wire-incompatible payload even if local encryption succeeds.

`EApiDecrypt` accepts the content encoding explicitly. `Client.Request` currently passes every EAPI response body directly to JSON decoding, so wrappers must request a plain response; encrypted `e_r=true` responses are not supported by the generic path. Add captured evidence and endpoint tests before introducing conditional response decryption.

### Linux API

`LinuxApiEncrypt` wraps JSON as hexadecimal `eparams`; `LinuxApiDecrypt` handles the corresponding encrypted response. Keep this mode separate from EAPI even though both use AES-ECB.

### XEAPI

XEAPI spans `api/api.go`, `api/options.go`, `api/xeapi.go`, and `pkg/crypto/crypto.go`:

- The original URL must contain `/api/`, `/eapi/`, or `/xeapi/`.
- The plaintext envelope keeps the logical API route while the transport URL uses `/xeapi/`.
- Query parameters move into the encrypted envelope and are removed from the transport URL.
- `Client` refreshes public-key state when absent/expired and coalesces refreshes with `singleflight`.
- The server's `X-Encr-Ssid` and `X-Encr-Sskey` response headers update session state for later requests.
- Requests set the XEAPI user agent and `X-Client-Enc-State: ENCRYPTED`.
- `SetXeapiOS` and `SetXeapiAppVer` override the default client identity only for XEAPI.

Do not change one layer in isolation. Validate URL rewriting, key refresh, header behavior, captured request vectors, and response decryption as applicable. Treat `docs/xeapi.md` as research context and read its current implementation-status table before the archived notes. Use `api/xeapi_test.go` and the XEAPI cases in `pkg/crypto/crypto_test.go` as executable evidence.

### NCBL

NCBL is an independent version 3 log-body wire format implemented in `pkg/crypto/ncbl.go`; it is not part of XEAPI public-key or session handling. Changes must preserve its frame header, metadata bounds, ChaCha20 key/nonce derivation, compression selection, and frame sequencing. Use `pkg/crypto/ncbl_test.go` for fixed vectors and malformed-input coverage.

## Cookie management

The Cookie Jar is URL-scoped. Both public helpers require a parsed URL:

```go
musicURL, err := url.Parse("https://music.163.com")
if err != nil {
	return fmt.Errorf("parse music URL: %w", err)
}

cli.SetCookies(musicURL, cookies)
stored := cli.GetCookies(musicURL)
```

Key behavior:

- The default embedded config stores cookies at `${HOME}/.ncmctl/cookie.json` and syncs every three seconds.
- The parent directory and file are created with `0700` and `0600` permissions on POSIX.
- `Client.Close` triggers a final export; callers must not discard close errors silently.
- `GetDeviceId` checks `deviceId` and `sDeviceId` across the music, interface, and interface3 domains for XEAPI use.
- CookieCloud retrieves third-party cookie data; keep credentials and returned cookies out of logs and fixtures.

## Configuration

`config.Config` contains pointers to the package-owned configuration types:

```go
type Config struct {
	Version  string
	Log      *log.Config
	Network  *api.Config
	Database *database.Config
}
```

Stable runtime behavior:

- No `--config`: use the embedded `config/config.yaml` directly.
- Intended `--config` behavior: Viper reads that exact file, rejects unknown fields through `UnmarshalExact`, and applies `NCMCTL_` environment overrides.
- `--home` replaces `${HOME}` in log, Cookie, and database paths after loading.
- Global `--debug` enables stdout/debug logging and network debug after config loading.

The repository-root `AGENTS.md` owns the current loader limitation and verification status. Do not duplicate its transient error text here. Never claim `~/.ncmctl/config.yaml` is auto-loaded; it is only a user-selected path.

When documenting a custom config, use the full shape from `config/config.yaml`. Invented top-level fields such as `timeout` or `download` are rejected or ignored by the actual schema.

## Database layer

The current interface in `pkg/database/database.go` is:

```go
type Database interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl ...time.Duration) error
	Exists(ctx context.Context, key string) (bool, error)
	Increment(ctx context.Context, key string, value int64, ttl ...time.Duration) (int64, error)
	Del(ctx context.Context, key string) error
	Close(ctx context.Context) error
}
```

`database.New` currently selects Badger. Scrobble stores per-account play deduplication and daily counters; preserve key construction and TTL behavior when refactoring. Always close the database and report close failures at the command boundary.

## Error handling

Use contextual wrapping for transport and local failures:

```go
resp, err := request.Feature(ctx, req)
if err != nil {
	return fmt.Errorf("Feature: %w", err)
}
if resp.Code != 200 {
	return fmt.Errorf("Feature response: %+v", resp)
}
```

Do not:

- return a non-nil zero-value response for an unimplemented endpoint;
- convert a transport error into an API business code;
- log secrets or full encrypted envelopes merely because debug logging is enabled;
- discard cleanup errors where a helper or deferred logger is available;
- treat one endpoint's response codes as a repository-wide enum.

## Testing strategy

Prefer deterministic, offline tests:

- `httptest.Server` or a fake `RoundTripper` for transport and headers;
- fixed crypto vectors from known captures or compatible implementations;
- malformed payload and boundary-size cases;
- explicit error injection for random sources, compressors, readers, writers, and closers;
- race tests for shared XEAPI or proxy state.

Useful focused commands:

```bash
go test ./api ./pkg/crypto
go test -run 'TestRewriteXeapiURL|TestXeapi' ./api
go test -run 'TestIssue174|TestXeapi' ./pkg/crypto
go test -run 'TestNCBL' ./pkg/crypto
```

The tests in `api/weapi` and `api/eapi` are not all isolated; many call real NetEase endpoints and some can act on an account. Do not include those packages in an automatic broad test run without explicit authorization and a deliberate test selection.
