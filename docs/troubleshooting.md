# Troubleshooting

## Manager cannot reach RunWisp

Check that both services mount the same `/var/lib/runwisp` volume and use the
same UID/GID. The daemon socket is normally `/var/lib/runwisp/runwisp.sock`.
Inspect container logs with `docker compose logs runwisp manager`; a socket
permission error usually indicates a UID mismatch or a missing shared volume.
If the manager exits with a socket-owner message, set both compose `user`
values and `RWM_UID` to the UID reported by the RunWisp image, then recreate
the manager image.

## Version mismatch or editing is disabled

The manager image includes the RunWisp binary matching its build version.
Keep `RUNWISP_VERSION` aligned with the daemon image. Check the manager status
banner and `docker compose logs manager` for strict-version or config-path
warnings. Confirm the config is mounted read/write and
`RWM_RUNWISP_CONFIG` points to the actual file.

## OIDC login fails

The issuer URL must exactly match the provider's issuer, including path and
scheme. The callback must be registered as
`<RWM_EXTERNAL_URL>/manager/auth/callback`. Check that the manager can resolve
and reach the provider and that client secret/session secret files are
readable. Verify the provider sends the configured `groups` claim and the
email verification claim expected by policy.

## Environment validation reports missing variables

Mount the same `runwisp.env` into the manager and RunWisp, then set
`RWM_VALIDATE_ENV_FILE` to its manager-side path. Prefer `${VAR}` references
in TOML so tokens remain outside Git.

## Reverse proxy IP or HTTPS behavior is wrong

Set `RWM_TRUSTED_PROXIES` to the reverse proxy's exact address or narrow
subnet. The manager only trusts forwarded headers from those peers. Set
`RWM_EXTERNAL_URL` to the public HTTPS URL and have the proxy forward the
original scheme and host.

## New settings do not take effect

Restart the manager after changing environment variables or secret files.
Restart RunWisp after changing its daemon-level environment. Config fields
marked restart-required cannot be applied through a live reload.
