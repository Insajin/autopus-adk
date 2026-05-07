---
name: rust
stack: rust
tools: [cargo, rustc, clippy]
test_framework: cargo test
linter: clippy
---

# Rust Executor Profile

## Idioms

Write safe, idiomatic Rust. Avoid `unsafe` unless there is no alternative.

### Greenfield Dependency Policy

When creating a new `Cargo.toml`, use the SPEC/PRD `## Technology Stack Decision` for Rust edition/toolchain and crate versions. Require concrete stable versions, source refs, and checked_at dates; block on missing evidence.

### Ownership and Borrowing

```rust
// Prefer borrowing over cloning when ownership is not needed
fn print_name(name: &str) {
    println!("{}", name);
}

// Use owned types when the function needs to store the value
struct Config {
    name: String,
}
```

### Result and Option

```rust
use std::num::ParseIntError;

// Chain Results with `?` operator
fn parse_and_double(s: &str) -> Result<i32, ParseIntError> {
    let n: i32 = s.trim().parse()?;
    Ok(n * 2)
}

// Use Option combinators
fn first_even(nums: &[i32]) -> Option<i32> {
    nums.iter().find(|&&n| n % 2 == 0).copied()
}
```

### Traits

```rust
// Define shared behavior through traits
trait Summarize {
    fn summary(&self) -> String;
}

impl Summarize for Article {
    fn summary(&self) -> String {
        format!("{}: {}", self.author, &self.title[..50.min(self.title.len())])
    }
}
```

### Derive Macros

```rust
// Use derive macros for common trait implementations
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
struct UserId(String);
```

### Unsafe Avoidance

- Prefer safe abstractions from `std` and well-audited crates.
- Document all `unsafe` blocks with a `// SAFETY:` comment explaining the invariants.

## Testing Patterns

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_and_double() {
        assert_eq!(parse_and_double("21").unwrap(), 42);
    }

    #[test]
    fn test_parse_error_propagates() {
        assert!(parse_and_double("not-a-number").is_err());
    }
}
```

## Completion Criteria

- [ ] `cargo test` — all tests pass
- [ ] `cargo clippy -- -D warnings` — zero warnings
- [ ] `cargo fmt --check` — code is formatted
- [ ] `cargo build` — compiles without errors
- [ ] No `unsafe` blocks without `// SAFETY:` justification
