# Threat model

## Boundaries and assets

The trust boundaries are browser to manager, manager to RunWisp over the
shared Unix socket, manager to the host filesystem through mounted config and
state volumes, and manager to the OIDC provider and Git remote. Assets include
configuration, session state, OIDC and Git credentials, audit history, and
the daemon's ability to execute configured tasks.

## Threat actors

The design considers unauthenticated network clients, authenticated users with
viewer/operator/editor/admin roles, malicious or malformed identity-provider
claims, a compromised Git remote, and malicious TOML or include paths.

## Security decisions

- OIDC is required by default. Disabled auth is accepted only for loopback
  listeners. Cookie-authenticated unsafe requests require same-origin CSRF
  checks.
- Role checks are applied to manager APIs and classified RunWisp proxy paths;
  unknown proxy operations are denied. The proxy strips browser credentials
  and forwarded identity headers before calling RunWisp.
- The manager runs as the daemon UID, with a read-only image root, a writable
  state volume, `/tmp` tmpfs, all Linux capabilities dropped, and
  `no-new-privileges`. The daemon socket and data volume are shared; no Docker
  socket is mounted.
- Git runs without a shell, with a constrained environment, hooks disabled by
  default, SSH host verification, bounded execution, and secret scanning on
  outbound commits.
- Config writes use managed paths, validation, secure backups, journals,
  atomic replacement, and rollback. Audit records are redacted and
  tamper-evident.
- Editor/admin access implies arbitrary command execution as the daemon's
  user through task configuration. This is an explicit privilege boundary,
  not a sandboxed capability.

## Residual risks

The daemon runs configured commands with its own privileges. A malicious
configuration, compromised Git remote accepted by an operator, or compromised
editor/admin account can therefore affect the host and data accessible to the
daemon UID. Keep that UID unprivileged and limit mounted volumes.

The test stack exercises local manager and daemon integration, but a real
Docker host and a real Pocket ID deployment are still required for final
deployment acceptance. Review the dated security review for checks that could
not run in the current environment.
