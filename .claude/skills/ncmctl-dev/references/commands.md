# Command Reference

## Table of Contents

- [login](#login)
- [logout](#logout)
- [task](#task)
- [sign](#sign)
- [partner](#partner)
- [scrobble](#scrobble)
- [download](#download)
- [cloud](#cloud)
- [ncm](#ncm)
- [crypto](#crypto)
- [curl](#curl)

## login

Login to NetEase Cloud Music. Supports multiple methods.

### Subcommands

| Subcommand | Status | Description |
|------------|--------|-------------|
| `phone` | Risk control issues | SMS or password login |
| `cookie` | Working | Import cookie string or file |
| `cookiecloud` | Working | Sync from CookieCloud extension |
| `qrcode` | Deprecated | QR code scan (blocked by 8821 verification) |

### Usage

```bash
# Phone SMS login
ncmctl login phone 188xxx8888

# Phone password login
ncmctl login phone 188xxx8888 -p 123456

# Cookie string login
ncmctl login cookie 'cookie_string'

# Cookie file login (supports header/json/netscape formats)
ncmctl login cookie -f cookie.txt

# CookieCloud login
ncmctl login cookiecloud -u <user> -p <password> -s http://0.0.0.0:8088
```

### Implementation

- `internal/ncmctl/login.go` - Parent command
- `internal/ncmctl/login_phone.go` - Phone login
- `internal/ncmctl/login_cookie.go` - Cookie import
- `internal/ncmctl/login_cookiecloud.go` - CookieCloud sync
- `internal/ncmctl/login_qrcode.go` - QR code (deprecated)

## logout

Clear stored login credentials.

```bash
ncmctl logout
```

Implementation: `internal/ncmctl/logout.go`

## task

Run daily tasks on a cron schedule as a long-running service.

```bash
# Run all tasks (sign + partner + scrobble)
ncmctl task

# Selective execution
ncmctl task --sign --scrobble

# Custom cron schedule
ncmctl task --scrobble.cron "0 20 * * *"

# Custom timezone
ncmctl task -l America/New_York
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--sign` | false | Enable sign task |
| `--partner` | false | Enable partner task |
| `--scrobble` | false | Enable scrobble task |
| `--runAll` | false | Enable all tasks |
| `--sign.cron` | `0 10 * * *` | Sign cron expression |
| `--partner.cron` | `0 18 * * *` | Partner cron expression |
| `--scrobble.cron` | `0 18 * * *` | Scrobble cron expression |
| `--sign.automatic` | false | Auto-claim sign rewards (ban risk!) |
| `--partner.star` | `3,4` | Base song score range (1-5) |
| `--partner.extStar` | `2,3,4` | Extra song score range (1-5) |
| `--partner.extNum` | `random` | Extra evaluation count (2-7 or number) |
| `--scrobble.num` | 300 | Scrobble song count |
| `-l` | `Asia/Shanghai` | Timezone |

Implementation: `internal/ncmctl/task.go`

## sign

Single execution of daily check-in (YunBei + VIP).

```bash
ncmctl sign
ncmctl sign -a  # Auto-claim rewards (ban risk!)
```

### What it does

1. YunBei sign-in (云贝签到)
2. If `--automatic`: claim sign-in rewards and complete YunBei tasks
3. VIP grow point check
4. VIP task sign (黑胶乐签)
5. If `--automatic`: claim VIP growth rewards

Implementation: `internal/ncmctl/sign.go`

## partner

Music partner auto-evaluation.

```bash
ncmctl partner
ncmctl partner -s 3,4 -e 2,3,4
ncmctl partner -n 5
```

### Flow

1. Check partner qualification (`PartnerUserinfo`)
2. Get daily 5 base songs (`PartnerDailyTask`)
3. For each song: simulate listening (15-25s random delay) → report play → evaluate with random score
4. Get extra task songs (`PartnerExtraTask`)
5. Evaluate extra songs (2-7 random count)

Implementation: `internal/ncmctl/partner.go`

## scrobble

Scrobble songs to increase listen count.

```bash
ncmctl scrobble
ncmctl scrobble -n 200
```

### Flow

1. Get user info and check level (skip if max level 10)
2. Check today's scrobble count from database
3. Get Top list playlists
4. For each playlist: get track IDs, filter already-heard songs via database
5. Submit play logs via `WebLog` API
6. Record played songs in database for dedup

### Important

- Dedup data stored in `~/.ncmctl/database/badger/` — do not delete
- May not reach 300 if Top list songs are limited or already heard
- Uses `time.UntilMidnight` for daily counter expiry

Implementation: `internal/ncmctl/scrobble.go`

## download

Download songs, albums, playlists by ID or URL.

```bash
# Single song by URL
ncmctl download -l hires 'https://music.163.com/song?id=1820944399'

# Single song by ID
ncmctl download -l hires 1820944399

# Album
ncmctl download -p 5 'https://music.163.com/#/album?id=34608111'

# Artist
ncmctl download --strict 'https://music.163.com/#/artist?id=33400892'

# Playlist
ncmctl download 'https://music.163.com/playlist?id=593617579'

# Custom output
ncmctl download -l SQ 'song_url' -o ./download/
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o` | `./download` | Output directory |
| `-p` | 5 | Parallel downloads (max 20) |
| `-l` | `lossless` | Quality level |
| `--strict` | false | Skip if quality unavailable |
| `--tag` | true | Write audio tags |

### URL Parsing

The `Parse()` function in `internal/ncmctl/utils.go` extracts resource type (song/album/artist/playlist) and ID from URLs or plain IDs.

### Download Flow

1. Parse input → determine resource type and IDs
2. Fetch song details via `SongDetail` API
3. For each song: query quality → get download URL via `SongPlayerV1` → download with progress bar → verify MD5 → rename temp file

Implementation: `internal/ncmctl/download.go`

## cloud

Upload music files to NetEase cloud disk.

```bash
# Single file
ncmctl cloud '/path/to/music.mp3'

# Directory
ncmctl cloud '/path/to/music/'

# With filters
ncmctl cloud -p 5 -m 1MB -r '.*\.flac$' '/path/to/music/'
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-p` | 3 | Parallel uploads (max 10) |
| `-m` | none | Minimum file size (e.g., 1MB, 500KB) |
| `-r` | none | Filename regex filter |

### Upload Flow

1. Read file and compute MD5
2. Check if upload needed (`CloudUploadCheck`)
3. Get upload token (`CloudTokenAlloc`)
4. Upload file data (`CloudUpload`)
5. Submit metadata (`CloudInfo`)
6. Check transcoding status (`CloudMusicStatus`, retry up to 3 times)
7. Publish to account (`CloudPublish`)

### Constraints

- Max file size: 500MB
- Max directory depth: 3
- Only music file extensions accepted

Implementation: `internal/ncmctl/cloud.go`

## ncm

Decrypt `.ncm` encrypted files to playable formats.

```bash
# Single file
ncmctl ncm '/path/to/file.ncm' -o ./output

# Directory (batch)
ncmctl ncm '/path/to/ncm/files' -o ./output -p 10
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o` | `./ncm` | Output directory |
| `-p` | 10 | Parallel decryption (1-50) |
| `--tag` | false | Disable tag writing (tags written by default) |

### NCM Format

Core decryption in `pkg/ncm/ncm.go`:
1. Read magic header
2. Decrypt RC4 key using AES-128-ECB
3. Decrypt metadata using AES-128-ECB (JSON with song info)
4. Stream-decode audio data using RC4 cipher

Audio tag handling in `pkg/ncm/tag/` supports MP3 (ID3v2), FLAC (Vorbis), WAV.

Implementation: `internal/ncmctl/ncm.go`

## crypto

Encrypt/decrypt API parameters for debugging.

```bash
# Encrypt
ncmctl crypto encrypt -k weapi '{"key":"value"}'

# Decrypt
ncmctl crypto decrypt -k eapi 'ciphertext'

# Decrypt from HAR file
ncmctl crypto decrypt http_request.har
```

Implementation: `internal/ncmctl/crypto.go`, `crypto_encrypt.go`, `crypto_decrypt.go`

## curl

Invoke API methods directly, like curl but with auto encryption.

```bash
ncmctl curl -k weapi -d '{}' Ping
ncmctl curl -k eapi -d '{"id":"123"}' SongDetail
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-m` | auto | HTTP method |
| `-d` | `{}` | Request JSON body |
| `-o` | none | Output file path |
| `-k` | `weapi` | API kind: weapi/eapi/linux/api |
| `-t` | 15s | Request timeout |

Uses Go reflection to find and call the method on the API struct.

Implementation: `internal/ncmctl/curl.go`
