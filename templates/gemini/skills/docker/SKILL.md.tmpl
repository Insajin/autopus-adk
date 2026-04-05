---
name: auto-docker
---

# Docker Skill

효율적이고 안전한 컨테이너 이미지를 빌드하는 스킬입니다.

## Go 멀티스테이지 Dockerfile

```dockerfile
# Stage 1: 빌드
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/app ./cmd/...

# Stage 2: 실행
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' appuser

COPY --from=builder /bin/app /bin/app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/app"]
```

## Docker Compose

```yaml
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/app
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: app
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U user"]
      interval: 5s
      retries: 5

volumes:
  pgdata:
```

## 이미지 최적화

### 레이어 캐싱 순서
```dockerfile
# 변경 빈도 낮은 것부터 (캐시 활용 극대화)
COPY go.mod go.sum ./     # 1. 의존성 정의 (거의 안 바뀜)
RUN go mod download       # 2. 의존성 다운로드
COPY . .                  # 3. 소스 코드 (자주 바뀜)
RUN go build ...          # 4. 빌드
```

### 이미지 크기 줄이기
| 베이스 이미지 | 크기 | 용도 |
|-------------|------|------|
| `scratch` | 0MB | 정적 바이너리 (CGO 없이) |
| `alpine` | ~5MB | CA 인증서, 쉘 필요 시 |
| `distroless` | ~2MB | 보안 중시 프로덕션 |
| `golang` | ~800MB | 빌드 스테이지만 |

### .dockerignore
```
.git
*.md
*.out
coverage*
vendor/
```

## 보안 원칙

- **root 실행 금지**: `USER appuser` 필수
- **시크릿 빌드 인자 금지**: `ARG`로 비밀번호 전달하지 않기
- **최소 베이스 이미지**: alpine 또는 distroless
- **이미지 스캐닝**: `docker scout`, `trivy`

## 체크리스트

- [ ] 멀티스테이지 빌드 사용
- [ ] 비-root 유저로 실행
- [ ] .dockerignore 설정
- [ ] 레이어 캐싱 순서 최적화
- [ ] 헬스체크 설정 (Compose)
- [ ] 시크릿 하드코딩 없음
