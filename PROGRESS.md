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
| 04 | Authenticated reverse proxy and bridge | Complete | committed; final hash reported in handoff |
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
| 2026-10-17 | 04 | Spikes 4.1, 4.2; classification contract | Started | Adding proxy configuration and the checked-in deny-by-default operation classification; live RunWisp OpenAPI is unavailable locally |
| 2026-10-17 | 04 | Spikes 4.1, 4.2; classification contract | Completed | Added landing/public-path/bridge configuration and `internal/proxy/classification.json`; recorded the unavailable `runwisp openapi` spike and classified known 1.7 operation families |
| 2026-10-17 | 04 | Reverse proxy core | Started | Reusing the daemon transport with credential stripping, trusted forwarding headers, body limits, HTML/SSE-aware proxy behavior, and upstream cookie removal |
| 2026-10-17 | 04 | Reverse proxy core | Completed | Added `internal/proxy` transport reuse, request sanitization, bounded bodies, upstream cookie removal, response header defaults, and bad-gateway handling |
| 2026-10-17 | 04 | Classification + RBAC middleware | Started | Connecting proxy classification to Phase 03 sessions and role checks, with safe HTML/API denial responses and landing selection |
| 2026-10-17 | 04 | Classification + RBAC middleware | Completed | Wired the authenticated root proxy, deny-by-default path classifier, viewer/operator/editor gates, public paths, manager landing redirect, and denial logging |
| 2026-10-17 | 04 | Status synthesis | Started | Adding the role-aware auth status response and sensitive-field stripping contract |
| 2026-10-17 | 04 | Status synthesis | Completed | Synthesized daemon auth status with authenticated/read-only role semantics, OIDC display name, and credential-field removal |
| 2026-10-17 | 04 | HTML injection + header handling | Started | Adding decompression-safe bridge injection, cache/content-length fixes, and origin-header preservation rules |
| 2026-10-17 | 04 | HTML injection + header handling | Completed | Added HTML injection at `</head>`/`</body>` fallback, gzip decoding, bounded pages, no-store behavior, and missing-only security headers |
| 2026-10-17 | 04 | SSE hardening + session-expiry closer | Started | Adding unbuffered streaming behavior and cancellation when the authenticated session expires or is revoked |
| 2026-10-17 | 04 | SSE hardening + session-expiry closer | Completed | Added no-buffer SSE response handling, compression suppression, and periodic session validity cancellation for long-lived streams |
| 2026-10-17 | 04 | Bridge | Started | Adding the dependency-free Shadow DOM bridge endpoint, role-aware task edit link, route awareness, and sign-out behavior |
| 2026-10-17 | 04 | Bridge | Completed | Added `/manager/bridge.js` with Shadow DOM pill, manager/edit/sign-out links, route hooks, 401 login redirect, position config, and disable toggle |
| 2026-10-17 | 04 | Tests + docs | Started | Adding proxy/bridge contract tests, refreshing environment documentation, and preparing full Go/UI verification |
| 2026-10-17 | 04 | Tests + docs | Completed | Added proxy classification, header, gzip injection, and bridge tests; full Go tests, race tests, and vet passed; UI checks/live daemon e2e remain environment-limited |
| 2026-10-17 | 04 | Final verification | Started | Running the final Go 1.27/race/vet/build/UI checks and reviewing the phase diff before commit |
| 2026-10-17 | 04 | Final verification | Completed | Go 1.27 tests, race tests, vet, CGO-disabled build, Svelte check, and diff check passed; Svelte retained one pre-existing unused `.theme` CSS warning; live `runwisp openapi`, daemon E2E, and govulncheck/gosec are unavailable or sandbox-limited |
| 2026-10-17 | 05 | Discovery + include graph + path safety | Started | Building the canonical config-file graph, include expansion, cycle/depth warnings, and managed-area safety checks |
| 2026-10-17 | 05 | Discovery + include graph + path safety | Completed | Added canonical root-relative include expansion with doublestar globs, cycle/depth/file limits, symlink boundary checks, stable file IDs, versions, and malformed-file warnings; unit tests cover nested includes, outside symlinks, and partial discovery |
| 2026-10-17 | 05 | Unit indexer | Started | Decoding readable TOML tables into stable units with line ranges, hashes, unknown-table handling, and duplicate detection |
| 2026-10-17 | 05 | Unit indexer | Completed | Added BOM/CRLF-tolerant TOML indexing, schema-oriented kinds, unknown-table fallback, display line ranges, canonical unit hashes, defensive field copies, and duplicate markers |
| 2026-10-17 | 05 | Masking + reveal + leak tests | Started | Adding recursive secret heuristics, reference exemptions, viewer-safe field output, and audited reveal support |
| 2026-10-17 | 05 | Masking + reveal + leak tests | Completed | Added recursive heuristic masking with reference exemptions, defensive JSON output, exact key-path reveal, and planted-secret serialization tests |
| 2026-10-17 | 05 | Watcher + versions + SSE event source | Started | Adding fsnotify directory watching, polling fallback, debounce, cancellation, and tree-change events |
| 2026-10-17 | 05 | Watcher + versions + SSE event source | Completed | Added cancelable fsnotify directory watching with 200ms debounce, two-second polling mode, stable tree/file versions, bounded events, and atomic-replace coverage; dependency cache/network limitation was resolved with approved download |
| 2026-10-17 | 05 | REST handlers + OpenAPI | Started | Wiring viewer/admin config-tree, file, unit, reveal, and SSE endpoints with opaque IDs and audit-safe responses |
| 2026-10-17 | 05 | REST handlers + OpenAPI | Completed | Added authenticated tree/file/unit/reveal/event endpoints, opaque file IDs, admin-only raw/reveal controls with rate limiting and audit emission, SSE session checks, and a checked-in Phase 05 OpenAPI path contract |
| 2026-10-17 | 05 | Read-only UI pages | Started | Adding config overview, task/service lists and details, file tree view, stale/warning states, and service alias navigation |
| 2026-10-17 | 05 | Read-only UI pages | Completed | Added overview counts/stale and warning states, task/service list/detail views, settings file graph, service alias navigation, read-only fields, and rebuilt embedded UI assets; Svelte check/build pass with one pre-existing `.theme` warning |
| 2026-10-17 | 05 | Final verification and phase review | Started | Running full Go tests, race/vet/build checks, web verification, secret-exposure review, and final diff inspection |
| 2026-10-17 | 05 | Final verification and phase review | Completed | Go 1.27 full tests, new-package race tests, vet, CGO-disabled build, Svelte check, and production web build passed; source diff check passed, while generated minified UI output retains one internal trailing-space warning; live daemon/UI E2E, govulncheck, and gosec remain unavailable or sandbox-limited; inherited `.theme` Svelte warning remains |
