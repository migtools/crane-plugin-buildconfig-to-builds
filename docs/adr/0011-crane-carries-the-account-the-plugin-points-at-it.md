# ADR-0011: crane carries a named ServiceAccount and its RBAC; the plugin only points at it

Status: accepted. Decided 2026-09-10 (BUILD-2402).
Enhancement proposal: not covered there.

## Context

BUILD-2402 asked the plugin to migrate the account a BuildConfig names, with its secrets,
RoleBindings and ClusterRoleBindings, instead of warning that they stay behind. The plugin
sees one resource per process and has no cluster access (ADR-0001), so it cannot read the
account. It does not need to. crane export writes every namespaced object, the account and
its user-created Secrets and RoleBindings included, plus the ClusterRoleBindings,
ClusterRoles and SecurityContextConstraints that name an exported account
(`cmd/export/cluster.go` in crane). crane-lib's KubernetesPlugin drops the token and
dockercfg secrets and strips the subject namespace from RoleBindings; crane-plugin-openshift
strips the `<name>-dockercfg-*` references from every account. This plugin passes all of it
through untouched. Two things do not come across: crane-lib clears every account's `secrets`
list (BUILD-2343), and the `builder`, `deployer` and `default` accounts are dropped under
`strip-default-rbac`, the first two by crane-plugin-openshift and `default` by crane-lib.

## Decision

The plugin emits nothing crane already migrates. For a named account it writes the name into
the BuildRun template (ADR-0010) and warns W9: what crane carries, and what to check on the
target. For `builder`, `deployer` or `default` it warns W67 instead. A flag that would let the plugin
read the export directory to synthesize or patch the account was declined: one owner per
object, and the single-resource contract stays.

## Rules

- Never emit a ServiceAccount, Secret, RoleBinding or ClusterRoleBinding for a named
  account. Rule 7 (ADR-0006) already forbids a same-named account; this extends it to
  everything crane carries for it.
- W9 names what crane carries and the three checks: the account was in the export,
  `--for=mount` links are re-linked, the `_cluster` resources were applied. It stays a
  warning until BUILD-2343 lands, because the secrets-list loss is real and the plugin
  cannot see whether it applies.
- W8, the pull-secret link command, no longer claims crane migrates the account, so it
  and W67 can share a Build without contradicting each other. Its command tail is
  unchanged; quoting the names in it is BUILD-2439.

## Consequences

- A named account keeps the outcome at `converted-with-warnings`.
- A label-selected export can leave the account out; the plugin cannot tell, which is why
  W9 says to check.
- The SCC grant a generated account needs is still a documented manual step (README,
  ADR-0006). Emitting a RoleBinding for it is a separate decision, proposed as its own
  story.
- Pinned by `converter_test.go`: `TestServiceAccountAssociationWarned`,
  `TestServiceAccountBuilderAndDeployerWarnedSeparately`,
  `TestNamedServiceAccountWithPullSecretIsNotGenerated`.
