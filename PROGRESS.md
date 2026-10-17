# RunWisp Manager Progress

This file is the implementation ledger. Phase workers must update it when
starting and completing each task, and must commit the completed phase.

## Baseline

- Go toolchain: `go version go1.27.0 linux/amd64`
- Overview reviewed: `00-README-overview.md`
- Phase documents present: 01–05, 07–10, 12–14
- Missing phase documents: 06 and 11; these remain blocked until their plans
  are supplied.

## Phase status

| Phase | Outcome | Status | Commit |
| --- | --- | --- | --- |
| 01 | Go binary serving RunWisp-identical empty shell | Not started | — |
| 02 | Env config, daemon client, discovery, version check | Complete | 96beec4aad9e6250cd830b19df7f607c2cbfdfab |
| 03 | OIDC login, sessions, roles, CSRF | Complete | 25f2930 |
| 04 | Authenticated reverse proxy and bridge | Not started | — |
| 05 | Multi-file config index and read model | Not started | — |
| 06 | Comment-preserving TOML editing library | Blocked: document missing | — |
| 07 | Schema service and validators | Not started | — |
| 08 | Safe save/apply/rollback/reload pipeline | Not started | — |
| 09 | Schema-driven task/service editor | Not started | — |
| 10 | Global settings editors | Not started | — |
| 11 | Git status, diff, commit, guards | Blocked: document missing | — |
| 12 | Git pull/update, push, conflicts, polling | Not started | — |
| 13 | Audit log, history, rollback | Not started | — |
| 14 | Packaging, hardening, release | Not started | — |

## Activity log

| Timestamp | Phase | Task | Status | Notes |
| --- | --- | --- | --- | --- |
| 2026-10-17 | — | Progress tracker created | Complete | Awaiting Phase 01 worker |
| 2026-10-17 | 01 | Go module and service skeleton | Started | Creating the command entrypoint, version flags, and package stubs |
| 2026-10-17 | 01 | Go module and service skeleton | Completed | Added go.mod, cmd/runwisp-manager, and internal package stubs |
| 2026-10-17 | 01 | SvelteKit web shell | Started | Setting up the fixed /manager app, theme persistence, and RunWisp UI dependencies |
| 2026-10-17 | 01 | SvelteKit web shell | Completed | Added SvelteKit/Tailwind configuration, shared theme bootstrap, shell layout, and placeholder routes |
| 2026-10-17 | 01 | Embedded UI handler | Started | Adding the static asset embed, SPA fallback rules, and cache behavior |
| 2026-10-17 | 01 | Embedded UI handler | Completed | Added go:embed serving, SPA fallback rules, cache headers, API problem responses, and tests |
| 2026-10-17 | 01 | HTTP server and health endpoint | Started | Adding chi routing, request IDs, access logging, recovery, headers, and health JSON |
| 2026-10-17 | 01 | HTTP server and health endpoint | Completed | Added chi router, request IDs, token-safe access logs, recovery, security headers, health JSON, and tests |
| 2026-10-17 | 01 | Build, CI, and parity harness | Started | Adding reproducible Make targets, static checks, GitHub Actions, and the browser parity script |
| 2026-10-17 | 01 | Build, CI, and parity harness | Completed | Added Makefile, ignore/lint config, CI workflow, and light/dark parity capture harness; live parity execution is environment-blocked without Bun/Chromium and a RunWisp demo |
| 2026-10-17 | 01 | Final verification | Blocked | Go unit tests, vet, Svelte check, and production build passed; live socket smoke test is blocked by the sandbox's bind restriction, and the final combined rerun was interrupted. Changes are committed with these limitations recorded. |
| 2026-10-17 | 02 | Configuration and environment documentation | Started | Adding typed environment loading, secret-file support, typo detection, cross-field validation, and generated environment documentation |
| 2026-10-17 | 02 | Configuration and environment documentation | Completed | Added typed config loading, `_FILE` secrets, unknown-variable suggestions, aggregated validation, redacted effective output, and generated environment documentation |
| 2026-10-17 | 02 | RunWisp binary runner and version policy | Started | Adding bounded command execution, version parsing, and supported-version policy |
| 2026-10-17 | 02 | RunWisp binary runner and version policy | Completed | Added shell-free context-bound runner, capped stdout/stderr capture, version extraction, and >=1.7.0 <2.0.0 policy tests |
| 2026-10-17 | 02 | CHAP authentication helper | Started | Adding the specified PBKDF2 response helper and vectors |
| 2026-10-17 | 02 | CHAP authentication helper | Completed | Added PBKDF2-HMAC-SHA256 response generation and the two required vectors |
| 2026-10-17 | 02 | Daemon transports and API client | Started | Adding Unix and HTTP clients, CHAP login, bearer authentication, retries, and typed API models |
| 2026-10-17 | 02 | Daemon transports and API client | Completed | Added Unix socket and HTTP clients, bearer-authenticated JSON calls, single-flight CHAP login, TLS CA support, 401 retry, and Retry-After handling |
| 2026-10-17 | 02 | Discovery and capabilities | Started | Adding daemon discovery, config readability/writability probes, version matching, edit capability, and periodic refresh |
| 2026-10-17 | 02 | Discovery and capabilities | Completed | Added capability refresh, Unix identity/HTTP info discovery, version policy, config probes, edit gating, and cancellable 30-second refresh |
| 2026-10-17 | 02 | Manager status and readiness endpoints | Started | Adding status/readiness responses and trusted-proxy real-IP handling |
| 2026-10-17 | 02 | Manager status and readiness endpoints | Completed | Added unauthenticated status and readiness endpoints plus trusted-proxy-gated forwarded client IP handling |
| 2026-10-17 | 02 | UI daemon status banner | Started | Connecting the Phase 01 shell to the manager status endpoint and showing daemon reachability |
| 2026-10-17 | 02 | UI daemon status banner | Completed | Added a resilient status fetch and visible connected/unreachable daemon indicator to the manager shell |
| 2026-10-17 | 02 | Phase 02 verification and review | Started | Running formatting, generation, Go 1.27 tests/vet, UI checks, and inspecting the final diff |
| 2026-10-17 | 02 | Phase 02 verification and review | Completed | Go 1.27 race tests, vet, generated environment docs, check-config smoke tests, Svelte check, and git diff check passed; Svelte retained one pre-existing unused CSS warning |
| 2026-10-17 | 03 | Config structs + validation + check-config OIDC checks | Started | Adding OIDC, session, role, gate, and disabled-mode configuration with static validation and discovery checks |
| 2026-10-17 | 03 | Config structs + validation + check-config OIDC checks | Completed | Added typed OIDC/session/role/gate settings, strict auth validation for serve/check-config, disabled-mode loopback enforcement, and secret redaction compatibility |
| 2026-10-17 | 03 | SQLite store (sessions, flows, janitor) | Started | Implementing WAL-backed auth state with HMAC session IDs, encrypted ID-token hints, single-use flows, expiry, and cleanup |
| 2026-10-17 | 03 | SQLite store (sessions, flows, janitor) | Completed | Added modernc SQLite schema/store, atomic flow consumption, absolute and idle session expiry, encrypted hints, hashed IDs, and cancellable janitor |
| 2026-10-17 | 03 | OIDC client wrapper | Started | Adding discovery, PKCE authorization, JWKS signature and claim verification, userinfo merging, custom CA, and provider logout metadata |
| 2026-10-17 | 03 | OIDC client wrapper | Completed | Added discovery metadata validation, PKCE authorization, JWT issuer/audience/expiry/nonce/signature checks, optional userinfo merge, JWKS caching, custom CA, and logout metadata |
| 2026-10-17 | 03 | Claim/role mapping + gates | Started | Normalizing provider claim shapes, applying email/domain gates, and resolving the highest configured role |
| 2026-10-17 | 03 | Claim/role mapping + gates | Completed | Added groups normalization for provider shapes, verified-email/domain/email gates, highest-role resolution, and deny-by-default behavior |
| 2026-10-17 | 03 | Handlers (login/callback/logout/me) + return_to validation | Started | Adding browser auth endpoints, opaque cookie sessions, safe redirects, logout provider redirects, and `/api/me` |
| 2026-10-17 | 03 | Handlers (login/callback/logout/me) + return_to validation | Completed | Added login/callback/logout/me handlers, single-use flow cookie binding, safe relative redirects, session cookie lifecycle, and HTML versus JSON errors |
| 2026-10-17 | 03 | Middleware: auth, RequireRole, CSRF, login/callback rate limit | Started | Wiring session authentication and role gates into the manager router with origin/content-type CSRF checks and bounded auth attempts |
| 2026-10-17 | 03 | Middleware: auth, RequireRole, CSRF, login/callback rate limit | Completed | Protected `/manager/api/status` and `/manager/api/me`, added role middleware, cookie-session auth, CSRF origin/referer checks, and 20-per-minute login/callback limiting |
| 2026-10-17 | 03 | Audit interface + slog sink | Started | Adding the audit event interface and emitting authentication lifecycle events without secrets or session identifiers |
| 2026-10-17 | 03 | Audit interface + slog sink | Completed | Added `audit.Event`, `audit.Emitter`, and structured slog sink; authentication emits login, logout, and denial events without credentials or session IDs |
| 2026-10-17 | 03 | Tests + fake IdP + docs | Started | Adding role/claim, redirect, CSRF, session expiry, and JWT integration coverage plus generated environment documentation |
| 2026-10-17 | 03 | Tests + fake IdP + docs | Completed | Added unit and fake-IdP JWT coverage, generated environment docs, Pocket ID deployment notes, and verified Go 1.27 tests/race/vet, config smoke checks, and diff; fake-IdP listener skipped by sandbox, live Pocket ID unavailable, govulncheck/gosec unavailable |
