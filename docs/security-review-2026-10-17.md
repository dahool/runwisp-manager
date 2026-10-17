# Security review — 2026-10-17

## Scope and result

Reviewed authentication and role code, reverse proxy classification,
configuration masking, save/rollback storage, Git execution and guards,
audit storage, UI HTML handling, Docker packaging, compose hardening, and CI
release permissions. No confirmed high or critical code defect was identified
in this pass. This is a source review and targeted test review, not a
deployment certification.

## Evidence reviewed

- CSRF and role-mapping table tests; OIDC JWT/provider tests.
- Proxy classification and sanitization tests; HTTP security-header tests.
- Config secret serialization and Git secret-guard/masked-diff tests.
- Atomic-save/journal tests and audit redaction/chain verification tests.
- Docker image declares an unprivileged UID, uses a read-only production
  manager root, `/tmp` tmpfs, dropped capabilities, and
  `no-new-privileges`; no Docker socket mount is present.
- CI scans Go with govulncheck and gosec, JavaScript dependencies with npm
  audit, container images with Trivy, and repository content with Gitleaks.
  Tagged releases request multi-architecture SBOM/provenance and keyless
  Cosign signing.

## Required privilege and operational decisions

- Editor/admin is equivalent to command execution as the RunWisp daemon user
  through the `run` task field. Deployment owners must keep that UID
  unprivileged and grant these roles narrowly.
- The manager and daemon must use the same UID; operators must set matching
  `RWM_UID`/`RWM_GID` values and compose `user` settings. Startup checks the
  owner of an existing Unix socket and exits with a UID-specific diagnostic
  on mismatch; if RunWisp has not created the socket yet, normal discovery
  retries continue.
- The test and production compose files do not publish the daemon port. The
  manager should be bound to loopback behind a TLS reverse proxy.

## Open release checks

- Docker daemon access was denied in this environment. The compose test stack,
  runtime UID mismatch diagnostic, `read_only` behavior, capability drops,
  and image healthcheck were not exercised against a running container.
- The checked-in compose E2E coverage currently verifies manager health and
  protected UI reachability; role-specific OIDC login, edit/apply, Git
  round-trip, and restart/reconnect journeys still require a working daemon
  fixture and should be completed before calling the 1.0 acceptance gate
  closed.
- govulncheck, gosec, Trivy, and live registry signature/SBOM verification
  could not be run locally. CI is configured to run the scanners and release
  attestation workflow.
- No clean-host Pocket ID deployment rehearsal was possible here.

These are validation gaps, not findings that a control passed. The 1.0
acceptance gate remains open until the container/E2E and clean-host checks are
green.
