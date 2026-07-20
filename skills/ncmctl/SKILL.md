---
name: ncmctl
description: >-
  Install, configure, operate, and troubleshoot the ncmctl command-line client for NetEase Cloud
  Music. Use this skill when the user mentions ncmctl or asks for CLI-based NetEase login, Cookie or
  CookieCloud import, QR/SMS login, scheduled sign/partner/scrobble tasks, song download, cloud-disk
  upload, NCM-to-MP3/FLAC decryption, API crypto/curl debugging, Docker deployment, or the local
  HTTP(S) capture proxy. Also use it for ncmctl flags, shell completion, config files, runtime paths,
  credentials, and proxy CA safety. Do not trigger for general music recommendations, official app usage, or repository
  development; the repository-local ncmctl-dev skill covers source changes.
---

# ncmctl User Guide

ncmctl is a Go CLI for NetEase Cloud Music login, scheduled account tasks, media download/upload, NCM decryption, API debugging, and local HTTP(S) traffic monitoring.

## How to use this skill

1. Identify whether the user needs installation/login help or a command reference.
2. Start with the matching reference file listed at the end. Load the other only when the request crosses installation/login and command/config concerns.
3. Prefer the installed binary's `ncmctl <command> --help` output when available; it is the exact syntax for that version.
4. Explain network, credential, filesystem, and account effects before suggesting a command that causes them.
5. Never invent flags, interactive prompts, environment variables, exit codes, or decryption capabilities that are not documented here or shown by the binary.

Do not perform an action that changes an account or credentials, writes or deletes files, modifies a trust store, starts a listener, or launches a long-running service unless the user explicitly requested that effect. Representative commands include login/logout, account tasks, media operations, `curl`, and `proxy`.

## Installation check

```bash
ncmctl --version
ncmctl --help
```

If the command is missing, read `references/install-and-login.md`. Source installation requires Go 1.25.0 or newer:

```bash
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest
```

Prebuilt binaries are published on the project's GitHub Releases page.

## Command map

| Command | Login | Purpose |
| --- | --- | --- |
| `login` | No | Phone/SMS, password, Cookie, CookieCloud, or QR login |
| `logout` | Existing session | Log out and remove the default persisted Cookie file |
| `task` | Yes | Run sign, partner, and/or scrobble on cron schedules |
| `sign` | Yes | Run YunBei and eligible VIP daily sign-in actions once |
| `partner` | Yes | Submit music-partner evaluations once |
| `scrobble` | Yes | Submit play logs, up to 300 per day |
| `download` | Yes | Download songs, albums, artists, or playlists |
| `cloud` | Yes | Upload local audio to the account's cloud disk |
| `ncm` | No | Decode local `.ncm` files to playable audio |
| `crypto` | No | Inspect supported API encryption formats |
| `curl` | Depends on API | Invoke an exported API wrapper method by name |
| `proxy` | No | Monitor the user's own NetEase HTTP(S) traffic |
| `completion` | No | Generate shell completion for bash, fish, PowerShell, or zsh |

Read `references/commands.md` for flags, limits, side effects, and examples.

## Global flags and runtime data

| Flag | Default | Meaning |
| --- | --- | --- |
| `--debug` | false | Enable debug/stdout logging and network debug |
| `-c, --config` | none | Select an exact complete YAML file; see the schema in `references/commands.md` |
| `--home` | OS user home | Base value substituted for `${HOME}` in runtime paths |

Without `--config`, ncmctl uses its embedded defaults; it does not automatically load `~/.ncmctl/config.yaml`.

Default runtime paths under `<home>`:

| Data | Path |
| --- | --- |
| Cookies | `<home>/.ncmctl/cookie.json` |
| Badger database | `<home>/.ncmctl/database/badger/` |
| Logs | `<home>/.ncmctl/log/ncm.log` |
| Proxy CA certificate | `<home>/.ncmctl/proxy/ca.crt` |
| Proxy CA private key | `<home>/.ncmctl/proxy/ca.key` |

For custom configuration, copy the full schema from `config/config.yaml`, edit it, and pass the path explicitly with `--config`.

## Safety boundaries

- **Account risk:** `scrobble`, partner evaluation, automatic reward claims, and other automation can trigger NetEase risk control. Scrobble has a particularly high ban risk.
- **Credentials:** Cookie values, `MUSIC_U`, phone passwords, and CookieCloud UUID/passwords are secrets. The current phone-password and CookieCloud commands accept credentials as flags; they do not provide a hidden password prompt or dedicated credential environment variable.
- **Cookie files:** ncmctl creates its default Cookie directory/file with restrictive permissions on POSIX, but backups and exported Cookie files remain sensitive. Prefer `login cookie -f` over placing a Cookie string directly in shell history.
- **Proxy CA:** Trust only `ca.crt` on a client you control. Never install, share, or commit `ca.key`. Remove trust when monitoring is finished if it is no longer needed.
- **Sensitive capture:** Proxy redaction is enabled by default. `--show-sensitive` can expose credentials and identifiers in the terminal or redirected files.
- **LAN proxy:** `--listen 0.0.0.0:9000` is unauthenticated. Use it only temporarily on a trusted network behind a firewall.
- **Local files:** Download, upload, NCM decode, QR login, HAR processing, and redirected proxy output read or write local files. Confirm paths before running them.

## Common workflows

### Schedule sign and scrobble

```bash
ncmctl task --sign --scrobble
```

`task` is a long-running service. With no selectors it registers sign, partner, and scrobble; explicit selectors limit the jobs. Press Ctrl+C or send SIGTERM to stop it.

### Download a playlist

```bash
ncmctl download -l lossless \
  'https://music.163.com/playlist?id=593617579' \
  -o ./music
```

### Decode local NCM files

```bash
ncmctl ncm '/path/to/ncm/files' -o ./decoded -p 10
```

The historical `ncm --tag` flag is inverted: tags are written by default, and passing `--tag` disables tag writing.

### Monitor local API traffic

```bash
ncmctl proxy
ncmctl proxy > capture.log
```

Configure the client to use `127.0.0.1:9000` for HTTP and HTTPS, then trust `<home>/.ncmctl/proxy/ca.crt`. The proxy never modifies the system trust store automatically.

Use `ncmctl --debug proxy` to correlate the CONNECT target with ClientHello SNI, the generated certificate SANs, and hostname-match results. Matching identities followed by a client handshake alert narrow the remaining causes to client trust policy or certificate pinning; the alert alone cannot distinguish them.

## References

| File | Read when |
| --- | --- |
| `references/install-and-login.md` | Installing, upgrading, logging in/out, or troubleshooting authentication |
| `references/commands.md` | Looking up flags, command behavior, config schema, proxy limitations, or debugging tools |
