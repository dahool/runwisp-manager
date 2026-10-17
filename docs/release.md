# Release process

1. Update `CHANGELOG.md`, the compatibility notes, and the version used by
   the deployment example.
2. Require green CI, including Go 1.26/1.27, oldest/latest RunWisp compose
   E2E, dependency/security scans, and image scan. Run the clean-host Pocket
   ID/edit/apply/Git rehearsal before the first 1.0 release.
3. Create and push a semantic version tag such as `v1.0.0`.
4. The release workflow publishes amd64/arm64 images for RunWisp 1.7.0 and
   latest, signs image references with keyless Cosign, and includes SBOM and
   build provenance attestations. It also uploads static Linux amd64/arm64
   archives and checksums to the GitHub Release.
5. Verify the image signature using the expected repository identity and
   inspect its SBOM/provenance before deployment. Smoke-test `/manager/healthz`
   and the login/config/Git flows after release.

The current release workflow runs on tag push and assumes the GitHub Release
has already been created. The production example uses a placeholder release
version; update it to the release being deployed.
