---
title: ncmctl Command Reference
description: Current command flags, behavior, safety boundaries, and configuration schema for ncmctl.
version: "0.4.0"
---

# Command Reference

## Contents

- [Getting help](#getting-help)
- [Global flags](#global-flags)
- [task](#task)
- [sign](#sign)
- [partner](#partner)
- [scrobble](#scrobble)
- [download](#download)
- [cloud](#cloud)
- [ncm](#ncm)
- [crypto](#crypto)
- [curl](#curl)
- [proxy](#proxy)
- [completion](#completion)
- [Configuration](#configuration)

Login and installation are documented in `install-and-login.md`.

## Getting help

The installed binary is authoritative for its version:

```bash
ncmctl --help
ncmctl COMMAND --help
ncmctl login METHOD --help
ncmctl crypto encrypt --help
ncmctl crypto decrypt --help
```

Do not infer that a value accepted by a generic flag is implemented by every subcommand. The limitations below reflect the current source behavior.

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `--debug` | false | Enable debug/stdout logging and network debug |
| `-c, --config <file>` | none | Load this exact complete YAML file; omitted sections are not merged |
| `--home <dir>` | OS user home | Substitute this value for `${HOME}` in runtime paths |
| `-v, --version` | - | Print build and runtime version information |

Without `--config`, ncmctl uses embedded defaults. It does not automatically read `~/.ncmctl/config.yaml`.

## task

Run selected account tasks on cron schedules as a long-running service. Login is required.

```bash
# No selectors means all three jobs
ncmctl task

# Only sign and scrobble
ncmctl task --sign --scrobble

# Change the scrobble schedule and timezone
ncmctl task --scrobble \
  --scrobble.cron '0 20 * * *' \
  --location Asia/Shanghai
```

| Flag | Default | Description |
| --- | --- | --- |
| `--runAll` | false | Register all jobs; no selectors has the same effect |
| `--sign` | false | Register the sign job |
| `--partner` | false | Register the partner job |
| `--scrobble` | false | Register the scrobble job |
| `--sign.cron` | `0 10 * * *` | Sign schedule |
| `--partner.cron` | `0 18 * * *` | Partner schedule |
| `--scrobble.cron` | `0 18 * * *` | Scrobble schedule |
| `--sign.automatic` | false | Claim eligible sign/VIP rewards; increased account risk |
| `--partner.star` | `3,4` | Base evaluation score choices, each 1-5 |
| `--partner.extStar` | `2,3,4` | Extra evaluation score choices, each 1-5 |
| `--partner.extNum` | `random` | Extra evaluation count: `random` (2-7) or 0-15 |
| `--scrobble.num` | 300 | Requested play-log count, 1-300 |
| `-l, --location` | `Asia/Shanghai` | IANA timezone for cron |

Schedules use standard five-field cron syntax. Press Ctrl+C or send SIGTERM to stop the service.

## sign

Run YunBei and eligible VIP sign-in actions once. Login is required.

```bash
ncmctl sign
ncmctl sign --automatic
```

| Flag | Default | Description |
| --- | --- | --- |
| `-a, --automatic` | false | Claim available YunBei and VIP rewards in addition to sign-in |

Automatic reward handling performs more account actions and may increase risk-control exposure.

## partner

Submit music-partner evaluations once. Login and partner eligibility are required.

```bash
ncmctl partner
ncmctl partner --star 3,4 --extra 2,3,4
ncmctl partner --num 5
```

| Flag | Default | Description |
| --- | --- | --- |
| `-s, --star` | `3,4` | Unique base score choices from 1 through 5 |
| `-e, --extra` | `2,3,4` | Unique extra score choices from 1 through 5 |
| `-n, --num` | `random` | Extra evaluation count: `random` (2-7) or 0-15 |

The command reports play events, waits 15-24 seconds per item, and submits evaluations. It changes account state; do not use it as a connectivity test. Failures propagate through the process exit status and through the `task` scheduler log.

## scrobble

Submit play logs to increase the account's listen count. Login is required and the feature has a high risk of account restrictions.

```bash
ncmctl scrobble
ncmctl scrobble --num 200
```

| Flag | Default | Description |
| --- | --- | --- |
| `-n, --num` | 300 | Requested songs, 1-300; the daily total is capped at 300 |

Deduplication and the daily counter are stored in `<home>/.ncmctl/database/badger/`. Deleting the database loses the local history and does not reset NetEase's server-side count. The command may complete fewer songs when the available top-list tracks are already recorded.

## download

Download one or more songs, albums, artists, or playlists. Login is required. A bare numeric input is treated as a song ID; URLs identify song, album, artist, or playlist resources.

```bash
# Song ID or URL
ncmctl download --level hires 1820944399
ncmctl download --level lossless \
  'https://music.163.com/song?id=1820944399'

# Album, artist, or playlist URL
ncmctl download 'https://music.163.com/#/album?id=34608111'
ncmctl download --strict 'https://music.163.com/#/artist?id=33400892'
ncmctl download 'https://music.163.com/playlist?id=593617579'

# Destination and parallelism
ncmctl download --output ./music --parallel 5 1820944399

# Multiple resources in one invocation
ncmctl download 1820944399 \
  'https://music.163.com/playlist?id=593617579'
```

| Flag | Default | Description |
| --- | --- | --- |
| `-o, --output` | `./download` | Output directory |
| `-p, --parallel` | 5 | Concurrent downloads, 1-20 |
| `-l, --level` | `lossless` | Requested quality |
| `--encode-type` | `flac` | Encode type sent to the player endpoint |
| `--immerse-type` | `c51` | Immersive-audio type sent to the endpoint |
| `--strict` | false | Skip a song if the exact requested quality is unavailable |
| `--tag` | true | Compatibility placeholder; download tag writing is not implemented and either boolean value currently has no effect |

Quality names and aliases:

| Quality | Aliases | Nominal level |
| --- | --- | --- |
| `standard` | `128` | 128 kbps |
| `higher` | `192` | 192 kbps |
| `exhigh` | `HQ`, `320` | 320 kbps |
| `lossless` | `SQ` | Lossless |
| `hires` | `HR` | Hi-Res |

The command writes to a temporary file, verifies the server-provided MD5, and then renames the completed file.

## cloud

Upload exactly one local music file or recursively scan one directory. Login is required and uploads modify the account's cloud disk.

```bash
ncmctl cloud '/path/to/music.mp3'
ncmctl cloud '/path/to/music/'
ncmctl cloud --parallel 5 --minsize 1MB \
  --regexp '.*\.flac$' '/path/to/music/'
```

| Flag | Default | Description |
| --- | --- | --- |
| `-p, --parallel` | 3 | Concurrent uploads, supported range 1-10 |
| `-m, --minsize` | none | Skip smaller files; units include B, KB, and MB variants |
| `-r, --regexp` | none | Regular expression matched against candidate paths |

The current upload limit is 500 MB per file, and directory traversal rejects paths deeper than three levels. The command accepts extensions recognized by the repository's music-extension list, submits local tag metadata, polls conversion up to three times, and publishes the uploaded song to the account.

## ncm

Decode local `.ncm` files without logging in.

```bash
ncmctl ncm '/path/to/file.ncm' --output ./decoded
ncmctl ncm '/path/to/directory' --output ./decoded --parallel 10

# Historical inverted flag: this disables tag writing
ncmctl ncm '/path/to/file.ncm' --tag
```

Every positional path is treated as an input. Set the destination only with `-o`/`--output`; for example, use `ncmctl ncm '/path/to/directory' -o .` to write to the current directory.

A missing path or an explicitly provided non-`.ncm` file returns an error before the output directory is created.

| Flag | Default | Description |
| --- | --- | --- |
| `-o, --output` | `./ncm` | Output directory |
| `-p, --parallel` | 10 | Concurrent decodes, 1-50 |
| `--tag` | false | Despite its name, setting it disables tag writing |

Tags are written by default for supported MP3 and FLAC output. Directory traversal rejects paths deeper than three levels. Existing destination names are preserved by adding a numeric suffix.

## crypto

Inspect legacy API encryption formats locally. This is a debugging tool, not an authentication bypass.

```bash
# Encrypt a JSON string or a file containing one JSON object
ncmctl crypto encrypt --kind weapi '{"key":"value"}'
ncmctl crypto encrypt --kind eapi \
  --url /eapi/v3/song/detail '{"c":[]}'
ncmctl crypto encrypt --kind linux '{"method":"POST"}'
ncmctl crypto encrypt --kind weapi request.json \
  --output encrypted.json

# Direct EAPI request decryption
ncmctl crypto decrypt --kind eapi --encode hex 'CIPHERTEXT'

# EAPI-focused HAR processing; restrict the path for mixed captures
ncmctl crypto decrypt --url '/eapi/*' capture.har
```

Parent flags:

| Flag | Default | Description |
| --- | --- | --- |
| `-k, --kind` | `weapi` | Accepted encryption mode: `weapi`, `eapi`, or `linux` |
| `-o, --output` | stdout | Write JSON output to this file |

`encrypt` adds `-u, --url`, which is required for EAPI because the route participates in the digest.

`decrypt` adds:

| Flag | Default | Description |
| --- | --- | --- |
| `-e, --encode` | `hex` | Ciphertext encoding: `string`, `hex`, or `base64` |
| `-u, --url` | `*` | Path glob used while selecting HAR entries |

Current limitation: direct request decryption is implemented for EAPI. WEAPI requires the unavailable random client key; direct Linux and plain-API decrypt branches are not implemented. `--kind` being accepted by the parent command does not imply decrypt support.

HAR files and decrypted output can contain credentials and personal data. Store and share them as secrets.

## curl

Reflectively invoke an exported method from one API wrapper package. This is an ncmctl subcommand, not the system `curl` command.

```bash
# Calls weapi.Api.GetUserInfo; requires an authenticated Cookie
ncmctl curl --kind weapi --data '{}' GetUserInfo

# The flag form overrides the positional method name
ncmctl curl --kind weapi --method GetUserInfo --data '{}'
```

| Flag | Default | Description |
| --- | --- | --- |
| `-m, --method` | positional argument | Exported Go API method name; not an HTTP verb |
| `-d, --data` | `{}` | JSON decoded into that method's request struct; unknown fields fail |
| `-o, --output` | stdout | Write formatted response JSON to this file |
| `-k, --kind` | `weapi` | Wrapper package: `weapi`, `eapi`, `linux`, or `api` |
| `-t, --timeout` | `15s` | Context deadline for the invocation |

The selected endpoint determines login requirements and side effects. Inspect the method before invoking unfamiliar names. Any `--kind` value outside the four listed values is rejected before the API client is created.

## proxy

Monitor HTTP and HTTPS requests from a client the user controls. The command itself does not require a NetEase login.

```bash
# Local-only listener
ncmctl proxy

# Captures to a file; startup and diagnostics remain on stderr
ncmctl proxy > capture.log

# Trusted LAN only: unauthenticated listener
ncmctl proxy --listen 0.0.0.0:9000

# Existing matching CA pair
ncmctl proxy --ca-cert ./ca.crt --ca-key ./ca.key

# Change runtime root and generated CA location
ncmctl --home /srv/ncmctl proxy

# Larger display limit and no redaction: sensitive output
ncmctl proxy --max-body 4MB --show-sensitive
```

| Flag | Default | Description |
| --- | --- | --- |
| `--listen` | `127.0.0.1:9000` | Explicit proxy host and port |
| `--ca-cert` | generated path | Existing CA certificate; requires `--ca-key` |
| `--ca-key` | generated path | Existing CA private key; requires `--ca-cert` |
| `--max-body` | `1MB` | Per-request/response display limit; forwarding is not truncated |
| `--show-sensitive` | false | Disable credential and personal-data redaction |

With no CA flags, ncmctl creates and reuses:

- `<home>/.ncmctl/proxy/ca.crt`
- `<home>/.ncmctl/proxy/ca.key`

Install and trust only `ca.crt` on the client device. The command prints its path and SHA-256 fingerprint but does not modify any trust store. Keep `ca.key` private.

Behavior and limitations:

- Only NetEase-related target domains are captured or MITM'd; other traffic is forwarded without capture.
- Structured content is formatted and recursively redacted by default. Binary, media, multipart, unknown-length streaming, invalid UTF-8, and unsafe unstructured bodies are summarized.
- Display truncation, decompression, parsing, or redaction failure does not change forwarded bytes.
- EAPI, Linux API, and plain API payloads are decoded when possible.
- A passive proxy cannot recover WEAPI's random request key or modern XEAPI session keys; those request fields are marked `unsupported`, not presented as plaintext.
- Certificate pinning, Android user-CA restrictions, QUIC/HTTP3, proxy bypass, WebSocket frames, and CONNECT requests addressed only by IP can prevent complete capture.
- Capture output uses a bounded queue. If stdout blocks, `CAPTURE_DROPPED` reports omitted capture blocks rather than delaying real traffic.
- `--listen 0.0.0.0:9000` exposes an unauthenticated proxy. Use it only temporarily on a trusted network.

Press Ctrl+C or send SIGTERM for graceful shutdown.

## completion

Generate a completion script locally without logging in or contacting NetEase:

```bash
ncmctl completion bash
ncmctl completion fish
ncmctl completion powershell
ncmctl completion zsh
```

The command writes the script to stdout. Follow `ncmctl completion <shell> --help` for the installed shell-specific setup instructions, and redirect to a file only after confirming the destination.

## Configuration

The intended customization flow is to copy the repository's `config/config.yaml`, edit it, and pass it explicitly:

```bash
ncmctl --config ~/.ncmctl/config.yaml COMMAND
```

The loader reads the exact file, applies `NCMCTL_` environment overrides, and rejects unknown fields or unsupported log formats and levels. Start from the complete schema below because partial files do not inherit omitted sections from the embedded defaults.

Current schema:

```yaml
version: "1.0"
log:
  app: ncm
  format: text
  level: info
  stdout: false
  rotate:
    filename: "${HOME}/.ncmctl/log/ncm.log"
    maxsize: 100
    maxage: 7
    maxbackups: 3
    localtime: true
    compress: true
network:
  debug: false
  timeout: 60s
  retry: 3
  cookie:
    filepath: "${HOME}/.ncmctl/cookie.json"
    interval: 3s
database:
  driver: badger
  path: "${HOME}/.ncmctl/database/badger/"
```

`log.stdout` is a legacy field name; when true, the configured logger mirrors output to stderr in addition to the rolling file.

The loader rejects unknown fields. There are no top-level `download`, `timeout`, or `output` configuration keys.

Environment variables use the `NCMCTL_` prefix and underscores for nested keys:

```bash
NCMCTL_LOG_LEVEL=debug \
NCMCTL_NETWORK_TIMEOUT=30s \
ncmctl --config ~/.ncmctl/config.yaml COMMAND
```

An explicit `--home` supplies the `${HOME}` replacement used by log, Cookie, database, and proxy paths. Without the flag, ncmctl uses the OS user-home lookup, which commonly derives from the shell environment on Unix.
