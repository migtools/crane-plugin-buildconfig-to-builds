# ADR-0007: Volumes are converted under their original names and left to fail on the cluster, legibly

Status: accepted. Decided 2026-08-20 (PR #21), reinforced 2026-08-25 (PR #56). Reviewed
2026-09-02, verified on a cluster.
Amended 2026-09-23 by ADR-0014 (BUILD-2265): the shipped strategies declare a second
overridable volume, `trusted-ca`.
Enhancement proposal: not covered there.

## Context

Shipwright matches a Build's volumes to the strategy by exact name and only accepts a volume
the ClusterBuildStrategy declares overridable. The shipped buildah and source-to-image
strategies declare exactly one, for entitlements. Every other converted volume makes the
Build fail registration with `UndefinedVolume`. The alternative, generic slot volumes in the
shipped strategies plus renaming on the plugin side, was rejected because mount paths live
only in the strategy's steps, so a renamed volume would still not be mounted where the build
expects it.

Amended 2026-09-23 (BUILD-2265, [ADR-0014](0014-trusted-ca-generated-volume-fails-visibly.md)):
"exactly one" was true when this was decided and is not any more. strategy-catalog `cb2432c`
added a second overridable volume to both shipped strategies, `trusted-ca`, and the plugin
generates a Build volume for it when a BuildConfig sets `spec.mountTrustedCA`. Nothing else
here moves: a volume the strategy does not declare still fails registration, and a user's own
volume is still copied under its own name rather than renamed into the new slot.

## Decision

Copy Secret and ConfigMap volumes onto the Build under their exact BuildConfig names and let
the cluster reject them. Spend the plugin's effort on making the rejection easy to fix: one
warning per volume with the three edits to make and the original mount path, and one summary
per BuildConfig naming the exact failure, both pointing at `docs/volume-migration.md`.

## Rules

- No renaming, no slot volumes, no silent drops (`processStrategyVolumes`).
- A volume with an unsupported source, a duplicate name, or no name is skipped with a
  warning. It never fails the whole conversion.

## Consequences

- Every converted Build with a volume ships knowing it will fail on the cluster until the
  operator copies the strategy. That is the intended, visible outcome.
- Verified 2026-09-02: refused with `UndefinedVolume`, registered after the strategy copy,
  built. Two details the warning does not say: the mount in the strategy copy must be
  `readOnly: true` or Shipwright refuses the BuildRun, and a secret key needs `subPath` to
  land as a file. The runbook says both; the warning's wording is on the defects list.
- The transform E2E fixtures use made-up volume names and would fail if applied. They test
  conversion only.
