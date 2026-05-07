---
name: typescript
stack: typescript
tools: [tsc, npm, node]
test_framework: vitest
linter: eslint
---

# TypeScript Executor Profile

## Idioms

Write strict TypeScript. Enable `"strict": true` in tsconfig.json.

### Greenfield Dependency Policy

When creating a new `package.json`, do not choose dependency versions from examples or model memory. Use the SPEC/PRD `## Technology Stack Decision` for concrete stable versions with source refs and checked_at dates. If it is missing, block before writing dependencies.

### ESM Imports

```typescript
// Use ESM imports with explicit file extensions for Node.js compatibility
import { fetchUser } from "./user.js";
import type { User } from "./types.js";
```

### Type-Safe Error Handling

```typescript
// Use Result type pattern instead of throwing for recoverable errors
type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E };

function parseUser(raw: unknown): Result<User> {
  if (!isUser(raw)) {
    return { ok: false, error: new Error("invalid user shape") };
  }
  return { ok: true, value: raw };
}
```

### Async/Await Patterns

```typescript
// Always handle promise rejections; avoid floating promises
async function loadData(id: string): Promise<Data> {
  const response = await fetch(`/api/data/${id}`);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.json() as Promise<Data>;
}
```

### Barrel Exports

```typescript
// Group related exports in an index.ts barrel
// src/features/user/index.ts
export { UserService } from "./service.js";
export type { User, CreateUserInput } from "./types.js";
```

### Type Guards

```typescript
function isUser(value: unknown): value is User {
  return (
    typeof value === "object" &&
    value !== null &&
    "id" in value &&
    "name" in value
  );
}
```

## Testing Patterns

Use vitest for all tests.

```typescript
import { describe, it, expect, vi } from "vitest";

describe("UserService", () => {
  it("returns user by id", async () => {
    const repo = { findById: vi.fn().mockResolvedValue({ id: "1", name: "Alice" }) };
    const service = new UserService(repo);
    const user = await service.getUser("1");
    expect(user.name).toBe("Alice");
  });
});
```

## Completion Criteria

- [ ] `tsc --noEmit` — zero type errors
- [ ] `eslint .` — zero warnings
- [ ] `vitest run` — all tests pass
- [ ] No `any` types unless explicitly justified with a comment
