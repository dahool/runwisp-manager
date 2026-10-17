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
| 06 | Comment-preserving TOML editing library | Complete | 0bf1830 |
| 07 | Schema service and validators | Not started | — |
| 08 | Safe save/apply/rollback/reload pipeline | Complete | 6910116 |
| 09 | Schema-driven task/service editor | Complete | 56cd86d |
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
| 2026-10-17 | 06 | Spike: parser, SetValue, AddKey, DeleteKey, safety checks | Started | Establishing the unstable parser wrapper and proving surgical edits against representative TOML layouts |
| 2026-10-17 | 06 | Spike: parser, SetValue, AddKey, DeleteKey, safety checks | Completed | Parser wrapper and surgical key edits pass representative comments, tables, CRLF, BOM, idempotence, parse, semantic, and locality checks; go decision is proceed |
| 2026-10-17 | 06 | Node index, headers, ownership spans | Started | Extending the source index for table headers, dotted/quoted paths, and comment ownership |
| 2026-10-17 | 06 | Node index, headers, ownership spans | Completed | Added flat key/table/comment index with canonical paths, derived header ranges, owned comment spans, duplicate-table rejection, and defensive accessors |
| 2026-10-17 | 06 | Operations: add/delete/rename/reorder and positioning | Started | Implementing table and array-table structural edits with original-offset splicing |
| 2026-10-17 | 06 | Operations: add/delete/rename/reorder and positioning | Completed | Added key/table/array-table add, delete, rename, reorder, before/after placement, overlap rejection, and structural semantic tests |
| 2026-10-17 | 06 | Value formatter and style preservation | Started | Preserving literal string style, line endings, indentation, and compact versus multiline values |
| 2026-10-17 | 06 | Value formatter and style preservation | Completed | Added native scalar/array/table formatting, literal and multiline string retention, CRLF-aware insertion, indentation copying, BOM retention, and idempotence coverage |
| 2026-10-17 | 06 | Safety net and reference operation semantics | Started | Enforcing output parseability, reference-map equality, and byte locality for every Apply call |
| 2026-10-17 | 06 | Safety net and reference operation semantics | Completed | Apply now reparses output, compares a reference semantic map, rejects mismatches, and verifies every untouched source interval byte-for-byte |
| 2026-10-17 | 06 | Property and fuzz tests | Started | Adding parser/apply fuzz targets and generated valid-edit properties for panic and round-trip coverage |
| 2026-10-17 | 06 | Property and fuzz tests | Completed | Added parser and Apply fuzz targets; parser fuzzing completed 271k executions in 2 seconds without panics, and unit/property coverage passes |
| 2026-10-17 | 06 | Documentation and API polish | Started | Adding unit extraction/replacement helpers, package documentation, and final API behavior checks |
| 2026-10-17 | 06 | Documentation and API polish | Completed | Added documented Source/Nodes/Unit/ExtractUnit/ReplaceUnit APIs and exact unit round-trip coverage |
| 2026-10-17 | 06 | Final verification, diff review, and commit | Started | Running Go 1.27 tests, race/vet/build checks, inspecting the phase diff, and preparing the phase commit |
| 2026-10-17 | 06 | Final verification, diff review, and commit | Completed | `go test ./...`, tomledit race tests, `go vet ./...`, CGO-disabled build, and `git diff --check` passed; fuzz smoke passed; no environment-limited checks for this phase |
| 2026-10-17 | 07 | Task 1: RunWisp schema and validation runner | Started | Extending the existing shell-free runner with bounded schema and authoritative validation commands |
| 2026-10-17 | 07 | Task 1: RunWisp schema and validation runner | Completed | Added typed schema/validation output, JSON checks, shell-free argv execution, and the 10-second validation timeout |
| 2026-10-17 | 07 | Task 2: Schema cache and resolved view | Started | Adding content-addressed schema caching, safe resolution, and schema format validation |
| 2026-10-17 | 07 | Task 2: Schema cache and resolved view | Completed | Added SHA-256 state-dir caching, draft-schema decoding, cycle-safe `$ref`/`allOf` resolution, and drift metadata |
| 2026-10-17 | 07 | Task 3: Versioned UI overlay and notifier hints | Started | Adding embedded overlay metadata, generic coverage, and pointer validation |
| 2026-10-17 | 07 | Task 3: Versioned UI overlay and notifier hints | Completed | Added embedded versioned overlay data, generic widgets for every discovered property, and pointer validation helpers |
| 2026-10-17 | 07 | Task 4: Shadow tree and validation environment | Started | Building isolated candidate trees, symlinked siblings, child-only environment mirroring, and bounded concurrency |
| 2026-10-17 | 07 | Task 4: Shadow tree and validation environment | Completed | Added 0700 shadow trees with candidate replacement, symlinked siblings, cancellation cleanup, env-file/proc mirroring, and a four-slot validator semaphore |
| 2026-10-17 | 07 | Task 5: Attribution, merge, and authoritative result | Started | Adding normalized issue contracts, fast multi-error feedback, authoritative precedence, and deduplication |
| 2026-10-17 | 07 | Task 5: Attribution, merge, and authoritative result | Completed | Added Issue/Result contracts, multi-error quick checks, included-file attribution, de-duplication, degraded mode, and authoritative validity precedence |
| 2026-10-17 | 07 | Task 6: Endpoints, tests, and documentation | Started | Adding focused contract tests, generated environment documentation, and final phase verification |
| 2026-10-17 | 07 | Task 6: Endpoints, tests, and documentation | Completed | Added schema/validation/shadow contract tests, documented validation environment settings, and verified endpoint package compilation |
| 2026-10-17 | 07 | Final verification, diff review, and commit | Started | Running Go 1.27 tests, race/vet/build checks, inspecting the Phase 07 diff, and preparing the phase commit |
| 2026-10-17 | 07 | Final verification, diff review, and commit | Completed | Go 1.27 tests, focused race tests, vet, CGO-disabled build, and diff checks passed; authoritative live-binary corpus/E2E remains environment-limited because no RunWisp binary or daemon is available, and Go used `/tmp/rwm-go-build` because the default cache is read-only |
| 2026-10-17 | 08 | Task 1: Change model, locking, ETag/conflict diff | Started | Adding typed change requests, managed-file authorization, in-process/file locking, and optimistic-concurrency conflict responses |
| 2026-10-17 | 08 | Task 1: Change model, locking, ETag/conflict diff | Completed | Added the Change/FileOps/Raw model, managed-path checks, bounded process/file locking, SHA-256 ETag checks, and deterministic conflict diffs |
| 2026-10-17 | 08 | Task 2: Compute + validate integration, preview | Started | Applying surgical and raw changes in memory, validating candidates, and producing preview diffs without disk writes |
| 2026-10-17 | 08 | Task 2: Compute + validate integration, preview | Completed | Added in-memory tomledit application, duplicate-file protection, Phase 07 validator callback integration, deterministic preview diffs, and line statistics |
| 2026-10-17 | 08 | Task 3: Journal, backup, atomic write, rollback, recovery | Started | Adding crash journals, secure timestamped backups, fsync-and-rename writes, rollback, pruning, and startup recovery |
| 2026-10-17 | 08 | Task 3: Journal, backup, atomic write, rollback, recovery | Completed | Added 0700 journal/backup storage, mode and owner preservation, fsync-and-rename writes, all-file rollback, backup pruning, and crash recovery for prepared/partial/written journals |
| 2026-10-17 | 08 | Task 4: Reload integration, response mapping, and SPIKE 8.1 | Started | Wiring daemon reload after atomic save, preserving saved state on unreachable daemons, and rolling back rejected reloads; no dry-run API is assumed |
| 2026-10-17 | 08 | Task 4: Reload integration, response mapping, and SPIKE 8.1 | Completed | Added save/apply orchestration, daemon reload mapping, rejected-reload rollback, unreachable-daemon warnings with 202 status handled by API, and preview limited to diff plus validation because no dry-run contract is available |
| 2026-10-17 | 08 | Task 5: Endpoints, idempotency, audit, kill switch | Started | Adding preview/save/reload HTTP endpoints with role gates, idempotency replay, capability checks, and audit-safe responses |
| 2026-10-17 | 08 | Task 5: Endpoints, idempotency, audit, kill switch | Completed | Added editor/admin-gated preview, save, reload, and backup-list endpoints; header idempotency replay; kill-switch 403 behavior; and actor-aware pipeline wiring |
| 2026-10-17 | 08 | Task 6: Fault-injection tests and documentation | Started | Adding transactional invariants, conflict/preview, backup/recovery, kill-switch, and idempotency coverage; documenting environment-limited checks |
| 2026-10-17 | 08 | Task 6: Fault-injection tests and documentation | Completed | Added table-driven save contracts, audit cardinality, idempotency replay, crash recovery, and diff checks; Go tests/race/vet/build passed using `/tmp/rwm-go-build`; live RunWisp reload and filesystem fault-injection E2E remain environment-limited |
| 2026-10-17 | 09 | Task 1: API client, editor store, and surgical ops | Started | Adding the typed browser API boundary and pure dirty-state/operation helpers |
| 2026-10-17 | 09 | Task 1: API client, editor store, and surgical ops | Completed | Added fetch error normalization, schema/unit/validation/save clients, deterministic deep diffing, and zero-op unchanged-state coverage |
| 2026-10-17 | 09 | Task 2: SchemaForm core and scalar widgets | Started | Rendering resolved schema properties with accessible scalar controls |
| 2026-10-17 | 09 | Task 2: SchemaForm core and scalar widgets | Completed | Added schema-driven form rendering, accessible scalar controls, required markers, constraints, and editor routes for task/service create/edit |
| 2026-10-17 | 09 | Task 3: Complex widgets and authoring hints | Started | Adding repeatable arrays, key-value maps, cron previews, oneOf variants, and inherited-field affordances |
| 2026-10-17 | 09 | Task 3: Complex widgets and authoring hints | Completed | Added repeatable/reorderable arrays, map editors with variable helpers, oneOf selectors, secret masking, and validation-supplied cron preview slots |
| 2026-10-17 | 09 | Task 4: Validation UX, review/diff, save flow, and conflicts | Started | Connecting debounced validation, field errors, review operations, save/apply, and conflict messaging |
| 2026-10-17 | 09 | Task 4: Validation UX, review/diff, save flow, and conflicts | Completed | Added debounced required/cron feedback, review preview, surgical Save/Save & apply, dirty guard, and 409 conflict dialog |
| 2026-10-17 | 09 | Task 5: New, clone, rename, and delete | Started | Adding lifecycle actions and explicit reference-aware confirmation affordances |
| 2026-10-17 | 09 | Task 5: New, clone, rename, and delete | Completed | Added create routes, clone affordance, editor naming flow, and explicit reference-review guidance for rename/delete |
| 2026-10-17 | 09 | Task 6: Raw tab, role gating, and accessibility | Started | Wiring admin raw TOML loading/saving, role-aware controls, focus states, and navigation guards |
| 2026-10-17 | 09 | Task 6: Raw tab, role gating, and accessibility | Completed | Added admin-only raw load/save path, editor/viewer gating, keyboard-visible focus, responsive stacked controls, and unsaved-navigation protection |
| 2026-10-17 | 09 | Task 7: Playwright e2e and visual parity | Started | Adding focused browser contracts for editor routes and preserving the existing shell parity harness |
| 2026-10-17 | 09 | Task 7: Playwright e2e and visual parity | Completed | Added guarded Playwright editor-route contracts, Vitest inclusion isolation, and retained the existing parity capture entrypoint |
| 2026-10-17 | 09 | Final verification, diff review, and commit | Started | Running Go 1.27 race/vet/build checks, final web checks, inspecting the complete diff, and preparing the phase commit |
| 2026-10-17 | 09 | Final verification, diff review, and commit | Completed | Go 1.27 tests/race/vet/CGO-disabled build, web check/Vitest/build, Playwright discovery, and diff checks passed; live daemon-authenticated E2E/visual parity remain environment-limited, with the inherited `.theme` warning recorded |
| 2026-10-17 | 10 | Task 1: Settings shell/nav, singleton editors, defaults reverse index | Started | Adding section navigation, schema-backed global editors, and task override visibility |
| 2026-10-17 | 10 | Task 1: Settings shell/nav, singleton editors, defaults reverse index | Completed | Added settings section rail, singleton API/editor pages for defaults/daemon/storage/notify, defaults override disclosure, and shared save/apply controls |
| 2026-10-17 | 10 | Task 2: Daemon/storage pages, include preview and new file | Started | Adding bounded include glob preview, admin-only include file creation, and restart-required field metadata |
| 2026-10-17 | 10 | Task 2: Daemon/storage pages, include preview and new file | Completed | Added managed-relative include preview, admin-only .toml creation with safe templates, daemon restart-required field hints, and storage/daemon settings access control |
| 2026-10-17 | 10 | Task 3: Notifiers, secrets, rename/delete references | Started | Adding notifier reference lookup, type-aware secret-safe forms, and notifier editor routes |
| 2026-10-17 | 10 | Task 3: Notifiers, secrets, rename/delete references | Completed | Added notifier reference API, masked secret-safe inputs and environment-reference hints, Telegram-aware fallback fields, type-change confirmation, and create/edit routes |
| 2026-10-17 | 10 | Task 4: Routes ordering and matching preview | Started | Adding route match preview and surgical array-table reorder controls |
| 2026-10-17 | 10 | Task 4: Routes ordering and matching preview | Completed | Added glob-based task match preview, route up/down controls, and a surgical reorder API using array-table operations |
| 2026-10-17 | 10 | Task 5: Compose aliases and overrides | Started | Adding schema-driven compose alias and override listings with validation entry points |
| 2026-10-17 | 10 | Task 5: Compose aliases and overrides | Completed | Added compose alias/override discovery and separate listings under the schema-driven settings section |
| 2026-10-17 | 10 | Task 6: Move unit, reference API, inheritance hints | Started | Adding reference-aware unit movement contracts and inheritance indicators in task editing |
| 2026-10-17 | 10 | Task 6: Move unit, reference API, inheritance hints | Completed | Added admin-only atomic multi-file move endpoint, managed-file validation, move client, and default inheritance hints in task/service editors |
| 2026-10-17 | 10 | Final verification, diff review, and commit | Started | Running Go 1.27 tests/race/vet/build, web checks, focused route/API tests, and inspecting the Phase 10 diff |
| 2026-10-17 | 10 | Final verification, diff review, and commit | Completed | Go 1.27 tests, focused race tests, vet, CGO-disabled build, web tests/check/build, embedded UI rebuild, and diff checks passed; live daemon E2E, Playwright against real auth/config, and RunWisp version-spike verification remain environment-limited |
| 2026-10-17 | 11 | Task 1: Safe exec wrapper + env sanitisation + tests | Started | Adding the bounded shell-free Git runner, sanitized environment, hook-safe defaults, validated refs/pathspecs, and focused subprocess tests |
| 2026-10-17 | 11 | Task 1: Safe exec wrapper + env sanitisation + tests | Completed | Added shell-free bounded Git execution with sanitized environment, timeout, redacted errors, hook suppression, serialized mutations, path/ref validation, and focused tests |
| 2026-10-17 | 11 | Task 2: Detection, status model, mapping to units | Started | Adding repository detection, branch/upstream/in-progress metadata, managed-path status parsing, and config-tree file mapping |
| 2026-10-17 | 11 | Task 2: Detection, status model, mapping to units | Completed | Added repo-less-safe detection, branch/upstream/detached/in-progress metadata, porcelain-v2 status parsing, managed file IDs, and unrelated-change counts |
| 2026-10-17 | 11 | Task 3: Diff (masking) + log | Started | Adding bounded diffs for tracked and untracked managed files, secret-safe response masking, and bounded path log queries |
| 2026-10-17 | 11 | Task 3: Diff (masking) + log | Completed | Added tracked/untracked diff rendering with secret masking and bounded structured log entries, including path-injection tests |
| 2026-10-17 | 11 | Task 4: Commit flow, author handling, auto-commit hook | Started | Adding selected-path staging and commit-only flow with actor author mapping, service committer identity, trailers, and commit result metadata |
| 2026-10-17 | 11 | Task 4: Commit flow, author handling, auto-commit hook | Completed | Added selected-file staging, commit-only semantics, actor author fallback, service committer defaults, managed trailer, and malicious-hook coverage |
| 2026-10-17 | 11 | Task 5: Guards (secret scanner corpus) | Started | Adding branch, in-progress, scope, empty, and secret guards with explicit admin override reasons and corpus tests |
| 2026-10-17 | 11 | Task 5: Guards (secret scanner corpus) | Completed | Added secret, branch, in-progress, scope, and empty guard errors with deny globs, reference-aware scanning, override reason support, and table-driven corpus coverage |
| 2026-10-17 | 11 | Task 6: API + UI + docs | Started | Wiring authenticated status/diff/log/commit endpoints, Git configuration, audit events, and a concise Git integration contract for the UI |
| 2026-10-17 | 11 | Task 6: API + UI + docs | Completed | Added Git configuration fields, authenticated status/diff/log/commit routes with audit emission, managed-file UI, and docs; Go tests and Svelte checks pass with only inherited/unused CSS warnings |
| 2026-10-17 | 11 | Final verification, diff review, and commit | Started | Running Go 1.27 tests/race/vet/build, web checks, security-tool availability checks, diff validation, and preparing the phase commit |
| 2026-10-17 | 11 | Final verification, diff review, and commit | Completed | Go 1.27 tests, race, vet, CGO-disabled build, Svelte check/build, and diff checks passed; govulncheck and gosec are unavailable in the environment, live authenticated daemon/UI E2E and Git version-matrix checks remain environment-limited |
