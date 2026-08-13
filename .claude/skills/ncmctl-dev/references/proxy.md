# Proxy Development

Use this reference for `internal/proxy` forwarding, MITM, capture, protocol decoding, redaction, output, CA handling, and shutdown.

## Contents

- [Primary invariant](#primary-invariant)
- [Component map](#component-map)
- [Forwarding and capture](#forwarding-and-capture)
- [Protocol decoding and redaction](#protocol-decoding-and-redaction)
- [Output and backpressure](#output-and-backpressure)
- [CA security](#ca-security)
- [Shutdown and concurrency](#shutdown-and-concurrency)
- [Testing](#testing)

## Primary invariant

Observation failure must not change real traffic. Capture, truncation, decompression, decoding, formatting, and redaction operate only on bounded observation copies. Never consume, replace, delay, or truncate the forwarded request or response to improve output.

Only configured NetEase target domains are captured and HTTPS-MITM'd. Pass non-target traffic through without capture output.

The proxy has no client authentication. Keep the default listener on `127.0.0.1`, require an explicit host in every listen address, and preserve the startup warning for non-loopback listeners. Treat any non-loopback binding as an unauthenticated open proxy that must be limited to a trusted network behind a firewall.

## Component map

| Surface | Main files |
| --- | --- |
| Configuration and host matching | `config.go`, `host.go` |
| Listener, forwarding, MITM, and shutdown | `server.go`, `connect.go`, `tracked_listener.go` |
| Bounded body observation and content decoding | `body.go`, `content_encoding.go` |
| Protocol classification and decryption | `protocol.go` |
| Recursive redaction and safe formatting | `redact.go` |
| Ordered asynchronous output | `recorder.go` |
| CA generation, loading, permissions, and certificate cache | `ca.go`, `ca_permissions_*.go` |

Read the matching tests beside each file before changing a shared helper.

## Forwarding and capture

- Match canonical hostnames, including subdomains, and keep the default domain slice immutable to callers.
- Use `ConnectAccept` for non-target hostname CONNECT traffic and `ConnectMitm` for matched hostname targets. For an IP CONNECT, boundedly inspect only enough TLS ClientHello bytes to select a route while concurrently observing the upstream: an exposed configured target SNI may be MITM'd, while upstream-first traffic, a missing or malformed SNI, an ECH case that does not expose the target, a non-target SNI, or non-TLS traffic must replay the exact inspected prefixes and remain a transparent tunnel. Never delay a server-first protocol while waiting for client bytes.
- Keep the outer proxy listener on HTTP/1.1. HTTP/2 is default-on only inside a CONNECT MITM tunnel: set the leaf `tls.Config.NextProtos` before installing a `GetConfigForClient` wrapper, advertise `h2` and `http/1.1`, and enable `ForceAttemptHTTP2` on the upstream transport. The two TLS legs negotiate independently; fallback is an ALPN choice, never an HTTP/1.1 replay after an h2 request has started. CONNECT-tunnel h2c compatibility is not an outer proxy protocol.
- When an IP CONNECT is selected for MITM, sign for its SNI but keep HTTP/1.1 and HTTP/2 upstream dials pinned to the original CONNECT address. Preserve `ForceAttemptHTTP2` and the original transport settings when cloning; set TLS `ServerName` to the SNI without replacing HTTPDNS routing with a fresh DNS lookup.
- Arm the bounded CONNECT handshake deadline before acknowledging a target tunnel. Preserve it across zero-value `SetDeadline`, `SetReadDeadline`, and `SetWriteDeadline` calls made by `net/http` or goproxy during hijacking; clear it explicitly only after TLS succeeds, a complete HTTP/1.1 header arrives, or all 24 bytes of `http2.ClientPreface` arrive. A strict incomplete preface, including its first 18 bytes ending in `\r\n\r\n`, is not a complete HTTP/1.1 header.
- Capture request bytes while the transport reads them; never pre-read the client body. The current policy omits every body with `ContentLength < 0`, including finite chunked requests, to avoid delaying long-lived streams. Do not relax it without defining response ordering, shutdown, and early-close behavior.
- Bound every captured body with `MaxBodyBytes`. Record truncation, omission, read, close, or content-decoding failures as observation metadata.
- Preserve original bodies, headers, encodings, and transport semantics. A capture wrapper must forward reads and closes to the real body on every path.
- Keep binary media and protocol upgrades out of body capture where the current omission policy requires it.

When changing forwarding behavior, test the bytes and errors seen by both endpoints, not only the printed capture.

## Protocol decoding and redaction

Classify API, WEAPI, EAPI, Linux API, XEAPI, and generic traffic without writing decoded data back to the HTTP flow.

- EAPI and Linux decoding must verify their actual envelopes and report malformed input as failed observation.
- Passive WEAPI request decryption is `unsupported` because the random AES key is unavailable.
- Passive XEAPI decoding recovers `R`, validates the outer `S` X25519/GCM frame without claiming to decrypt it, and decrypts `B` only when `R` names a session in the shared cache. Restore the logical method, `/api/...` path, query, content type, and Base64 inner body before recursive redaction.
- Never label ciphertext, a guessed value, or a local round-trip result as observed plaintext.

The XEAPI cache is process-local, concurrency-safe, update-ordered, capped at 256 entries with session IDs of at most 1024 bytes, and never persisted. Startup seeds retain their `state-file` or `command-line` source; later complete valid `X-Encr-Ssid` / `X-Encr-Sskey` response headers replace the same ID while other IDs remain available for concurrent in-flight traffic. Learn headers synchronously in the transport immediately after they arrive and before returning the response. Header learning must not depend on HTTP status, body capture/decryption, recorder submission, or output queue capacity. Invalid pairs produce value-free diagnostics and never replace an existing entry. Repeated identical header pairs may refresh eviction order but must not rebuild an unchanged snapshot.

XEAPI requests are `decrypted` only when `B/R/S` are unambiguous and valid and the session key matches. Preserve recovered metadata and report `partial` when one field has conflicting values across duplicates or sources, the session is empty/unknown, a recoverable field fails, or capture omission/truncation/read failure makes the observation incomplete. Identical duplicates are safe to accept. Use `failed` for an invalid/missing `R` only when the request copy is complete. A response-learned key applies to later requests; do not retry an earlier empty-session request.

For XEAPI responses, valid JSON is `plaintext`; an empty body is not plaintext. Otherwise attempt only raw-binary traditional EAPI AES-ECB decryption and bounded inner-gzip expansion, including for non-200 responses. Any omitted, truncated, read-failed, or content-decoding-failed response observation is `partial`, regardless of what its captured prefix resembles. Do not add speculative ASCII-hex compatibility. Provisional XEAPI request records must default `responseEncrypted` to true so dropped request output cannot prevent independent response decryption.

Redact recursively by default across URLs, headers, forms, JSON, nested JSON strings, diagnostics, recovered metadata, and protocol output. For XEAPI, redact every outer `R` copy because its static-key plaintext contains the session ID. If structured redaction cannot be proven safe, emit a bounded placeholder or summary. Invalid UTF-8 and malformed unstructured bodies must fail closed.

Do not use `url.URL.Query()` for capture formatting because it silently discards malformed fields. Preserve any safely decoded field name and replace an unparseable value with `[REDACTED]`; if the name is also unsafe, replace both name and value.

Keep display placeholders out of protocol parsing, parameter precedence, and conflict detection. The `go.mod` language baseline is Go 1.25.0, whose original standard library has no `ParseQuery` parameter-count limit; the limit was added in Go 1.26 and backported to Go 1.25.6. A toolchain with that limit may reject a whole query before parsing fields. Represent the whole-query failure with a bounded placeholder instead of treating it as a malformed field or bypassing the limit.

Only explicit `ShowSensitive` or `--show-sensitive` may expose raw sensitive values. Keep even that mode bounded, and never add secrets to debug logs or test fixtures.

## Output and backpressure

`recorder` uses a bounded asynchronous queue so a blocked stdout or FIFO cannot backpressure forwarding. Preserve request/response ordering when output can progress, but drop observation tasks when the queue is full and emit `CAPTURE_DROPPED` with the accumulated count later.

Treat an `Out.Write` error or short write as an observation failure: report one bounded, always-redacted `CAPTURE_OUTPUT_ERROR` diagnostic to `ErrOut`, without retrying or affecting forwarded traffic.

Closing the recorder marks it closed and waits only for a bounded interval. Do not turn output draining into an unbounded shutdown wait. If `Out.Write` never returns, the command can finish after the timeout while the recorder worker remains blocked; do not promise unconditional worker-goroutine exit.

## CA security

Never use goproxy's bundled public CA private key. Load or generate a repository-owned CA pair and validate that the certificate is a live CA, can sign when key usage is present, matches the private key, and uses distinct paths.

- For the managed default CA, keep parent directories private, create them with restrictive permissions, and create the private key exclusively.
- On POSIX, enforce `0600` on every loaded or generated private key; the certificate may be `0644`. Existing parent-directory permissions are checked only when the managed-path policy is enabled, not for an explicitly supplied existing CA pair.
- On Windows, apply a protected DACL granting full control only to the current process user; file modes are not an ACL substitute.
- Keep generated leaf certificates in the bounded in-memory cache.
- Never commit, print, or reuse a captured CA private key.

Treat cross-compilation as syntax coverage only; Windows ACL behavior requires native Windows tests.

## Shutdown and concurrency

Preserve context cancellation, bounded HTTP shutdown, forced close after timeout, tracked connection cleanup, and transport idle-connection cleanup. Protect shared recorder, certificate-cache, body-capture, and listener state with the existing synchronization abstractions. Require goroutine-exit assertions only for components whose underlying I/O can be unblocked; the recorder follows the bounded-wait limitation above.

Return setup and server errors with context. Logging is appropriate only where asynchronous observation or cleanup cannot return an error to the caller.

## Testing

Run both focused and race checks after shared-state or lifecycle changes:

```bash
go test ./internal/proxy
go test -race ./internal/proxy
```

Use deterministic readers, writers, channels, fake transports, and local TLS servers. Cover loopback and non-loopback listeners, startup warnings, hostname and IP CONNECT targets, target/non-target/missing SNI routing, upstream-first and exact fallback bytes, tunnel copy errors, WebSocket exit, truncated and malformed bodies, output saturation and write failures, cancellation, CA permissions, and cleanup failures without contacting NetEase services or modifying the system trust store.
