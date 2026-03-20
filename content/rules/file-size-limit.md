---
name: file-size-limit
description: Hard limit of 300 lines per source file
category: structure
---

# File Size Limit

IMPORTANT: No single file may exceed 300 lines. This is a HARD limit.

## Thresholds

- Target: Under 200 lines per file
- Warning: 200-300 lines (consider splitting)
- Hard limit: 300 lines (MUST split before committing)

## Splitting Strategies

- By type: Move struct definitions and methods to separate files
- By concern: Group related functions (validation, serialization)
- By layer: Separate handler, service, and repository logic

## Exclusions

This limit applies to source code files only. The following are excluded:
- Generated files: `*_generated.go`, `*.pb.go`, `*_gen.go`
- Documentation files: `*.md`, `*.txt`, `*.rst`
- Configuration files: `*.yaml`, `*.yml`, `*.json`, `*.toml`
- Lock files: `go.sum`, `package-lock.json`

## Counting

Count ALL lines including comments, blank lines, and imports.
Test files follow the same limit.
