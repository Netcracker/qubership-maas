---
description: OpenSpec — create proposal, delta specs, design, and tasks
---

You are running OpenSpec `/opsx:propose` in qubership-maas.

Read `openspec/config.yaml`, `AGENTS.md` (OpenSpec section), and existing `openspec/changes/`. Create or update one
change folder:

- `proposal.md` — why / what / non-goals
- `design.md` — decisions and rejected alternatives
- `specs/<capability>/spec.md` — ADDED / MODIFIED / REMOVED with SHALL and GIVEN/WHEN/THEN
- `tasks.md` — checkbox implementation list
- `.openspec.yaml` — `schema: spec-driven`

Do not implement code. Larger changes go to a separate SPEC PR before `/opsx-apply`. Link existing docs/ architecture
instead of pasting it into spec.md.
