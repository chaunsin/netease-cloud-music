---
title: ncmctl Installation and Login Guide
description: Install, upgrade, authenticate, log out, and troubleshoot ncmctl safely.
version: "0.4.0"
---

# Installation and Login Guide

## Contents

- [Install](#install)
- [Upgrade](#upgrade)
- [Runtime home](#runtime-home)
- [Login methods](#login-methods)
- [Log out](#log-out)
- [Troubleshooting](#troubleshooting)

## Install

### Prebuilt binary

Download the archive for the user's OS and architecture from the project's [GitHub Releases](https://github.com/chaunsin/netease-cloud-music/releases), extract `ncmctl`, place it on `PATH`, and verify it:

```bash
ncmctl --version
ncmctl --help
```

### Go install

Go 1.25.0 or newer is required:

```bash
go version
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest
```

The binary is normally installed in `$(go env GOPATH)/bin`. Add that directory to `PATH` if the shell cannot find `ncmctl`.

### Clone and install

```bash
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music
make install
```

### Docker

Published images:

```bash
docker pull chaunsin/ncmctl:latest
docker pull ghcr.io/chaunsin/ncmctl:latest
```

Persist `/root/.ncmctl` by mounting a host directory at `/root`:

```bash
mkdir -p ./data
chmod 700 ./data

# cookie.txt must exist at ./data/cookie.txt on the host
docker run --rm -it \
  -v "$PWD/data:/root" \
  chaunsin/ncmctl:latest \
  /app/ncmctl login cookie -f /root/cookie.txt

docker run -d \
  --name ncmctl-daily \
  --restart unless-stopped \
  -v "$PWD/data:/root" \
  chaunsin/ncmctl:latest \
  /app/ncmctl task --sign --scrobble
```

The mounted directory contains authentication and database data. Do not use a shared or world-readable host path.

Build the image locally with:

```bash
make build-image
```

### Qinglong

Use the repository's [Qinglong guide](https://github.com/chaunsin/netease-cloud-music/blob/master/docs/qinglong.md). Its `NCMCTL_QINGLONG_*` variables belong to the Qinglong wrapper scripts; they are not general credential variables implemented by the ncmctl binary.

## Upgrade

Re-run the installation mechanism and verify the resulting version:

```bash
# Go installation
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest

# Docker installation
docker pull chaunsin/ncmctl:latest

ncmctl --version
```

For a cloned checkout, fetch the desired revision and run `make install`. Preserve `<home>/.ncmctl/`; replacing the binary should not require deleting cookies or the Badger database.

## Runtime home

The global `--home` value is substituted for `${HOME}` in the default runtime paths:

```bash
ncmctl --home /srv/ncmctl COMMAND
```

Default authentication data is stored at `<home>/.ncmctl/cookie.json`. The Cookie directory and file are created with restrictive POSIX permissions, but copied/exported files and Docker volumes remain the user's responsibility.

The optional `--config` flag loads one exact YAML path; ncmctl does not auto-load `<home>/.ncmctl/config.yaml`. Copy the complete schema from `config/config.yaml`, because omitted sections are not merged from the embedded defaults. Unknown fields are rejected and `NCMCTL_` environment variables can override loaded values.

## Login methods

ncmctl exposes four login subcommands and five user flows because `login phone` supports either SMS or password. Login contacts live NetEase services and persists cookies after success.

Prefer Cookie-file login when it is available: it avoids placing a full Cookie string or password directly in shell history. All authentication material is sensitive.

### Cookie file or string

The imported cookies must include `MUSIC_U`.

```bash
# Prefer a protected file
chmod 600 cookie.txt
ncmctl login cookie -f cookie.txt

# Direct string; convenient but stored in shell history on many shells
ncmctl login cookie 'MUSIC_U=...; __csrf=...'

# Explicit formats when auto-detection is unsuitable
ncmctl login cookie --format json -f cookies.json
ncmctl login cookie --format netscape -f cookies.txt
ncmctl login cookie --format header 'MUSIC_U=...; __csrf=...'
```

| Format | Input |
| --- | --- |
| `header` | Semicolon-separated `name=value` pairs |
| `json` | Cookie Editor-style JSON array |
| `netscape` | Netscape/cookies.txt format |
| empty | Auto-detect Netscape, JSON, then header |

Each explicit format accepts either `-f <file>` or a positional string. Auto-detection creates a fresh reader for every candidate format, so JSON strings and header files follow the same order as other inputs.

Use the spelling `netscape` for `--format`, even though older help examples contained the typo `netscaple`.

Do not post exported cookies in issues, logs, screenshots, or chat. Logging out of the browser can invalidate a previously exported Cookie.

Cookie and CookieCloud imports are first checked with an in-memory Cookie jar. ncmctl writes them to the configured Cookie file only after the account endpoint returns code 200 with both account and profile data.

### CookieCloud

CookieCloud syncs browser cookies through a configured server. First log in to the NetEase web player and perform a manual CookieCloud sync, then run:

```bash
ncmctl login cookiecloud \
  --uuid '<uuid>' \
  --password '<end-to-end-password>' \
  --server 'http://127.0.0.1:8088'
```

| Flag | Default | Description |
| --- | --- | --- |
| `-u, --uuid` | required | CookieCloud account UUID |
| `-p, --password` | required | CookieCloud password |
| `-s, --server` | `http://127.0.0.1:8088` | CookieCloud server URL |
| `-t, --timeout` | `30s` | Request timeout |
| `-H, --headers` | none | Comma-separated `key=value` request headers |

The current command requires UUID and password flags. It does not prompt for them and does not read dedicated `COOKIECLOUD_*` variables. These values can be visible in shell history and process listings, so run only on a trusted machine and prefer Cookie-file login if that exposure is unacceptable. Treat third-party CookieCloud servers as credential-bearing services.

### Phone SMS

```bash
ncmctl login phone 188xxx8888
```

The command sends an SMS, prompts for the captcha, verifies it, then completes login.

| Flag | Default | Description |
| --- | --- | --- |
| `--countrycode` | 86 | Telephone country code |
| `-t, --timeout` | `10m` | Network request deadline; it does not interrupt the terminal while waiting for captcha input |

SMS sending has service limits and can trigger risk control. Avoid repeated attempts.

### Phone password

```bash
ncmctl login phone 188xxx8888 --password '<password>'
```

The current command has no hidden password prompt or dedicated password environment variable. Omitting `--password` selects SMS login; it does not prompt for a password. The flag can be visible in shell history and process listings, and the request may fail with code 8821 or another behavior-verification response. Prefer Cookie or SMS login when possible.

### QR code

```bash
ncmctl login qrcode
ncmctl login qrcode --timeout 5m --dir ./private-qr
```

| Flag | Default | Description |
| --- | --- | --- |
| `-t, --timeout` | `5m` | Login timeout |
| `-d, --dir` | current directory | QR image directory |
| `-l, --level` | 1 | Recovery level: 0=7%, 1=15%, 2=25%, 3=30% |

The command writes `qrcode.png` with restrictive permissions and prints the QR content in the terminal. Scan it with the NetEase Cloud Music mobile app and confirm the login. The image is removed after a successful login; it may remain after failure, cancellation, or timeout and should then be deleted securely.

Status codes reported by the remote login flow:

| Code | Meaning |
| --- | --- |
| 800 | Expired, missing, or cancelled |
| 801 | Waiting for scan |
| 802 | Scanned, waiting for confirmation |
| 803 | Authorized successfully |

Re-run the command if the code expires. Repeated login attempts can trigger risk control.

## Log out

```bash
ncmctl logout
```

The command calls the remote logout endpoint, flushes the updated Cookie jar, and then removes `<home>/.ncmctl/cookie.json` so the final flush cannot recreate it. With `--home`, it removes the corresponding path under that home. A custom Cookie filepath loaded through `--config` is not the path removed by the current logout implementation; remove that file separately if necessary.

## Troubleshooting

| Problem | Check |
| --- | --- |
| `ncmctl: command not found` | Add `$(go env GOPATH)/bin` or the extracted binary directory to `PATH` |
| Go version error | Install Go 1.25.0 or newer |
| `cookie not found MUSIC_U value` | Export a fresh authenticated Cookie containing `MUSIC_U` |
| Cookie login fails after browser logout | Re-authenticate in the browser and export again |
| CookieCloud says no matching Cookie | Confirm web login, manual sync, UUID/password/server, and a `music.163.com` Cookie containing `MUSIC_U` |
| Code 8821 / behavior verification | Stop repeated attempts and use a different supported login flow later |
| QR expires | Re-run `login qrcode`; confirm within the timeout |
| Docker login is not persisted | Mount the same host directory at `/root` for login and later commands |
| Permission denied in Docker | Verify ownership and restrictive permissions of the host-mounted directory |
| Custom config is not discovered automatically | Pass its exact path with `--config`; automatic discovery is unsupported |
| Custom config reports unknown or missing fields | Start from the complete `config/config.yaml` schema and change only the required values |

For exact flags on the installed version, run `ncmctl login <method> --help`.
