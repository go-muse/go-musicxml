# Validation design and requirements

Status: **PROPOSED**. Research snapshot: 8 October 2026.

This directory contains a proposed validation architecture and its MusicXML 4.0
requirements evidence. It is documentation for review, not an implementation,
a new public API, or a claim of complete validation by the current library.

## Reading order

1. [Architecture proposal](architecture.md): one XML parse, optional strict source
   validation, direct model validation, shared rules, and self-contained diagnostics.
2. [Existing-code map](existing-code-map.md): reuse candidates, confirmed gaps,
   model evidence, compatibility, and first implementation candidates.
3. [Sources and coverage](sources-and-coverage.md): the pinned publication, what was
   reviewed, provenance limitations, and the boundary of completeness.
4. [Open questions](open-questions.md): 28 issues and proposed dispositions.
5. [Research registry](registry/README.md): canonical requirements and supporting
   machine-readable evidence.

The catalog contains 2,218 XSD contracts and 342 prose records. These 2,560
records are not 2,560 independent mandatory executable rules. Source inventory
and review cover the scoped 974-page publication and 642 schema annotations;
external normative closure, executable predicates, model mapping, and rule-level
implementation tests remain future work.

The repository-facing narrative is in English, matching the main README. The
research data retains its original bilingual fields and stable identifiers so
that translation does not silently rewrite evidence or decisions.
