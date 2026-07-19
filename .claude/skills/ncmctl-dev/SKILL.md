---
name: ncmctl-dev
description: >-
  Repository development guide for github.com/chaunsin/netease-cloud-music and the ncmctl Go CLI.
  Use this skill for any code, test, documentation, review, refactor, or debugging work in this
  repository, including Cobra commands, WEAPI/EAPI/Linux/API endpoint packages, XEAPI transport,
  crypto and NCBL,
  cookies, CookieCloud, Badger, NCM decoding, downloads, cloud uploads, scheduled account tasks,
  and the HTTP(S) monitoring proxy. Trigger even when the user names only a repository path or Go
  package rather than ncmctl. Do not use it for general NetEase music questions unrelated to this codebase.
---

# ncmctl Development Guide

Use this skill to navigate the repository safely and load only the task-specific details needed for the current change.

## Start here

1. Read the repository-root `AGENTS.md` completely. It is a symlink to the canonical `CLAUDE.md` and defines test side effects, architecture, protocol invariants, and documentation ownership.
2. Run `git status --short`. Preserve unrelated staged, unstaged, and untracked work.
3. Inspect the nearest implementation, callers, and tests before editing. Protocol behavior must come from source, captures, history, or fixed vectors rather than UI symptoms or guesses.
4. Select the smallest relevant reference below; do not load both reference files by default.

## Reference routing

| Task | Read |
| --- | --- |
| Add or change a Cobra command, command lifecycle, scheduled task, or CLI test | `references/commands.md` |
| Look up user-facing flags and examples | Repository-root `skills/ncmctl/references/commands.md` |
| Add or change an API endpoint, client option, cookie/config/database path, or crypto mode | `references/api-guide.md` |
| Change XEAPI wire behavior | The XEAPI section of `references/api-guide.md`, then the current-status table and relevant research sections in repository-root `docs/xeapi.md` |
| Change NCBL wire behavior | The NCBL section of `references/api-guide.md`, `pkg/crypto/ncbl.go`, and `pkg/crypto/ncbl_test.go`; NCBL is not part of `docs/xeapi.md` |
| Change the proxy | Root `AGENTS.md`, `internal/proxy/`, and focused proxy tests; use the user command reference only for public flags |
| Change NCM decoding or tags | `pkg/ncm/`, `pkg/ncm/tag/`, and their tests |
| Update only installation or login guidance | Repository-root `skills/ncmctl/references/install-and-login.md` |

## Evidence by task

- **Current CLI behavior:** inspect the implementation, focused tests, and help from a freshly built binary. Cobra source and generated help should agree.
- **Protocol compatibility:** combine captures or independently sourced fixed vectors with implementation and tests. A self-consistent implementation test does not outrank wire evidence.
- **Build and configuration:** use `Makefile`, `go.mod`, `config/config.yaml`, and the active loader code.
- **Documentation:** use root guidance for repository policy and references for task-specific detail; README prose is not executable evidence.

Update only documentation directly affected by the requested behavior change. Report unrelated drift instead of expanding the task.

## Validation selection

Start narrow and expand only when safe:

```bash
# Examples of offline-focused checks
go test ./api ./pkg/crypto
go test ./internal/ncmctl
go test ./internal/proxy
go test -race ./internal/proxy

# Repository checks
make fmt
make lint
git diff --check
```

Do not run `go test ./...` or `make test` automatically. Untagged tests under `api/weapi` and `api/eapi` call live NetEase services and may act on an account when a valid cookie exists. Tests under `example/` require `-tags=integration` and can log in, upload, or download. Run those only with explicit authorization for their network and account effects.

`make lintfix` is a broad mutating operation. Inspect its diff and do not change `.golangci.yaml` merely to silence source findings unless lint policy is the task.

## Finish

- Follow root `AGENTS.md` for documentation ownership and synchronization triggers.
- Validate each skill changed by the task; do not validate an untouched skill merely because it exists in the repository.
- Check relative links only in changed documentation and confirm a canonical symlink only when the task touched its source or link.

Use an available skill validator for changed skills, then run the relevant focused tests and diff checks.
