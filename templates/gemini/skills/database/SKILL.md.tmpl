---
name: auto-database
---

# Database Skill

데이터베이스 스키마를 설계하고 안전하게 마이그레이션하는 스킬입니다.

## 스키마 설계 원칙

### 정규화 기본
| 정규형 | 규칙 | 예시 |
|--------|------|------|
| 1NF | 원자값, 반복 그룹 없음 | 전화번호 배열 → 별도 테이블 |
| 2NF | 부분 함수 종속 제거 | 복합 키의 일부에만 의존하는 컬럼 분리 |
| 3NF | 이행 종속 제거 | A→B→C이면 C를 별도 테이블로 |

### 명명 규칙
```sql
-- 테이블: 복수형, snake_case
CREATE TABLE users (...)
CREATE TABLE order_items (...)

-- 컬럼: snake_case, 의미 명확
user_id, created_at, is_active

-- 인덱스: idx_{table}_{columns}
CREATE INDEX idx_users_email ON users(email);

-- 외래 키: fk_{table}_{ref_table}
CONSTRAINT fk_orders_users FOREIGN KEY (user_id) REFERENCES users(id)
```

### 공통 컬럼 패턴
```sql
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    -- 비즈니스 컬럼
    name        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    -- 감사 컬럼
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ  -- soft delete
);
```

## 마이그레이션 관리

### 마이그레이션 파일 구조
```
migrations/
├── 001_create_users.up.sql
├── 001_create_users.down.sql
├── 002_add_user_email_index.up.sql
├── 002_add_user_email_index.down.sql
```

### 안전한 마이그레이션 원칙

**호환되는 변경 (무중단 배포):**
- 컬럼 추가 (NULL 허용 또는 DEFAULT 값)
- 인덱스 추가 (`CONCURRENTLY`)
- 테이블 추가

**비호환 변경 (주의):**
- 컬럼 삭제 → 먼저 코드에서 사용 중단, 다음 배포에서 삭제
- 컬럼 이름 변경 → 새 컬럼 추가 + 데이터 복사 + 구 컬럼 삭제
- 타입 변경 → 새 컬럼으로 마이그레이션

### 대용량 테이블 마이그레이션
```sql
-- 인덱스 생성 (잠금 방지)
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

-- 컬럼 추가 (잠금 최소화)
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
-- NOT NULL 추가는 별도 단계
ALTER TABLE users ALTER COLUMN phone SET DEFAULT '';
UPDATE users SET phone = '' WHERE phone IS NULL;
ALTER TABLE users ALTER COLUMN phone SET NOT NULL;
```

## 쿼리 최적화

### EXPLAIN 분석
```sql
EXPLAIN ANALYZE SELECT * FROM users WHERE email = 'test@example.com';
```

주요 지표:
- **Seq Scan**: 전체 테이블 스캔 (인덱스 필요 시사)
- **Index Scan**: 인덱스 사용 (정상)
- **Rows**: 예상 vs 실제 행 수 차이 → 통계 갱신 필요

### 인덱스 전략
```sql
-- 단일 컬럼 (등호 검색)
CREATE INDEX idx_users_email ON users(email);

-- 복합 인덱스 (순서 중요: 선택도 높은 것 먼저)
CREATE INDEX idx_orders_user_status ON orders(user_id, status);

-- 부분 인덱스 (조건부)
CREATE INDEX idx_users_active ON users(email) WHERE is_active = true;

-- 커버링 인덱스 (쿼리의 모든 컬럼 포함)
CREATE INDEX idx_users_cover ON users(email) INCLUDE (name);
```

### N+1 쿼리 방지
```go
// Bad: N+1
for _, user := range users {
    orders, _ := db.FindOrdersByUserID(user.ID)
}

// Good: JOIN 또는 IN
orders, _ := db.FindOrdersByUserIDs(userIDs)
```

## Go에서 DB 사용

### 권장 라이브러리
| 라이브러리 | 용도 |
|-----------|------|
| `database/sql` | 표준 인터페이스 |
| `sqlx` | 구조체 매핑 확장 |
| `pgx` | PostgreSQL 네이티브 드라이버 |
| `goose` / `migrate` | 마이그레이션 관리 |

## 체크리스트

- [ ] 테이블/컬럼 명명 규칙 준수
- [ ] 마이그레이션 up/down 쌍 작성
- [ ] 대용량 테이블 변경 시 CONCURRENTLY 사용
- [ ] 자주 쓰는 쿼리에 인덱스 설정
- [ ] N+1 쿼리 방지
- [ ] soft delete 패턴 (deleted_at) 적용
