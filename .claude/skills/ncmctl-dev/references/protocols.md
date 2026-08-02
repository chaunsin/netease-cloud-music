# Protocol and Crypto Development

Use this reference for WEAPI, EAPI, Linux API, XEAPI, NCBL, and other wire-format or cryptographic changes.

## Contents

- [Evidence requirements](#evidence-requirements)
- [WEAPI](#weapi)
- [EAPI](#eapi)
- [Linux API](#linux-api)
- [XEAPI](#xeapi)
- [NCBL](#ncbl)
- [Testing](#testing)

## Evidence requirements

Combine source and focused tests with captures or independently sourced fixed vectors. A local encrypt/decrypt round trip proves internal consistency only; it does not prove wire compatibility.

Preserve malformed-input handling, boundary limits, content encodings, and error propagation. Keep protocol-derived secrets and captured credentials out of logs and fixtures.

## WEAPI

`pkg/crypto.WeApiEncrypt`:

1. Marshal the request to JSON.
2. Generate one random 16-byte base62 secret key.
3. Encrypt JSON with the preset key using AES-CBC and the protocol IV.
4. Encrypt that base64 result with the random secret key using AES-CBC.
5. Reverse and RSA-encrypt the random key without padding.
6. Return `params` and `encSecKey`.

A passive proxy cannot recover the random secret key and therefore cannot truthfully decrypt an observed WEAPI request. Preserve the `unsupported` boundary in proxy and user documentation.

## EAPI

`pkg/crypto.EApiEncrypt` currently replaces the first `eapi` substring in the supplied request URL with `api`; the implementation is not path-segment-aware. It then combines that route, the JSON body, and the protocol MD5 digest before AES-ECB encryption.

The route is authenticated input. Do not make normalization appear more precise than the implementation, and do not change substring or path-boundary behavior without fixed vectors and boundary tests.

`EApiDecrypt` accepts the content encoding explicitly. `Client.Request` normalizes EAPI JSON-object payloads before encryption from their final marshaled JSON, not reflected Go fields: a missing `e_r` becomes `true`, and a missing, `null`, or empty-string `header` becomes the JSON string `"{}"`. The normalized payload is local to the request. `types.EApiReqCommon` uses `*bool` so an explicit `false` remains distinguishable from omission.

The same resolved `e_r` value controls response decoding. When it is true, the body is decrypted as raw binary ciphertext; `x-aeapi=true` means the decrypted payload is gzip-compressed and must be expanded. ASCII-hex ciphertext and plaintext JSON are not accepted on this path. When `e_r` is false, the body is decoded as plaintext JSON. EAPI response-processing failures return `*api.APIError`: `StatusCode` records the HTTP response status, and `Err` contains any decryption, decompression, or JSON-decoding failure. Non-200 responses go through the same decoding path before `Client.Request` returns `APIError`. Do not infer omission versus `false` from an ordinary Go `bool`: that information is already lost before JSON normalization can inspect it.

## Linux API

`LinuxApiEncrypt` wraps JSON as hexadecimal `eparams`; `LinuxApiDecrypt` handles the corresponding encrypted response. Keep this mode separate from EAPI even though both use AES-ECB.

## XEAPI

XEAPI spans `api/api.go`, `api/options.go`, `api/xeapi.go`, and `pkg/crypto/xeapi.go`:

- Require the original URL to contain `/api/`, `/eapi/`, or `/xeapi/`.
- Keep the logical API route in the plaintext envelope while transporting on `/xeapi/`.
- Move query parameters into the encrypted envelope and remove them from the transport URL.
- Protect public-key and session state with `xeapi.mux`; use `singleflight` only to coalesce refreshes, not as a replacement for the mutex.
- Refresh expired or absent public-key state and apply `X-Encr-Ssid` and `X-Encr-Sskey` response updates under the same state-machine discipline.
- Set the XEAPI user agent and `X-Client-Enc-State: ENCRYPTED`.
- Keep the XEAPI identity in `Headers.XEAPI` (`appver`, `os`, and `User-Agent`); do not leak those defaults into other modes.

Do not change URL rewriting, key refresh, session updates, locking, headers, or response decryption in isolation. Read the current-status table and only the relevant research section in `docs/xeapi.md` before archived notes.

## NCBL

NCBL is an independent version 3 log-body wire format implemented in `pkg/crypto/ncbl.go`; it is not part of XEAPI public-key or session handling. Preserve its frame header, metadata bounds, ChaCha20 key/nonce derivation, compression selection, and frame sequencing.

## Testing

Use fixed vectors and malformed-input coverage:

```bash
go test ./api ./pkg/crypto
go test -run 'TestRewriteXeapiURL|TestXeapi' ./api
go test -run 'TestIssue174|TestXeapi' ./pkg/crypto
go test -run 'NCBL' ./pkg/crypto
```

Run race coverage when changing XEAPI shared state. Do not run live endpoint packages merely to validate local protocol changes.
