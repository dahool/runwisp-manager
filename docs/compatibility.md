# Compatibility matrix

| RunWisp daemon | Manager binary in image | CI coverage |
|---|---|---|
| 1.7.0 | Matching 1.7.0 binary | Required oldest supported version |
| Latest 1.x (`latest` image) | Matching latest image binary | Compose E2E and nightly compatibility canary |

The `latest` entry is intentionally a moving canary. Before production use,
select an exact RunWisp version and its matching manager image tag. The
manager's supported API range is `>=1.7.0 <2.0.0`; CI should be green for the
exact versions chosen for an installation.
