# API Client and Endpoint Development

Use this reference for endpoint wrappers, `api.Client`, request options, auxiliary HTTP clients, and transport security.

## Contents

- [Architecture](#architecture)
- [Constructing a client](#constructing-a-client)
- [Adding an endpoint](#adding-an-endpoint)
- [Request options](#request-options)
- [Transport security](#transport-security)
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

`api.Client` owns transport, retry/timeout settings, the persistent Cookie Jar, crypto-mode dispatch, response decoding, and XEAPI state. Endpoint packages define typed request/response contracts and select options. Load `references/protocols.md` through the skill routing table for wire-format or cryptographic changes.

## Constructing a client

Use `api.NewClient` wherever initialization errors can be returned. `api.New` panics and is retained only for callers or tests that intentionally accept that behavior.

```go
cli, err := api.NewClient(networkConfig, logger)
if err != nil {
	return fmt.Errorf("create API client: %w", err)
}
defer func() {
	if closeErr := cli.Close(ctx); closeErr != nil {
		logger.Logger().Error("close API client", "error", closeErr)
	}
}()
```

Inside `internal/ncmctl`, use the existing `closeAPIClient` helper instead of repeating cleanup logic. Load `references/configuration.md` through the skill routing table when changing Cookie configuration or persistence behavior.

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

| Mode | Endpoint/client use | Current constraint |
| --- | --- | --- |
| `CryptoModeWEAPI` | Default Web and mini-program requests | Plain JSON response |
| `CryptoModeEAPI` | PC and mobile requests | JSON-object payloads receive missing `e_r=true` and `header="{}"`; `e_r` selects plaintext or encrypted responses and `x-aeapi` selects inner gzip compression |
| `CryptoModeLinux` | Linux-client requests | Uses the Linux request and response path |
| `CryptoModeAPI` | Plain API requests | The generic layer does not serialize `req` into query or form parameters |
| `CryptoModeXEAPI` | Stateful Aegis/XEAPI requests | Client coordinates URL rewriting, keys, session headers, and response decryption |

There is no `api.WithCryptoMode`. Select modes with `SetCryptoModeWEAPI`, `SetCryptoModeEAPI`, `SetCryptoModeLinux`, `SetCryptoModeAPI`, or `SetCryptoModeXEAPI`.

`api.Client.Request` currently supports GET and POST. Do not set another verb without implementing its transport branch and tests. Check whether an existing GET endpoint actually serializes the request fields before copying its pattern.

For EAPI requests, `Client.Request` normalizes every payload from its final JSON representation, so `map`, struct, and custom `MarshalJSON` inputs follow the same path without field reflection or mutation of the caller's value. Missing `e_r` defaults to `true`; missing, `null`, or empty-string `header` defaults to the JSON string `"{}"`. A non-object payload or an `e_r` value that is not a boolean is rejected before the HTTP request is sent.

The generic layer can only observe the wire JSON. A plain `bool` cannot express both an omitted value and an explicit `false`; requests that need that distinction use `*bool` with `omitempty`, as `types.EApiReqCommon` does. Its `SetResponseEncrypted(false)` method selects a plaintext response. This is a source-breaking change from the former `ER bool` and `Header any` fields; migrate struct literals to the setter (or a `*bool`) and a JSON string header.

For an `e_r=false` response, `Client.Request` decodes plaintext JSON. For an `e_r=true` response, it decrypts raw binary ciphertext; when `x-aeapi=true`, the decrypted payload is gzip-compressed and is transparently expanded. ASCII-hex ciphertext and plaintext JSON are not accepted as `e_r=true` response bodies. EAPI response-processing failures return `*api.APIError`, which callers can retrieve with `errors.As`. Its `StatusCode` is the HTTP status, and `Err` contains any decryption, decompression, or JSON-decoding failure. Non-200 responses go through the same response processing before `Client.Request` returns `APIError`.

## Transport security

The current `api.Client`, CookieCloud client, and HTTP alert client all set `tls.Config.InsecureSkipVerify` to `true`. HTTPS connections therefore encrypt traffic but do not authenticate the server certificate. Treat this as a current security defect, not a compatibility requirement:

- Do not describe these connections as peer-authenticated.
- Do not copy the setting into new clients or tests.
- Keep credentials and Cookie values out of transport diagnostics.
- When changing TLS behavior, use local trusted and untrusted TLS servers to cover certificate validation; do not contact live services.

Removing this setting changes runtime behavior and is outside a documentation-only task. Keep the limitation explicit until the implementation and regression coverage change together.

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

- `httptest.Server` or a fake `RoundTripper` for methods, request serialization, headers, cookies, retries, and timeouts;
- table tests for endpoint request/response shapes and transport-versus-business errors;
- malformed HTTP or JSON responses, canceled contexts, and boundary-size cases;
- injected reader, writer, and closer failures at client transport boundaries;
- local trusted and untrusted TLS servers for certificate-validation behavior.

Useful focused commands:

```bash
go test ./api ./pkg/cookiecloud ./pkg/alert/...
```

The live tests in `api/weapi` and `api/eapi` call `testutil.RequireLiveAPI` and are skipped unless `NCMCTL_RUN_LIVE_TESTS=1`. Some can act on an account, so keep the switch unset in automation and use `make test-live` only deliberately and with explicit authorization.
