# PRD: Context engineering evolution

## Problem

The completed context-engineering SPEC aligns generated guidance, but three
runtime gaps remain:

- no measurable `full` versus JIT candidate exists before a delivery default
  can be reconsidered;
- the canonical five-field worker return has duplicate types but no
  provider-neutral parser or persisted pipeline consumer;
- Claude and Gemini can bypass the opt-in skill compiler, while Gemini drops
  source agent tool restrictions.

## Goal

1. Emit a body-free `autopus.context_plan.v2` shadow sidecar while active
   delivery remains complete `autopus.context_delivery.v1`.
2. Parse an explicitly marked, bounded worker receipt and persist it in the
   pipeline without breaking markerless legacy output.
3. Make Claude and Gemini honor the existing opt-in compiler and project known
   Gemini tool capabilities into native subagent frontmatter.
4. Add at most three compact failure-derived examples at high-risk boundaries.

## Non-Goals

- active JIT delivery or required-body shrink;
- default skill/shared-surface reduction;
- provider-native history mutation or tool-result deletion;
- orchestra debate output parsing;
- claiming guidance-only capabilities are natively enforced;
- new retrieval infrastructure or dependencies;
- direct edits to generated root platform surfaces.

## Success Metrics

| Metric | Target |
|---|---|
| active mode | `full` in every plan |
| plan raw body/query leakage | 0 |
| unlabeled hit rate | JSON `null` |
| valid marked receipt projection | exactly once |
| malformed marked receipt accepted | 0 |
| markerless pipeline compatibility | unchanged |
| Claude/Gemini split long-tail leakage | 0 |
| unsupported Gemini tool projection | 0 |

## Constraints

- Source of truth is nested `autopus-adk`.
- Existing dirty WIP and compatibility defaults must be preserved.
- New source files remain below 300 lines.
- Promotion from shadow requires later paired quality evidence and explicit
  approval.
