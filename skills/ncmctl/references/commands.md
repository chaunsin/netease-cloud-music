# Command Reference

## Table of Contents

- [task](#task)
- [sign](#sign)
- [partner](#partner)
- [scrobble](#scrobble)
- [download](#download)
- [cloud](#cloud)
- [ncm](#ncm)
- [crypto](#crypto)
- [curl](#curl)

## task

Run daily tasks on a cron schedule as a long-running service. Requires login.

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

Runs as a service; press Ctrl+C to stop. Uses standard [crontab](https://crontab.guru/) expressions.

| Flag | Default | Description |
|------|---------|-------------|
| `--sign` | false | Enable sign task |
| `--partner` | false | Enable partner task |
| `--scrobble` | false | Enable scrobble task |
| `--runAll` | false | Enable all tasks |
| `--sign.cron` | `0 10 * * *` | Sign cron expression |
| `--partner.cron` | `0 18 * * *` | Partner cron expression |
| `--scrobble.cron` | `0 18 * * *` | Scrobble cron expression |
| `--sign.automatic` | false | Auto-claim sign rewards (**ban risk!**) |
| `--partner.star` | `3,4` | Base song score range (1-5) |
| `--partner.extStar` | `2,3,4` | Extra song score range (1-5) |
| `--partner.extNum` | `random` | Extra eval count: `random` (2-7) or number |
| `--scrobble.num` | 300 | Scrobble song count |
| `-l, --location` | `Asia/Shanghai` | Timezone |

## sign

Single execution of daily check-in (YunBei + VIP). Requires login.

```bash
ncmctl sign
ncmctl sign -a  # Auto-claim rewards (ban risk!)
```

| Flag | Default | Description |
|------|---------|-------------|
| `-a, --automatic` | false | Auto-claim sign-in rewards (**ban risk!**) |

Execution flow:
1. YunBei sign-in (云贝签到)
2. If `--automatic`: claim sign-in rewards and complete YunBei tasks
3. VIP grow point check
4. VIP task sign (黑胶乐签)
5. If `--automatic`: claim VIP growth rewards

## partner

Music partner auto-evaluation. Requires login and partner qualification.

```bash
ncmctl partner
ncmctl partner -s 3,4 -e 2,3,4
ncmctl partner -n 5
```

| Flag | Default | Description |
|------|---------|-------------|
| `-s, --star` | `3,4` | Base song score range (1-5, unique) |
| `-e, --extra` | `2,3,4` | Extra song score range (1-5, unique) |
| `-n, --num` | `random` | Extra eval count: `random` (2-7) or number (0-15) |

Execution flow:
1. Check partner qualification (`PartnerUserinfo`)
2. Get 5 base daily songs (`PartnerDailyTask`)
3. For each song: simulate listening (15-25s random delay) → report play → evaluate with random score
4. Get extra task songs (`PartnerExtraTask`)
5. Evaluate extra songs (2-7 random count)

Error code 703 = not a music partner. Code 405 = task already completed.

## scrobble

Scrobble songs to increase listen count. Requires login. **High ban risk!**

```bash
ncmctl scrobble
ncmctl scrobble -n 200
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --num` | 300 | Number of songs (1-300) |

Execution flow:
1. Get user info and check level (skip if max level 10)
2. Check today's scrobble count from database
3. Get Top list playlists
4. For each playlist: get track IDs, filter already-heard songs via database
5. Submit play logs via `WebLog` API
6. Record played songs in database for dedup

Dedup data in `~/.ncmctl/database/badger/` — do not delete. May not reach 300 if Top list songs are limited or already heard.

## download

Download songs, albums, playlists by ID or URL. Requires login.

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

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `./download` | Output directory |
| `-p, --parallel` | 5 | Parallel downloads (max 20) |
| `-l, --level` | `lossless` | Quality level (see below) |
| `--encode-type` | `flac` | Song encode type |
| `--immerse-type` | `c51` | Song immerse type |
| `--strict` | false | Skip if quality unavailable |
| `--tag` | true | Write audio tags |

**Quality levels:**

| Level | Aliases | Format |
|-------|---------|--------|
| `standard` | `128` | 128kbps |
| `higher` | `192` | 192kbps |
| `exhigh` | `HQ`, `320` | 320kbps |
| `lossless` | `SQ` | FLAC |
| `hires` | `HR` | Hi-Res |

**URL parsing:** Supports song/album/artist/playlist URLs or plain numeric IDs. The `Parse()` function extracts resource type and ID from input.

**Download flow:**
1. Parse input → determine resource type and IDs
2. Fetch song details via `SongDetail` API
3. For each song: query quality → get download URL via `SongPlayerV1` → download with progress bar → verify MD5 → rename temp file

## cloud

Upload music files to NetEase cloud disk. Requires login.

```bash
# Single file
ncmctl cloud '/path/to/music.mp3'

# Directory
ncmctl cloud '/path/to/music/'

# With filters
ncmctl cloud -p 5 -m 1MB -r '.*\.flac$' '/path/to/music/'
```

| Flag | Default | Description |
|------|---------|-------------|
| `-p, --parallel` | 3 | Parallel uploads (max 10) |
| `-m, --minsize` | none | Minimum file size (e.g., `1MB`, `500KB`) |
| `-r, --regexp` | none | Filename regex filter |

**Upload flow:**
1. Read file and compute MD5
2. Check if upload needed (`CloudUploadCheck`)
3. Get upload token (`CloudTokenAlloc`)
4. Upload file data (`CloudUpload`)
5. Submit metadata (`CloudInfo`)
6. Check transcoding status (`CloudMusicStatus`, retry up to 3 times)
7. Publish to account (`CloudPublish`)

**Constraints:** Max file size 500MB, max directory depth 3, only music file extensions.

## ncm

Decrypt `.ncm` encrypted files to playable formats. No login required.

```bash
# Single file
ncmctl ncm '/path/to/file.ncm' -o ./output

# Directory (batch)
ncmctl ncm '/path/to/ncm/files' -o ./output -p 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `./ncm` | Output directory |
| `-p, --parallel` | 10 | Parallel decryption (1-50) |
| `--tag` | false | Disable tag writing (tags written by default) |

**NCM format decryption:**
1. Read magic header
2. Decrypt RC4 key using AES-128-ECB
3. Decrypt metadata using AES-128-ECB (JSON with song info)
4. Stream-decode audio data using RC4 cipher

Audio tag handling supports MP3 (ID3v2), FLAC (Vorbis), WAV. Max directory depth 3.

## crypto

Encrypt/decrypt API parameters for debugging NetEase Cloud Music API traffic. No login required.

> **Note**: This is a debugging tool for analyzing API requests/responses. It is not for bypassing authentication or circumventing API protections. Use only for legitimate debugging of your own traffic.

```bash
# Encrypt
ncmctl crypto encrypt -k weapi '{"key":"value"}'

# Decrypt
ncmctl crypto decrypt -k eapi 'ciphertext'

# Decrypt from HAR file
ncmctl crypto decrypt http_request.har
```

| Flag | Default | Description |
|------|---------|-------------|
| `-k, --kind` | `weapi` | Mode: `weapi`/`eapi`/`linux` |
| `-o, --output` | none | Output file path |

Subcommands: `encrypt`, `decrypt`

## curl

Invoke NetEase Cloud Music API methods directly with auto encryption. No login required (but most APIs need it).

> **Note**: `ncmctl curl` is a subcommand of ncmctl for calling NetEase Cloud Music APIs, not the system `curl` tool. It handles API encryption automatically.

```bash
ncmctl curl -k weapi -d '{}' Ping
ncmctl curl -k eapi -d '{"id":"123"}' SongDetail
```

| Flag | Default | Description |
|------|---------|-------------|
| `-m, --method` | auto | HTTP method |
| `-d, --data` | `{}` | Request JSON body |
| `-o, --output` | none | Output file path |
| `-k, --kind` | `weapi` | API kind: `weapi`/`eapi`/`linux`/`api` |
| `-t, --timeout` | 15s | Request timeout |

Uses Go reflection to find and call the method on the API struct. The method name is the positional argument.
