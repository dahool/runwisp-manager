# Upgrade and compatibility

1. Read the release notes and compatibility matrix before changing image
   versions.
2. Back up the RunWisp config repository and `/var/lib/runwisp-manager` state
   volume. Keep a copy of the current image tag.
3. Update manager and RunWisp together to the matching tested versions. Keep
   `RWM_UID`/`RWM_GID` unchanged and verify the manager still has access to the
   daemon socket and config.
4. Recreate the services and verify `/manager/healthz`, OIDC login, config
   discovery, and Git status before applying edits.

The release CI matrix checks the oldest supported RunWisp release, 1.7.0,
and the `latest` image as a moving compatibility canary. `latest` is not a
reproducible version pin; production deployments should use a tested exact
RunWisp version and the corresponding `-runwisp<VERSION>` manager image tag.
Only the latest manager minor is planned to receive fixes.
