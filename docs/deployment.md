# Docker deployment

The manager and RunWisp must run with the same OS UID so the manager can use
the daemon socket and write configuration files. Discover the image's user
with:

```sh
docker pull runwisp/runwisp:1.7.0
docker run --rm --entrypoint id runwisp/runwisp:1.7.0
```

Use the reported UID and GID for both `user` settings and build args. The
checked-in example defaults to `1000:1000`; change every occurrence if the
RunWisp image reports another identity. Do not run either service as root.

From `deploy/compose`, prepare local configuration and secrets:

```sh
mkdir -p config secrets
cp runwisp.env.example runwisp.env
cp config/runwisp.toml.example config/runwisp.toml
cp secrets/oidc_client_secret.example secrets/oidc_client_secret
cp secrets/session_secret.example secrets/session_secret
cp secrets/github_deploy_key.example secrets/github_deploy_key
chmod 600 secrets/*
openssl rand -hex 32 > secrets/session_secret
```

Edit `runwisp.env`, replace all example secret contents, and configure
`config/runwisp.toml` as needed. Create an OIDC confidential client whose
callback URL is `https://manager.example.com/manager/auth/callback`. Use the
role group names in the compose file or update them to match your IdP. Never
commit the populated secret files or environment file.

Update the manager image tag to the release you intend to run. Set
`RWM_EXTERNAL_URL` to its public HTTPS URL. `RWM_TRUSTED_PROXIES` must list
only the address from which the reverse proxy connects to the manager. The
example assumes a host-level proxy connecting over loopback. For a container
proxy, use a dedicated compose network and its narrow subnet.

```sh
docker compose up -d
docker compose ps
```

Only the manager is published, on `127.0.0.1:9478`; the daemon port is not
published. Terminate TLS at a reverse proxy. For Caddy on the host:

```caddyfile
manager.example.com {
    reverse_proxy 127.0.0.1:9478
}
```

The manager container uses a read-only root filesystem, a state volume,
`/tmp` tmpfs, no Linux capabilities, and `no-new-privileges`. Keep the config,
daemon data, state, and secret mounts backed up. Do not mount
`/var/run/docker.sock` into either service.

For GitHub Git sync, use a dedicated deploy key with read/write rights only to
the configuration repository. Install the matching private key as
`secrets/github_deploy_key`, verify the checked-in known-hosts entry, and set
`RWM_GIT_PUSH_BRANCHES` to the intended branch. See [Git integration](git-core.md).
