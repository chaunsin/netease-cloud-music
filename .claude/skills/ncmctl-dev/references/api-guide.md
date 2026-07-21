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
| `CryptoModeEAPI` | PC and mobile requests | `Client.Request` does not transparently decrypt encrypted `e_r=true` responses |
| `CryptoModeLinux` | Linux-client requests | Uses the Linux request and response path |
| `CryptoModeAPI` | Plain API requests | The generic layer does not serialize `req` into query or form parameters |
| `CryptoModeXEAPI` | Stateful Aegis/XEAPI requests | Client coordinates URL rewriting, keys, session headers, and response decryption |

There is no `api.WithCryptoMode`. Select modes with `SetCryptoModeWEAPI`, `SetCryptoModeEAPI`, `SetCryptoModeLinux`, `SetCryptoModeAPI`, or `SetCryptoModeXEAPI`.

`api.Client.Request` currently supports GET and POST. Do not set another verb without implementing its transport branch and tests. Check whether an existing GET endpoint actually serializes the request fields before copying its pattern.

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

The tests in `api/weapi` and `api/eapi` are not all isolated; many call real NetEase endpoints and some can act on an account. Do not include those packages in an automatic broad test run without explicit authorization and a deliberate test selection.
