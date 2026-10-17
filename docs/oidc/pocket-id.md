# Pocket ID OIDC setup

Phase 03 targets Pocket ID as the chosen provider. Configure a confidential
client with callback URL `<RWM_EXTERNAL_URL>/manager/auth/callback`, enable
PKCE, and grant the client access to the four role groups. The manager maps
`rwm-admin`, `rwm-editor`, `rwm-operator`, and `rwm-viewer` through the matching
`RWM_OIDC_ROLE_*_GROUPS` variables.

## Verification status

The Phase 03 worker could not perform the live Pocket ID spike in this build
environment: no Pocket ID endpoint or credentials were supplied. Therefore the
real ID-token claim shape, `email_verified` behavior, and
`end_session_endpoint` support remain to be confirmed during deployment.

Expected deployment settings:

```text
RWM_OIDC_SCOPES=openid profile email groups
RWM_OIDC_CLAIM_GROUPS=groups
RWM_OIDC_REQUIRE_EMAIL_VERIFIED=false  # only if the provider omits the claim
```

The manager verifies issuer, audience, signature, expiry, and nonce, and keeps
the provider's ID-token hint encrypted in the SQLite session store.
