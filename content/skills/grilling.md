---
name: grilling
description: 계획·결정·아이디어를 design tree로 놓고 frontier 질문을 라운드 단위로 집요하게 물어 사용자와 공유 이해에 도달하는 인터뷰 스킬
triggers:
  - grill
  - grilling
  - grill me
  - 그릴
  - 질문으로 검증
  - 빈틈 찾아
  - stress-test
  - 인터뷰해줘
category: methodology
level1_metadata: "design tree, frontier 라운드, 추천 답 동반 질문, 사실은 에이전트가 조사"
---

# Grilling

사용자가 원하는 것을 정확히 알기 전에는 만들지 않는다. 계획·결정·아이디어를 **design tree**로
놓고, 모든 가지가 방문될 때까지 사용자를 집요하게 인터뷰한다. `auto plan`이 SPEC을 쓰기 전,
`auto idea` 뒤, 큰 리팩토링 전에 쓴다. `--auto`(무확인) 워크플로에서는 실행하지 않는다.

## Design tree와 frontier

- **Design tree**: 하나의 결정은 그 아래 매달린 결정들로 갈라진다. 가지는 **결과에 영향을 주는
  결정**만 담는다(Outcome Lock, 계약, 경계, 되돌리기 어려운 선택). 구현 세부와 선택적 개선은
  가지로 만들지 않고 prune한다 — 그래야 tree가 끝난다.
- **Frontier**: 선행 결정이 이미 정해져 *지금* 물어도 추측이 섞이지 않는 질문 전부.
- 한 라운드에 frontier 전체를 **한 메시지**로 묻는다. 결정마다 번호를 붙이고 **추천 답**을
  함께 낸다. 그 메시지에 대한 **한 번의 응답**을 받기 전에는 다음 라운드로 가지 않는다.
- 이번 라운드의 다른 질문에 답이 달려 있는 질문은 *다음* 라운드로 미룬다.

라운드 형식:

```
❓ **Q1** - **<질문 제목>**: <질문 본문. 여러 문단·선택지 가능>

➡️ <추천 답>

---

❓ **Q2** - **<질문 제목>**: <질문 본문>

➡️ <추천 답>
```

답을 받으면 tree를 다시 그린다: 정해진 결정이 frontier를 바깥으로 밀어 그 아래 질문을 연다.
frontier를 다시 계산해 다음 라운드를 묻는다.

## 사실과 결정을 나눈다

- **사실**(파일시스템, 코드, 설정, 도구 출력)은 에이전트의 몫이다. 직접 찾을 수 있는 것을
  사용자에게 묻지 않는다. 범위가 좁으면 grep/read로 직접 조사하고, 넓거나 여러 조사가 독립적이면
  서브에이전트(`explorer`, scout)를 병렬로 보낸다. 조사에 의존하는 질문만 기다리고 나머지
  frontier는 지금 묻는다. 조사한 사실은 산출물에 파일:라인으로 남긴다.
- **결정**은 사용자의 몫이다. 각 결정을 개별 번호로 제시하고, 라운드 단위로 기다린다.
- 프로젝트 용어는 `CONTEXT.md`를 따르고, 없으면 `AGENTS.md`/`CLAUDE.md` →
  `.autopus/project/{product,structure,tech}.md` 순으로 찾는다. 사용자의 표현이 glossary와
  어긋나면 `domain-modeling` 스킬로 그 자리에서 바로잡는다.

## 완료 기준과 마지막 게이트

frontier가 비었을 때 끝난다: design tree의 모든 가지를 방문했고, 말없이 가정한 것이 없다.
그때 **공유 이해 요약**을 보여 주고 `confirm` / `adjust <번호>` / `stop` 중 하나를 받는다.
`confirm` 전에는 실행(코드·SPEC 작성)으로 넘어가지 않는다. `adjust`는 해당 결정을 다시 열어
frontier를 재계산하고, `stop`은 요약을 미결 표시와 함께 남기고 끝낸다.

## Handoff: Grilling Summary

완료 결과는 아래 형식의 `## Grilling Summary`로 `planning`/`prd` 스킬 또는 `auto plan`에 넘긴다.
SPEC 파이프라인에서는 `research.md`에만 보존하고 `spec.md` 본문은 건드리지 않는다.
기존 `Clarification Ledger`/`Question Audit` 계약은 그대로 두고 이 요약을 덧붙인다.

```md
## Grilling Summary
- status: complete | skipped (--auto) | stopped
- frontier: empty | not evaluated | <미결 결정 번호>
- decisions: 번호 · 결정 · 추천과 다르면 그 이유
- rejected: 버린 대안과 이유
- evidence: 조사한 사실(파일:라인)
- question audit: 라운드 수, 질문 수
```

`--auto`에서는 grilling을 실행하지 않으므로 `status: skipped (--auto)`, `frontier: not evaluated`로
적고, 결정 근거는 기존 assumed/deferred Ledger만 권위로 삼는다 — 무확인 진행을 "공유 이해 완료"로
보이게 하지 않는다.

## 안티패턴

- 한 번에 한 질문씩 묻기(라운드가 아니라 핑퐁이 된다)
- 추천 답 없는 질문(사용자에게 백지 사고를 떠넘긴다)
- 조사하면 아는 사실을 질문으로 던지기
- frontier가 남았는데 "대략 이해했다"며 진행하기
- 구현 세부까지 가지로 만들어 tree가 끝나지 않기

Adapted from mattpocock/skills `grilling` (MIT).
