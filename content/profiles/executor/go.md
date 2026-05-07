---
name: go
stack: go
tools: [go, golangci-lint, gopls]
test_framework: go test
linter: golangci-lint
---

# Go Executor Profile

## Idioms

Write idiomatic Go. Prefer clarity over cleverness.

### Greenfield Dependency Policy

When creating a new `go.mod`, use the SPEC/PRD `## Technology Stack Decision` for the Go toolchain and module dependency versions. Require concrete stable versions, source refs, and checked_at dates; block on missing evidence.

### Error Handling

```go
// Always return errors explicitly; never ignore them
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething: %w", err)
}
```

### Interface-Driven Design

```go
// Define interfaces at the point of use, not at the point of implementation
type Reader interface {
    Read(ctx context.Context, id string) (*Entity, error)
}
```

### Struct Embedding

```go
// Use embedding for behavior composition, not inheritance
type Service struct {
    repo    Repository
    logger  *slog.Logger
}
```

### Context Propagation

```go
// Always pass context as the first parameter
func (s *Service) Get(ctx context.Context, id string) (*Entity, error) {
    return s.repo.Find(ctx, id)
}
```

## Testing Patterns

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name    string
        a, b    int
        want    int
    }{
        {"positive", 1, 2, 3},
        {"zero", 0, 0, 0},
        {"negative", -1, 1, 0},
    }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            got := Add(tc.a, tc.b)
            if got != tc.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
            }
        })
    }
}
```

### Use `t.Parallel()` for independent tests to speed up the suite.

### Subtests for grouped assertions

```go
t.Run("group", func(t *testing.T) {
    // assertions for this group
})
```

## Database Migration Files

WHEN creating or modifying database migration files (SQL migration scripts):

### Naming Convention
- Format: `{6-digit zero-padded number}_{description}.{up,down}.sql`
- Example: `000376_add_retry_tracking.up.sql`, `000376_add_retry_tracking.down.sql`
- NEVER use unpadded numbers (e.g., `376_` instead of `000376_`)

### Number Assignment
1. Scan the migrations directory for existing files: `ls migrations/*.sql 2>/dev/null | sort`
2. Extract the highest existing migration number from filenames
3. Increment by 1 and zero-pad to 6 digits with `fmt.Sprintf("%06d", nextNum)`
4. Verify no existing file shares the same number before creating

### Duplicate Prevention
- Before writing migration files, grep for the target number: `ls migrations/{number}_*.sql 2>/dev/null`
- If a conflict exists, increment until a unique number is found
- Both `.up.sql` and `.down.sql` files MUST use the same number

## Completion Criteria

- [ ] `go test -race ./...` — all tests pass, no data races
- [ ] `go vet ./...` — no issues
- [ ] `golangci-lint run` — no warnings
- [ ] Coverage 85%+
- [ ] `go build ./...` — compiles cleanly
