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
5. Reverse and RSA-encrypt the random key with no OAEP or PKCS#1 padding (textbook RSA with preset modulus).
6. Return `params` and `encSecKey`.

A passive proxy cannot recover the random secret key and therefore cannot truthfully decrypt an observed WEAPI request. Preserve the `unsupported` boundary in proxy and user documentation.

## EAPI

`pkg/crypto.EApiEncrypt` currently replaces the first `eapi` substring in the supplied request URL with `api`; the implementation is not path-segment-aware. It then combines that route, the JSON body, and the protocol MD5 digest before AES-ECB encryption.

The route is authenticated input. Do not make normalization appear more precise than the implementation, and do not change substring or path-boundary behavior without fixed vectors and boundary tests.

`EApiDecrypt` accepts the content encoding explicitly. `Client.Request` normalizes EAPI JSON-object payloads before encryption from their final marshaled JSON, not reflected Go fields: a missing `e_r` becomes `true`, and a missing, `null`, or empty-string `header` becomes the JSON string `"{}"`. The normalized payload is local to the request. `types.EApiReqCommon` uses `*bool` so an explicit `false` remains distinguishable from omission.

The same resolved `e_r` value controls response decoding: `true` → raw binary ciphertext (gzip-expanded if `x-aeapi=true`); `false` → plaintext JSON. ASCII-hex ciphertext is not accepted on the encrypted path. Response-processing failures return `*api.APIError`; see `references/api-guide.md` § Request options for caller details. Do not infer omission versus `false` from an ordinary Go `bool`: that information is already lost before JSON normalization can inspect it.

## Linux API

`LinuxApiEncrypt` wraps JSON as hexadecimal `eparams`; `LinuxApiDecrypt` handles the corresponding encrypted response. Both use AES-ECB, but with a different static key and URL scheme from EAPI — keep the two modes separate. Linux API requests target `/api/linux/forward`, while EAPI requests route through `/eapi/`-prefixed paths.

## XEAPI

XEAPI spans `api/api.go`, `api/options.go`, `api/xeapi.go`, and `pkg/crypto/xeapi.go`:

- Require the original URL to contain `/api/`, `/eapi/`, or `/xeapi/`.
- Keep the logical API route in the plaintext envelope while transporting on `/xeapi/`.
- Move query parameters into the encrypted envelope and remove them from the transport URL.
- Protect public-key and session state with `xeapi.mux`; use `singleflight` only to coalesce refreshes, not as a replacement for the mutex. (`singleflight` deduplicates concurrent identical calls but does not protect reads/writes between calls; the mutex guards state consistency across the full read-modify-write cycle.)
- Refresh expired or absent public-key state and apply `X-Encr-Ssid` and `X-Encr-Sskey` response updates under the same state-machine discipline.
- Set the XEAPI user agent and `X-Client-Enc-State: ENCRYPTED`.
- Keep the XEAPI identity in `Headers.XEAPI` (`appver`, `os`, and `User-Agent`); do not leak those defaults into other modes.
- `XeapiDecryptRequest` always recovers `R`, optionally decrypts `B` with a raw 16/24/32-byte dynamic/session key, and strictly validates a present `S` as a Base64 frame containing a 32-byte X25519 public key, 12-byte GCM nonce, tag, and non-empty ciphertext. Missing `S` remains a caller policy decision; neither the crypto API nor proxy may claim to decrypt it without the relevant private key.

The active API client and passive proxy use different session state machines; see `references/proxy.md` § Protocol decoding and redaction for the proxy-side rules. `crypto decrypt` remains independent and does not read either state source automatically.

Do not change URL rewriting, key refresh, session updates, locking, headers, or response decryption in isolation. Read the current-status table and only the relevant research section in `docs/xeapi.md` before archived notes.

## NCBL

NCBL is an independent version 3 log-body wire format implemented in `pkg/crypto/ncbl.go`; it is not part of XEAPI public-key or session handling. Preserve its frame header, metadata bounds, ChaCha20 key/nonce derivation, compression selection, and frame sequencing.

## Testing

Use fixed vectors and malformed-input coverage:

```bash
go test ./api ./pkg/crypto
go test -run 'TestRewriteXeapiURL|TestXeapi' ./api
go test -run 'TestIssue174|TestXeapi' ./pkg/crypto  # TestIssue174 is a regression for XEAPI URL rewriting edge cases
go test -run 'NCBL' ./pkg/crypto
```

Run race coverage when changing XEAPI shared state. Do not run live endpoint packages merely to validate local protocol changes.
