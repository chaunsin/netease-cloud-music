# Installation & Login Guide

## Table of Contents

- [Installation](#installation)
- [Login Guide](#login-guide)

## Installation

### Method 1: Download Pre-built Binary

Download from [GitHub Releases](https://github.com/chaunsin/netease-cloud-music/releases) for your platform.

### Method 2: Install from Source

```bash
# Direct install
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest

# Clone and build
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make install
```

Default install path: `$GOPATH/bin`

Requires Go >= 1.24.0

### Method 3: Docker

```bash
# Docker Hub
docker pull chaunsin/ncmctl:latest

# GitHub Container Registry
docker pull ghcr.io/chaunsin/ncmctl:latest
```

Docker login (first time):

```bash
docker run --rm -it -v ./data:/root chaunsin/ncmctl:latest /app/ncmctl login cookie -f /root/cookie.txt
```

Docker run tasks:

```bash
docker run -it -d -v ./data:/root chaunsin/ncmctl:latest /app/ncmctl task --sign --scrobble
```

Build image locally:

```bash
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make build-image
```

### Method 4: Qinglong Panel

See [Qinglong Guide](https://github.com/chaunsin/netease-cloud-music/blob/master/docs/qinglong.md).

## Login Guide

ncmctl supports 5 login methods. Cookie login is the most reliable.

### 1. Cookie Login (Recommended)

When other login methods fail, use cookie login as fallback.

Get cookies via browser extension like [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl).

```bash
# Import cookie string directly
ncmctl login cookie 'cookie_string_content'

# Import from file (auto-detect format)
ncmctl login cookie -f cookie.txt

# Specify format
ncmctl login cookie --format json -f cookie.json
ncmctl login cookie --format netscape -f cookie.txt
ncmctl login cookie --format header -f cookie.txt
```

**Supported file formats:**

| Format       | Description                                   |
| ------------ | --------------------------------------------- |
| `header`   | `key1=value1; key2=value2` style            |
| `json`     | Cookie Editor JSON export                     |
| `netscape` | Netscape cookies.txt format                   |
| (default)    | Auto-detect: tries netscape → json → header |

**Cookie must contain `MUSIC_U` field**, otherwise login will fail.

### 2. CookieCloud Login

[CookieCloud](https://github.com/easychen/CookieCloud) is a browser extension that syncs encrypted cookies to cloud.

Steps:

1. Install CookieCloud browser extension
2. Configure the extension
3. Login to NetEase Cloud Music web player
4. Click "Manual Sync" in the extension
5. Run login command:

```bash
ncmctl login cookiecloud -u <uuid> -p <password> -s http://127.0.0.1:8088
```

| Flag               | Default                   | Description                                  |
| ------------------ | ------------------------- | -------------------------------------------- |
| `-s, --server`   | `http://127.0.0.1:8088` | CookieCloud server address                   |
| `-u, --uuid`     | (required)                | Account UUID                                 |
| `-p, --password` | (required)                | Account password                             |
| `-t, --timeout`  | 30s                       | Request timeout                              |
| `-H, --headers`  | none                      | Custom headers, e.g.,`key1=val1,key2=val2` |

### 3. Phone SMS Login

```bash
ncmctl login phone 188xxx8888
```

After sending SMS, enter the captcha when prompted. SMS has daily limits; avoid frequent logins.

| Flag              | Default | Description   |
| ----------------- | ------- | ------------- |
| `--countrycode` | 86      | Country code  |
| `-t, --timeout` | 10m     | Login timeout |

### 4. Phone Password Login

```bash
ncmctl login phone 188xxx8888 -p your_password
```

| Flag               | Default | Description    |
| ------------------ | ------- | -------------- |
| `-p, --password` | none    | Login password |
| `--countrycode`  | 86      | Country code   |
| `-t, --timeout`  | 10m     | Login timeout  |

May trigger `8821 behavior verification` error due to risk control. Use as fallback only.

### 5. QR Code Login

Scan QR code with NetEase Cloud Music mobile app to login.

```bash
ncmctl login qrcode
```

After running the command:
1. A QR code image (`qrcode.png`) is generated in the current directory
2. The QR code is also printed to the terminal
3. Open NetEase Cloud Music app on your phone and scan the QR code
4. Confirm the login on your phone
5. Login completes automatically after scanning

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --timeout` | 5m | Login timeout |
| `-d, --dir` | `./` | QR code image output directory |
| `-l, --level` | 1 | QR code recovery level: 0→7%, 1→15%, 2→25%, 3→30% |

**QR code check status codes:**

| Code | Meaning |
|------|---------|
| 800 | QR code expired or cancelled |
| 801 | Waiting for scan |
| 802 | Scanned, waiting for confirmation |
| 803 | Login successful |

If the QR code expires (code 800), re-run the command to generate a new one.

### Logout

```bash
ncmctl logout
```

Clears stored credentials and removes `~/.ncmctl/cookie.json`.
