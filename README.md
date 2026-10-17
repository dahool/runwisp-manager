# RunWisp Manager

RunWisp Manager is a web administration UI for a RunWisp daemon. It shares
the daemon's configuration/data volumes and Unix socket. OIDC is enabled by
default; the manager should run as the same unprivileged UID as RunWisp behind
a TLS reverse proxy.

## Deploy

Start with the [deployment guide](docs/deployment.md). It covers the matching
UID, OIDC client, secret files, compose volumes, and reverse proxy setup. The
production compose example is [deploy/compose/compose.yaml](deploy/compose/compose.yaml).

An editor or administrator can configure arbitrary commands to run as the
RunWisp daemon user. Grant those roles only to trusted users. See
[SECURITY.md](SECURITY.md) and [the threat model](docs/threat-model.md).

## Develop

Requirements: Go 1.27 and Node.js 22 with npm. Run `make test`, `make lint`,
and `make build`. For browser integration, start the local stack with
`make compose-test-up`, run `make e2e`, then remove it with
`make compose-test-down`.

See [environment variables](docs/ENVIRONMENT.md),
[troubleshooting](docs/troubleshooting.md), and
[release instructions](docs/release.md).
