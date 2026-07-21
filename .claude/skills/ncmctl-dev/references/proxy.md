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
| Listener, forwarding, MITM, and shutdown | `server.go`, `tracked_listener.go` |
| Bounded body observation and content decoding | `body.go`, `content_encoding.go` |
| Protocol classification and decryption | `protocol.go` |
| Recursive redaction and safe formatting | `redact.go` |
| Ordered asynchronous output | `recorder.go` |
| CA generation, loading, permissions, and certificate cache | `ca.go`, `ca_permissions_*.go` |

Read the matching tests beside each file before changing a shared helper.

## Forwarding and capture

- Match canonical hostnames, including subdomains, and keep the default domain slice immutable to callers.
- Use `ConnectAccept` for non-target CONNECT traffic and `ConnectMitm` only for matched hosts.
- Capture request bytes while the transport reads them; never pre-read the client body. The current policy omits every body with `ContentLength < 0`, including finite chunked requests, to avoid delaying long-lived streams. Do not relax it without defining response ordering, shutdown, and early-close behavior.
- Bound every captured body with `MaxBodyBytes`. Record truncation, omission, read, close, or content-decoding failures as observation metadata.
- Preserve original bodies, headers, encodings, and transport semantics. A capture wrapper must forward reads and closes to the real body on every path.
- Keep binary media and protocol upgrades out of body capture where the current omission policy requires it.

When changing forwarding behavior, test the bytes and errors seen by both endpoints, not only the printed capture.

## Protocol decoding and redaction

Classify API, WEAPI, EAPI, Linux API, XEAPI, and generic traffic without writing decoded data back to the HTTP flow.

- EAPI and Linux decoding must verify their actual envelopes and report malformed input as failed observation.
- Passive WEAPI request decryption is `unsupported` because the random AES key is unavailable.
- Passive XEAPI request decryption is `unsupported` because the session key is unavailable.
- Never label ciphertext, a guessed value, or a local round-trip result as observed plaintext.

Redact recursively by default across URLs, headers, forms, JSON, nested JSON strings, diagnostics, and protocol output. If structured redaction cannot be proven safe, emit a bounded placeholder or summary. Invalid UTF-8 and malformed unstructured bodies must fail closed.

Only explicit `ShowSensitive` or `--show-sensitive` may expose raw sensitive values. Keep even that mode bounded, and never add secrets to debug logs or test fixtures.

## Output and backpressure

`recorder` uses a bounded asynchronous queue so a blocked stdout or FIFO cannot backpressure forwarding. Preserve request/response ordering when output can progress, but drop observation tasks when the queue is full and emit `CAPTURE_DROPPED` with the accumulated count later.

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

Use deterministic readers, writers, channels, fake transports, and local TLS servers. Cover loopback and non-loopback listeners, startup warnings, target and non-target traffic, truncated and malformed bodies, output saturation, cancellation, CA permissions, and cleanup failures without contacting NetEase services or modifying the system trust store.
