# ADR-0010: The BuildRun template exists whenever it names a ServiceAccount

Status: accepted. Decided 2026-09-10 (BUILD-2402). Amends ADR-0005.
Enhancement proposal: not covered there.

## Context

A Shipwright Build has no serviceAccount field; the BuildRun has one. The template in the
`buildconfig-to-shipwright/buildrun-template` annotation is therefore the only place the
plugin can say which account a BuildRun must run as. ADR-0005 wrote the template only for
`spec.resources`, and recorded two gaps: a BuildConfig that names an account but sets no
resources got no template, so the name reached no emitted object, and a generated account
was emitted with nothing pointing at it. A BuildRun written by hand then ran as the
namespace `pipeline` account and the pull failed far from its cause.

## Decision

`processResources` writes the template when `spec.resources` has requests or limits, or
when there is a `serviceAccount` to name, generated in step 4 or named by the BuildConfig.
Under a strategy override with resources the template still carries `serviceAccount`, but
not `stepResources`, because the override's step names are unknown; W47 asks for them.
That case predates this record. A BuildConfig with neither gets no template, because it would hold nothing but
the Build's name. `stepResources`, and the two warnings about them, appear only when
`spec.resources` has requests or limits.

## Rules

- One gate, in `processResources`: no resources and no account means no template.
- `stepResources` and the warnings W47 and W48 depend on resources alone. A template that
  carries only an account raises neither.
- The generated account wins when there is one. It is only generated when the BuildConfig
  names none, so the two never compete.
- A template on every Build is still declined (BUILD-2314). The template exists to carry
  something.

## Consequences

- The INFO line that reports the mapped account fires for every named or generated account,
  not only when resources forced a template.
- A Build for a BuildConfig that names an account but sets no resources carries an
  annotation it did not before. The committed examples are unchanged: the one with an
  account also has resources.
- Pinned by `converter_test.go`: `TestServiceAccountTemplateWrittenWithoutResources`,
  `TestGeneratedServiceAccountTemplateWrittenWithoutResources`,
  `TestServiceAccountTemplateCustomStrategyWithoutResources`,
  `TestConvertResourcesEmptyNoAnnotation`.
