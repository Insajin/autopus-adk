---
name: python
stack: python
tools: [python, pip, uv]
test_framework: pytest
linter: ruff
---

# Python Executor Profile

## Idioms

Write modern Python (3.11+). Use type hints everywhere. Prefer `uv` for dependency management.

### Greenfield Dependency Policy

When creating a new `pyproject.toml` or `requirements.txt`, use the SPEC/PRD `## Technology Stack Decision` for the Python runtime and dependency versions. Require concrete stable versions, source refs, and checked_at dates; block on missing evidence.

### Type Hints

```python
from typing import Optional

def find_user(user_id: str) -> Optional["User"]:
    """Return user by ID, or None if not found."""
    ...
```

### Dataclasses

```python
from dataclasses import dataclass, field

@dataclass
class UserConfig:
    name: str
    tags: list[str] = field(default_factory=list)
    active: bool = True
```

### Async Patterns

```python
import asyncio

async def fetch_all(ids: list[str]) -> list[dict]:
    tasks = [fetch_one(i) for i in ids]
    return await asyncio.gather(*tasks)
```

### F-strings

```python
# Use f-strings for string interpolation
message = f"User {user.name!r} created at {user.created_at:%Y-%m-%d}"
```

### Virtualenv Setup

```bash
# Use uv for fast, reproducible environments
uv venv
source .venv/bin/activate
uv pip install -r requirements.txt
```

## Testing Patterns

Use pytest with fixtures.

```python
import pytest
from myapp.service import UserService

@pytest.fixture
def service(mock_repo):
    return UserService(repo=mock_repo)

@pytest.fixture
def mock_repo(mocker):
    return mocker.MagicMock()

def test_get_user_returns_none_when_missing(service, mock_repo):
    mock_repo.find.return_value = None
    result = service.get_user("unknown")
    assert result is None
```

### Parametrize for table-driven tests

```python
@pytest.mark.parametrize("a, b, expected", [
    (1, 2, 3),
    (0, 0, 0),
    (-1, 1, 0),
])
def test_add(a: int, b: int, expected: int) -> None:
    assert add(a, b) == expected
```

## Completion Criteria

- [ ] `pytest` — all tests pass
- [ ] `mypy .` — zero type errors
- [ ] `ruff check .` — zero linting issues
- [ ] `ruff format --check .` — code is formatted
- [ ] Coverage 85%+
