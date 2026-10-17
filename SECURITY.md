# Security policy

RunWisp Manager is intended to run on a trusted host alongside the RunWisp
daemon. Keep the manager behind HTTPS and an authenticated reverse proxy or
OIDC provider. Do not publish the RunWisp daemon port, mount the Docker socket,
or expose the manager directly to the public internet.

An editor or administrator can change a task's `run` command and therefore
execute arbitrary commands as the RunWisp daemon's operating-system user.
Grant those roles only to trusted operators. A viewer can read configuration
and operational metadata; secrets are masked in manager responses and Git
diffs.

Report suspected vulnerabilities privately through the repository owner's
GitHub Security Advisories. Include the affected version, a reproduction, and
the impact. Do not include live credentials or customer configuration in an
issue.

Supported releases receive security fixes on the latest minor release. Images
are intended to be signed with keyless Cosign and published with SBOM and
build-provenance attestations; verify those attestations before deployment.
