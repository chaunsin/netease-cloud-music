---
name: ncmctl-dev
description: >-
  Repository-specific development workflow for github.com/chaunsin/netease-cloud-music and the
  ncmctl Go CLI. Use for code, tests, documentation, reviews, refactors, or debugging in this
  repository, including Cobra commands, API and crypto protocols, XEAPI and NCBL, configuration,
  cookies and database persistence, NCM decoding, account tasks, downloads and uploads, and the
  HTTP(S) monitoring proxy. Trigger when the user names this checkout or one of its Go packages,
  even without saying ncmctl. Do not use for general NetEase Music questions or user-only CLI help.
---

# ncmctl Development Guide

## Workflow

1. Read the repository-root `AGENTS.md` completely. It links to the canonical `CLAUDE.md` and owns repository-wide policy, test side effects, and documentation boundaries.
2. Run `git status --short`; inspect relevant staged, unstaged, and untracked contents. Preserve unrelated work.
3. Classify the task with the routing table and load only the matching resources. If no row matches, use root guidance plus the nearest source, callers, and tests without loading unrelated references.
4. Trace the nearest implementation, callers, tests, and public contract before editing. Derive protocol behavior from source, captures, history, or fixed vectors, not symptoms or guesses.
5. Reuse existing types and patterns and keep behavior stable unless requested otherwise. Before deleting, prove usage and remove associated exports, references, tests, and documentation.
6. Return errors with enough context to locate the failed operation. Log only at cleanup or asynchronous boundaries where an error cannot be returned.
7. Validate the smallest safe scope described by the selected route or nearest tests, then apply the root completion checklist.

## Reference routing

| Task | Read |
| --- | --- |
| Change a Cobra command, root lifecycle, scheduler, concurrent file command, or CLI test | `references/commands.md` |
| Change config loading, runtime paths, cookies, logs, CookieCloud, or database persistence | `references/configuration.md` |
| Add or change an endpoint, `api.Client`, request options, auxiliary HTTP client, or TLS transport | `references/api-guide.md` |
| Change WEAPI, EAPI, Linux API, XEAPI, NCBL, or other crypto/wire behavior | `references/protocols.md` |
| Change proxy forwarding, capture, decoding, redaction, output, CA handling, or shutdown | `references/proxy.md` |
| Change NCM decoding or tags | `pkg/ncm/`, `pkg/ncm/tag/`, and their tests |
| Verify user-facing flags, examples, config, proxy, or debugging behavior | Repository-root `skills/ncmctl/references/commands.md` and freshly built Cobra help |
| Verify installation, login, logout, or authentication behavior | Repository-root `skills/ncmctl/references/install-and-login.md` and freshly built Cobra help |
| Change the user Skill quick reference or safety boundaries | Repository-root `skills/ncmctl/SKILL.md` |
| Change build, CI, dependencies, release files, or an unmatched package | Root guidance, then the nearest implementation and tests; use `Makefile`, `go.mod`, or `.github/` only as applicable |

Load multiple references only for a real cross-cutting change. For example, read `commands.md` with `configuration.md` for root bootstrap, `proxy.md` with `protocols.md` for proxy decoding, or `configuration.md` with `api-guide.md` for CookieCloud transport.

For XEAPI wire changes, read `protocols.md`, then the current-status table and only the relevant research section in repository-root `docs/xeapi.md`. For NCBL, use `protocols.md`, `pkg/crypto/ncbl.go`, and `pkg/crypto/ncbl_test.go`; NCBL is not part of XEAPI.

## Evidence by task

- Treat source plus focused tests as the current implementation contract.
- Treat generated Cobra help, `Makefile`, `go.mod`, and `config/config.yaml` as executable documentation sources for their respective surfaces.
- Require captures or independently sourced fixed vectors for protocol compatibility claims; a local encrypt/decrypt round trip is insufficient by itself.
- Update only documentation directly affected by the requested behavior. Report unrelated drift instead of broadening scope.
