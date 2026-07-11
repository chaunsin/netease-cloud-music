---
name: ncmctl
description: >-
  ncmctl CLI reference and usage guide for NetEase Cloud Music. Use this skill when the user
  mentions ncmctl, 网易云音乐命令行, 网易云音乐, 网易云, 网易音乐, NetEase Cloud Music CLI,
  or asks how to install, login, download songs, upload to cloud, decrypt NCM files, convert ncm to mp3/flac,
  run daily tasks (sign/partner/scrobble), 刷歌, 云贝签到, 音乐合伙人, 黑胶签到, use API debugging tools,
  or monitor HTTP/HTTPS traffic with a local proxy (抓包, 接口代理, MITM).
  Also trigger on questions about ncmctl configuration, cookie management, or Docker deployment.
  Use even if the user does not explicitly say "ncmctl" when the work is clearly related to NetEase Cloud Music CLI operations.
metadata:
  author: chaunsin
  version: "0.2.0"
---

# ncmctl - NetEase Cloud Music CLI

ncmctl is a Go CLI for NetEase Cloud Music: login, daily tasks, music download, cloud upload, NCM decryption, API debugging, and HTTP(S) traffic monitoring.

## Prerequisites

```bash
# Check if ncmctl is installed
ncmctl --version

# Install options:

# Go install (requires Go >= 1.25.0)
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest

# Or download pre-built binary from GitHub Releases
# https://github.com/chaunsin/netease-cloud-music/releases
```

## Quick Reference

| Command      | Login | Description                                                      |
| ------------ | ----- | ---------------------------------------------------------------- |
| `login`    | No    | Phone/Cookie/CookieCloud/QR code login                           |
| `logout`   | No    | Clear stored credentials                                         |
| `task`     | Yes   | Run daily tasks on cron schedule                                 |
| `sign`     | Yes   | YunBei + VIP daily check-in                                      |
| `partner`  | Yes   | Music partner auto-evaluation                                    |
| `scrobble` | Yes   | Scrobble songs daily (default 300, max 300)                      |
| `download` | Yes   | Download songs/albums/playlists                                  |
| `cloud`    | Yes   | Upload music to cloud disk                                       |
| `ncm`      | No    | Decrypt .ncm to .mp3/.flac                                       |
| `crypto`   | No    | Encrypt/decrypt API parameters (debugging only)                  |
| `curl`     | No    | Invoke API methods directly (ncmctl subcommand, not system curl) |
| `proxy`    | No    | Monitor NetEase HTTP(S) API requests and responses               |

## Global Flags

| Flag             | Default       | Description                     |
| ---------------- | ------------- | ------------------------------- |
| `--debug`      | false         | Enable debug mode               |
| `-c, --config` | none          | Config file path                |
| `--home`       | user home     | Base home used for runtime data  |

## Configuration Paths

| Item     | Default Path                   |
| -------- | ------------------------------ |
| Config   | `~/.ncmctl/config.yaml`      |
| Cookie   | `~/.ncmctl/cookie.json`      |
| Database | `~/.ncmctl/database/badger/` |
| Logs     | `~/.ncmctl/log/ncm.log`      |
| Proxy CA | `<home>/.ncmctl/proxy/ca.crt` |
| CA key   | `<home>/.ncmctl/proxy/ca.key` |

Env var prefix: `NCMCTL_` (e.g., `NCMCTL_LOG_LEVEL=debug`).

## Security Considerations

> **IMPORTANT**: ncmctl handles authentication credentials and performs actions on your NetEase Cloud Music account. Pay close attention to the following safety guidelines:

- **Never pass passwords on the command line** — they are visible in shell history and process listings. Prefer interactive prompts or environment variables.
- **Protect cookie files** — `~/.ncmctl/cookie.json` contains sensitive session credentials. Set file permissions immediately after login:
  ```bash
  chmod 600 ~/.ncmctl/cookie.json
  ```
- **Be cautious with CookieCloud credentials** — the UUID and password for CookieCloud login are sensitive; avoid sharing or logging them. Use environment variables or interactive prompts instead of command-line arguments.
- **Docker volume mounts** — when running in Docker, the mounted volume (`-v ./data:/root`) may expose credentials on the host filesystem. Ensure the host directory has restricted permissions:
  ```bash
  chmod 700 ./data
  ```
- **Account ban risks** — automated tasks (scrobble, sign automatic rewards, partner evaluation) may trigger NetEase risk control and result in account restrictions. Use at your own risk.
- **Protect the proxy CA key** — `<home>/.ncmctl/proxy/ca.key` can sign certificates trusted by devices where its CA is installed. Keep it private and never commit or share it.
- **Keep redaction enabled** — proxy output is redacted by default. `--show-sensitive` can expose cookies, tokens, phone numbers, email addresses, device identifiers, and passwords in terminal history or redirected files.

## Important Warnings

- Scrobble (刷歌) has high ban risk due to strict risk control
- `--sign.automatic` auto-claim rewards has ban risk, disabled by default
- Cookie persistence is interval-based (3s); unclean shutdown may lose recent cookies
- Do not delete `~/.ncmctl/database/` (scrobble dedup data)
- Directory depth limit: 3 for cloud upload and NCM decryption
- Cloud upload max file size: 500MB
- LAN proxy mode (`--listen 0.0.0.0:9000`) has no authentication; use it only temporarily on a trusted network

## Common Workflows

### Daily Automation (Sign + Scrobble)

```bash
# Run as a background service with cron scheduling
ncmctl task --sign --scrobble

# Or run commands individually
ncmctl sign
ncmctl scrobble -n 200
```

### Batch Download Playlist and Decrypt NCM

```bash
# Download a playlist
ncmctl download 'https://music.163.com/playlist?id=593617579' -o ./music/

# Decrypt downloaded NCM files
ncmctl ncm ./music/ -o ./decrypted/ -p 10
```

### Docker Scheduled Tasks

```bash
# Run daily tasks in Docker container
docker run -d -v ./data:/root \
  --name ncmctl-daily \
  --restart unless-stopped \
  chaunsin/ncmctl:latest \
  /app/ncmctl task --sign --scrobble
```

### Monitor API Traffic

```bash
# Start the local proxy and configure the client to use 127.0.0.1:9000
ncmctl proxy

# Save captured request/response blocks while keeping diagnostics on stderr
ncmctl proxy > capture.log
```

On first use, trust `<home>/.ncmctl/proxy/ca.crt` on the client device to inspect HTTPS. Here `<home>` is the global `--home` value. Do not install or share `ca.key`. See `references/commands.md` for LAN mode, custom CA flags, redaction, and protocol limitations.

## Reference Files

| File                                | Content                                     | When to read                                                         |
| ----------------------------------- | ------------------------------------------- | -------------------------------------------------------------------- |
| `references/install-and-login.md` | Installation methods and login procedures   | Setting up ncmctl for the first time or troubleshooting login issues |
| `references/commands.md`          | All command flags, parameters, and examples | Looking up detailed command syntax, flags, or execution flow         |
