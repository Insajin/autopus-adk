---
name: frontend
stack: typescript
framework: frontend
tools: [npm, playwright]
test_framework: vitest
linter: eslint
---

# Frontend Executor Profile

## Idioms

Build accessible, responsive UI components. Extend the TypeScript profile patterns.

### Greenfield Dependency Policy

For new frontend projects, use the SPEC/PRD `## Technology Stack Decision` before choosing React, Next.js, Vite, Tailwind, router, state, test, or component-library versions. Require concrete stable versions, source refs, and checked_at dates; block on missing evidence.

### Design System Docs Preflight

Before creating or changing UI components, inspect the project design sources and provider docs:

```bash
auto design pack --format markdown
auto design docs --format markdown
```

- If Astryx is detected, read `npx astryx template --list --dense`, `npx astryx template <name> --skeleton --dense`, `npx astryx component <Name> --dense`, and `npx astryx docs tokens --dense` before writing Astryx component code.
- If Astryx is not detected, do not add it by default. Reuse the local `DESIGN.md`, token/theme files, and shared UI primitives.
- Do not invent component props, import paths, raw colors, or spacing values when provider docs or local component implementations exist.

### Semantic HTML

```html
<!-- Use semantic elements to convey meaning -->
<nav aria-label="Main navigation">
  <ul>
    <li><a href="/home">Home</a></li>
  </ul>
</nav>

<main>
  <article>
    <h1>Page Title</h1>
    <p>Content...</p>
  </article>
</main>
```

### ARIA Labels

```tsx
// Add aria-label when the visible text is insufficient
<button aria-label="Close modal" onClick={onClose}>
  <XIcon aria-hidden="true" />
</button>

// Use aria-describedby to link form fields to error messages
<input
  id="email"
  aria-describedby="email-error"
  aria-invalid={!!errors.email}
/>
<span id="email-error" role="alert">{errors.email}</span>
```

### data-testid for Test Selectors

```tsx
// Add data-testid to interactive elements for reliable test targeting
<button data-testid="submit-button" type="submit">
  Submit
</button>
```

### Component Composition

```tsx
// Build small, focused components and compose them
function UserCard({ user }: { user: User }) {
  return (
    <article data-testid="user-card">
      <Avatar src={user.avatarUrl} alt={`${user.name}'s avatar`} />
      <UserInfo name={user.name} email={user.email} />
    </article>
  );
}
```

### CSS Modules / Tailwind

```tsx
// CSS Modules: scope styles to the component
import styles from "./Button.module.css";
<button className={styles.primary}>Click</button>

// Tailwind: use utility classes directly
<button className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
  Click
</button>
```

### Responsive Design

```tsx
// Use mobile-first breakpoints
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
  {items.map(item => <Card key={item.id} {...item} />)}
</div>
```

## Testing Patterns

### Vitest for Unit/Component Tests

```typescript
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { UserCard } from "./UserCard";

describe("UserCard", () => {
  it("displays user name", () => {
    render(<UserCard user={{ name: "Alice", email: "a@example.com" }} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });
});
```

### Playwright for E2E

```typescript
import { test, expect } from "@playwright/test";

test("submit button is accessible", async ({ page }) => {
  await page.goto("/form");
  const btn = page.getByTestId("submit-button");
  await expect(btn).toBeVisible();
  await expect(btn).toBeEnabled();
});
```

## Completion Criteria

- [ ] `vitest run` — all component tests pass
- [ ] `playwright test` — all E2E tests pass
- [ ] `tsc --noEmit` — zero type errors
- [ ] `eslint .` — zero warnings
- [ ] Accessibility audit: no critical violations (axe-core or Playwright accessibility checks)
