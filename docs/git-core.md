# Git core integration

Phase 11 exposes local repository status, masked diffs, bounded history, and
editor commits at `/manager/api/git/{status,diff,log,commit}`. The integration
is optional: `RWM_GIT_ENABLED=auto` hides it when the configuration is not in a
Git work tree, while `false` disables detection explicitly.

Git commands run through the manager's shell-free runner. It allow-lists the
process environment, disables hooks by default, rejects option-like paths and
Git pathspec magic, applies a timeout and output limit, and serializes writes.
Only managed configuration files can be staged. Commits use the OIDC actor as
author, the service identity as committer, and add `Managed-By:
runwisp-manager`; `RWM_GIT_SECRET_GUARD=true` blocks literal secrets and private
key material. An administrator may override the secret or branch guard only
with a written reason, which is included in the audit event.

Relevant settings are documented by `check-config`, including
`RWM_GIT_REPO`, `RWM_GIT_BRANCH`, `RWM_GIT_AUTHOR_*`, `RWM_GIT_DENY_GLOBS`,
`RWM_GIT_ALLOW_HOOKS`, and `RWM_GIT_TIMEOUT`. Fetch, pull, push, and conflict
resolution remain outside this phase.
