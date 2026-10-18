# RunWisp Manager

RunWisp Manager is a small web application for operating a RunWisp daemon from a browser. It exposes the daemon's configuration and runtime state through a secure UI, manages authentication, supports Git-backed config workflows, and helps an operator validate and edit service configuration without manually editing files on the host.

This repository contains the Go backend and the embedded Svelte frontend used to serve the manager interface.

## Disclaimer

> This project is fully AI generated and reviewed by Human.
> It is intended only for my own personal use.
> It is not intended for public distribution, shared deployment, or general-purpose production use by others.

## What this project does

- Serves a manager UI for a RunWisp daemon over HTTP/HTTPS
- Connects to the daemon by Unix socket or HTTP endpoint
- Discovers the daemon, checks health, and exposes status information
- Allows trusted users to inspect and edit configuration and task definitions
- Supports OIDC-based authentication with role-based access control
- Stores audit records and session state locally
- Enables Git sync workflows for config repositories, including pull/push and validation hooks
- Provides environment validation and checks before startup

## Core features

- Browser-based management UI for task configuration and daemon status
- Secure configuration editing behind authenticated access
- Role separation for viewer, operator, editor, and admin access
- Built-in audit logging for configuration and operational changes
- Git integration for repository-driven configuration workflows
- Trusted proxy handling, TLS configuration, and environment validation
- Docker-friendly deployment pattern for local or self-hosted use

## Architecture overview

- Go service entrypoint: `cmd/runwisp-manager`
- Application logic and auth/configuration code: `internal/`
- Frontend UI: `web/src`
- Docker deployment examples: `deploy/compose`
- Operational docs: `docs/`

The manager runs as a separate service beside RunWisp and is designed to share the same host user context so it can access the daemon socket and config files safely.

## Repository layout

- `cmd/runwisp-manager/` — application entrypoint and CLI commands
- `internal/` — backend packages for config, auth, API, Git, UI, audit, validation, and daemon integration
- `internal/ui/` — embedded web UI serving assets
- `web/` — frontend source files and build system
- `deploy/compose/` — example Docker Compose configuration and secret templates
- `docs/` — deployment, environment, security, troubleshooting, and operational guidance
- `SECURITY.md` — security model and handling expectations

## Requirements

- Go 1.26 or newer
- Node.js 22 with npm (for frontend build and checks)
- A working RunWisp daemon instance
- Optional: Docker for local testing or Compose-based deployment

## Quick start

1. Clone the repository.
2. Install frontend dependencies if you want to build the UI locally:

```bash
cd web
npm install
```

3. Build the binary:

```bash
make build
```

4. Run the manager locally:

```bash
make run
```

This starts the manager with the default configuration loading logic. In practice, you will usually configure environment variables or a config file before running it in a real environment.

## Configuration

The project reads configuration from environment variables, with a generated environment reference in `docs/ENVIRONMENT.md`.

Common variables include:

- `RWM_LISTEN` — manager bind address
- `RWM_EXTERNAL_URL` — public URL for the app
- `RWM_STATE_DIR` — local state directory
- `RWM_RUNWISP_SOCKET` or `RWM_RUNWISP_URL` — daemon connection target
- `RWM_RUNWISP_CONFIG` — configuration path for the daemon
- `RWM_AUTH_MODE` — authentication mode
- `RWM_OIDC_*` — OpenID Connect settings
- `RWM_SESSION_SECRET` — session signing secret
- `RWM_GIT_*` — Git sync configuration

Examples are provided in:

- `deploy/compose/runwisp.env.example`
- `deploy/compose/config/runwisp.toml.example`

## Deployment

A Compose-based deployment example is included in `deploy/compose/compose.yaml`.

Typical setup:

```bash
cd deploy/compose
cp runwisp.env.example runwisp.env
cp config/runwisp.toml.example config/runwisp.toml
cp secrets/oidc_client_secret.example secrets/oidc_client_secret
cp secrets/session_secret.example secrets/session_secret
cp secrets/github_deploy_key.example secrets/github_deploy_key
chmod 600 secrets/*
```

Then update the example values with your real settings and run:

```bash
docker compose up -d
```

See `docs/deployment.md` for the full deployment procedure, including UID matching, reverse proxy setup, OIDC configuration, and security guidance.

## Development and testing

Common project commands:

```bash
make build
make test
make lint
make web
```

For browser-level integration checks:

```bash
make compose-test-up
make e2e
make compose-test-down
```

These commands are defined in the project `Makefile` and are the recommended validation flow for local development.

## Security notes

This project is intended to run on a trusted host and should be placed behind HTTPS with authentication enabled. The manager can modify configuration and execute commands as the RunWisp daemon user for trusted operators; grant those privileges only to administrators you trust.

Key guidance:

- Keep the manager behind a reverse proxy with TLS termination
- Use OIDC or another trusted auth layer
- Do not expose the daemon port publicly
- Do not mount Docker sockets or other privileged host resources into the manager
- Protect session secrets and deploy keys
- Review `SECURITY.md` and `docs/threat-model.md` before deployment

## Project status

This repository is a personal-use project focused on operating a local RunWisp environment with a browser-based administrative UI. It is not a general-purpose SaaS or public application framework.

## License

The project is licensed under GPL-3.0-or-later.

## Related documentation

- `docs/deployment.md`
- `docs/ENVIRONMENT.md`
- `docs/troubleshooting.md`
- `docs/release.md`
- `SECURITY.md`
- `docs/threat-model.md`
