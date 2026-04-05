# auto-api-design

# API Design Skill

확장 가능하고 일관된 API를 설계하는 스킬입니다.

## RESTful API 설계 원칙

### URL 설계
```
GET    /api/v1/users          # 목록 조회
GET    /api/v1/users/:id      # 단건 조회
POST   /api/v1/users          # 생성
PUT    /api/v1/users/:id      # 전체 수정
PATCH  /api/v1/users/:id      # 부분 수정
DELETE /api/v1/users/:id      # 삭제
```

규칙:
- 복수형 명사 사용 (`users`, `orders`)
- 동사 금지 (`/getUsers` → `/users`)
- 계층 관계는 중첩 (`/users/:id/orders`)
- 최대 2단계 중첩 (그 이상은 쿼리 파라미터)

### HTTP 상태 코드
| 코드 | 의미 | 사용 시점 |
|------|------|----------|
| 200 | OK | 성공 (GET, PUT, PATCH) |
| 201 | Created | 리소스 생성 (POST) |
| 204 | No Content | 삭제 성공 (DELETE) |
| 400 | Bad Request | 요청 데이터 오류 |
| 401 | Unauthorized | 인증 필요 |
| 403 | Forbidden | 권한 없음 |
| 404 | Not Found | 리소스 없음 |
| 409 | Conflict | 상태 충돌 |
| 422 | Unprocessable | 유효성 검증 실패 |
| 500 | Internal Error | 서버 에러 |

### 에러 응답 형식
```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "입력값이 유효하지 않습니다",
    "details": [
      {"field": "email", "reason": "이메일 형식이 아닙니다"}
    ]
  }
}
```

### 페이지네이션
```
GET /api/v1/users?page=2&per_page=20
```

응답 헤더:
```
X-Total-Count: 150
Link: <...?page=3>; rel="next", <...?page=1>; rel="prev"
```

## gRPC 설계

### Proto 파일 구조
```protobuf
syntax = "proto3";
package api.v1;

service UserService {
  rpc GetUser(GetUserRequest) returns (User);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc CreateUser(CreateUserRequest) returns (User);
}

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}
```

### 사용 시점
- 마이크로서비스 간 내부 통신
- 높은 처리량, 낮은 지연시간 필요 시
- 양방향 스트리밍 필요 시

## API 버전 관리

### URL 버전 (권장)
```
/api/v1/users
/api/v2/users
```

### 호환성 규칙
- 필드 추가: 호환 (기존 클라이언트 영향 없음)
- 필드 제거: 비호환 (새 버전 필요)
- 필드 타입 변경: 비호환 (새 버전 필요)
- 필수 → 선택: 호환
- 선택 → 필수: 비호환

## 설계 체크리스트

- [ ] 일관된 URL 패턴
- [ ] 적절한 HTTP 메서드/상태 코드
- [ ] 표준화된 에러 응답
- [ ] 페이지네이션 지원
- [ ] 버전 관리 전략 결정
- [ ] 인증/인가 설계
- [ ] Rate limiting 고려
