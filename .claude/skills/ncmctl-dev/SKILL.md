---
name: ncmctl-dev
description: >-
  Repository-specific development workflow for github.com/chaunsin/netease-cloud-music and the
  ncmctl Go CLI. Use for code, tests, documentation, reviews, refactors, or debugging in this
  repository, including Cobra commands, API and crypto protocols, XEAPI (Aegis request encryption)
  and NCBL (version 3 log-body wire format), configuration, cookies and database persistence, NCM
  decoding, account tasks, downloads and uploads, and the HTTP(S) monitoring proxy. Trigger when
  the user names this checkout or any Go package within it (e.g. pkg/crypto, internal/proxy,
  pkg/ncm, api/weapi), even without saying ncmctl. Do not use for general NetEase Music questions
  or user-only CLI help.
---

# ncmctl Development Guide

## Workflow

1. `AGENTS.md` (repository root) is already loaded as a user rule — confirm its repository-wide policy, test side effects, and documentation boundaries apply before proceeding.
2. Run `git status --short`; inspect relevant staged, unstaged, and untracked contents. Preserve unrelated work.
3. Classify the task with the routing table and load only the matching resources. If multiple rows match, load all relevant references (see cross-cutting examples below). If no row matches, use root guidance plus the nearest source, callers, and tests.
4. Trace the nearest implementation, callers, tests, and public contract before editing. For protocol behavior, see the evidence requirements in `references/protocols.md`; derive behavior from source, captures, or fixed vectors — not symptoms or guesses.
5. Reuse existing types and patterns and keep behavior stable unless requested otherwise. Before deleting, prove usage and remove associated exports, references, tests, and documentation.
6. Return errors with enough context to locate the failed operation. Log only at cleanup or asynchronous boundaries where an error cannot be returned.
7. Validate the smallest safe scope described by the selected route or nearest tests, then apply the root completion checklist (compile, tests + race, lint — see `AGENTS.md` § 完成检查).

## Reference routing

The rows below are the primary routing targets. The last two rows in the "Verify" group reference the **user-facing skill** at `skills/ncmctl/` (not this development skill); they are cross-skill references.

| Task | Read |
| --- | --- |
| Change a Cobra command, root lifecycle, scheduler, concurrent file command, or CLI test | `references/commands.md` |
| Change config loading, runtime paths, cookies, logs, CookieCloud (`pkg/cookiecloud`), or database persistence | `references/configuration.md` |
| Add or change an endpoint, `api.Client`, request options, auxiliary HTTP client (`pkg/alert`, `pkg/cookiecloud`), or TLS transport | `references/api-guide.md` |
| Change WEAPI, EAPI, Linux API, XEAPI, NCBL, or other crypto/wire behavior | `references/protocols.md` |
| Change proxy forwarding, capture, decoding, redaction, output, CA handling, or shutdown | `references/proxy.md` |
| Change NCM decoding or tags | `pkg/ncm/`, `pkg/ncm/tag/`, and their tests |
| Verify user-facing flags, examples, config, proxy, or debugging behavior | *(user skill)* `skills/ncmctl/references/commands.md` and freshly built Cobra help |
| Verify installation, login, logout, or authentication behavior | *(user skill)* `skills/ncmctl/references/install-and-login.md` and freshly built Cobra help |
| Change the user Skill quick reference or safety boundaries | *(user skill)* `skills/ncmctl/SKILL.md` |
| Change build, CI, dependencies, release files, or an unmatched package | Root guidance, then the nearest implementation and tests; use `Makefile`, `go.mod`, or `.github/` only as applicable |

**Cross-cutting examples:** read `commands.md` + `configuration.md` for root bootstrap; `proxy.md` + `protocols.md` for proxy decoding; `configuration.md` + `api-guide.md` for CookieCloud transport.

**XEAPI:** read `protocols.md`, then the current-status table and only the relevant research section in `docs/xeapi.md`. **NCBL:** use `protocols.md`, `pkg/crypto/ncbl.go`, and `pkg/crypto/ncbl_test.go`; NCBL is independent of XEAPI.

## Evidence by task

- Treat source plus focused tests as the current implementation contract.
- Treat generated Cobra help, `Makefile`, `go.mod`, and `config/config.yaml` as executable documentation sources for their respective surfaces.
- For protocol compatibility claims, require captures or independently sourced fixed vectors; see `references/protocols.md` § Evidence requirements for details.
- Update only documentation directly affected by the requested behavior. Report unrelated drift instead of broadening scope.
