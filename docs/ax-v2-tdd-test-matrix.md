# ax v2 CLI 고도화 TDD 구현 계획 (2/3) — Test Matrix

- plan_id: `ax-v2-tdd-test-matrix-v1`
- status: `implementation-reviewed-complete`
- source: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- test_command_red: `go test -v ./...`
- test_command_green: `go test -v ./...`
- format_command: `gofmt -w $(git ls-files '*.go')`
- integration_test_command: `go test -v -tags=integration ./...`

---

## Overview
요구사항을 수용 테스트 단위(AT-01~AT-26)로 고정하고, Phase/Tier별로 실행 가능한 테스트 순서를 정의한다.

---

## Requirements Digest (테스트 관점)
- 상태/전이/컨텍스트: 결정성과 복구성 보장
- run/quick/TDD 루프: 강제 순서 + 사용자 제어 + 재개
- verify/archive: 판정 근거 + 감사 메타데이터
- engine 연동: 세션 lifecycle + streaming + approval
- 운영성: observability 필드 + 에러 분류 코드
- 내장 스킬/인터뷰/party 모드: 계약 기반 검증

---

## Acceptance Test Matrix (필수 26개)

| ID | 요구사항 | Tier | Layer | Gap Ref |
|---|---|---|---|---|
| AT-01 | Red→Green→Refactor 강제 | T2 | Integration | §11-1 |
| AT-02 | Tier progression gate(T0→T1→T2) | T2 | Unit/Integration | §11-2, §14.1 |
| AT-03 | TDD step↔Turn 1:1 | T2 | Contract/Integration | §11-3 |
| AT-04 | reject/steer 왕복 + approval policy 분기 | T2 | Integration | §11-4, §10.7 |
| AT-05 | review 8-lens 완전성 | T4 | Integration | §11-5, §10.5 |
| AT-06 | verify 판정 결정성 | T2 | Unit/Integration | §11-6 |
| AT-07 | `plan --from` 실패 경로 | T1 | Contract | §11-7 |
| AT-08 | quick 임계치 승격 | T2 | Integration | §11-8 |
| AT-09 | archive strict mode | T3 | Integration | §11-9 |
| AT-10 | run crash 후 resume | T2 | Resilience | §11-10 |
| AT-11 | app-server 단절 복구 + lifecycle 재개 | T3 | Resilience | §11-11, §10.6 |
| AT-12 | archive metadata schema/idempotency | T3 | Unit/Integration | §11-12 |
| AT-13 | state.yaml round-trip + 전이 메타필드 파싱 | T1 | Unit | §11-13, §10.1 |
| AT-14 | observability 필수 필드(step/phase/thread_id/turn_id/error_code) | T3 | Integration | §11-14, §12 |
| AT-15 | git worktree 생성/머지/정리 | T3 | Integration | §16-15, §14.2 |
| AT-16 | adaptive depth 분류 정확성 | T2 | Unit | §16-16, §14.3 |
| AT-17 | reject→재실행→accept 사이클 | T2 | Integration | §16-17 |
| AT-18 | steer 후 Turn 방향 변경 | T2 | Integration | §16-18 |
| AT-19 | quality gate 횟수 초과 차단 | T2 | Unit/Integration | §16-19, §14.6 |
| AT-20 | party mode 3-persona 완전성 | T1 | Contract/Integration | §16-20, §14.4 |
| AT-21 | context 영향경로 + 3-Layer 선택 일관성 | T2 | Unit/Integration | §16-21, §10.3, §14.7 |
| AT-22 | tdd-go diff/설명/리뷰 자동생성(+기본 엔진 프로파일 검증) | T2 | Integration | §16-22, §14.8 |
| AT-23 | tdd-go-loop tier 풀사이클 | T3 | E2E | §16-23 |
| AT-24 | critical 이슈 시 tier 차단 | T2 | Integration | §16-24 |
| AT-25 | interview 질문→답변→반영 E2E | T1 | E2E | §16-25, §14.8 |
| AT-26 | 내장 스킬 6개 로딩/registry 정합성 | T1 | Contract/Integration | §16-26, §14.8 |

---

## Supplemental Requirement Trace (RT, 비-AT 누락 보정)

| ID | 요구사항(보강) | Tier | Gap Ref |
|---|---|---|---|
| RT-01 | `propose` 필수 섹션 + `tasks>=10` + `current.proposal` 연동 검증 | T1 | §4.1 |
| RT-02 | `plan` 필수 섹션 + tasks-step ID 매핑 검증 | T1 | §4.2 |
| RT-03 | `run` 입력 plan 검증 + 시작/종료/실패 로그 아티팩트 검증 | T2 | §4.3 |
| RT-04 | `verify` 미충족 목록/다음 액션 생성 + 이전 verify diff 경로 검증 | T2 | §4.4 |
| RT-05 | `discover` 필수 섹션 + discovery phase 기록 검증 | T1 | §4.6 |
| RT-06 | `quick` 종료 후 state 복원/정리 + 실패 시 standard flow 승격 안내 검증 | T2 | §4.7 |
| RT-07 | `state` 출력(phase/current refs/last result/error/progress) + human/machine 포맷 검증 | T1 | §4.8 |
| RT-08 | app-server stdio JSON-RPC + 기본 3메서드 + timeout/error 매핑 + mock 단위테스트 검증 | T3 | §4.9 |

---

## Phase/Tier Checklist (실행 배치)

### Batch A — Foundation (T1)
- [x] AT-07, AT-13, AT-20, AT-25, AT-26
- [x] phase: state/plan/discover 계약 안정화
- [x] RT-01, RT-02, RT-05, RT-07

### Batch B — Core Execution (T2)
- [x] AT-01, 02, 03, 04, 06, 08, 10, 16, 17, 18, 19, 21, 22, 24
- [x] phase: run/tdd/depth/reject-steer 전체 구현
- [x] phase: quality gate(AT-19 범위) 구현/테스트 추가
- [x] RT-03, RT-04, RT-06

### Batch C — Integration & Ops (T3)
- [x] AT-09, 11, 12, 14, 15, 23
- [x] phase: archive/worktree/streaming/recovery 전체 구현
- [x] phase: archive strict + app-server lifecycle/streaming 기본 경로 검증
- [x] RT-08
- [x] phase supplemental: compaction 정책 검증(비-AT 추적 항목)
- [x] phase supplemental: compound triage/Osmani filter/gotcha schema 검증(비-AT 추적 항목)

### Batch D — Delivery Review (T4)
- [x] AT-05
- [x] phase: 8-lens 결과 스키마 + severity 검증

---

## Tier Checklist Review (문서 반영 완료)
- [x] T1 배치(AT-07/13/20/25/26 + RT-01/02/05/07)와 gap §4/§14 요구사항 매핑 확인
- [x] T2 배치(AT-01/02/03/04/06/08/10/16/17/18/19/21/22/24 + RT-03/04/06)와 run/tdd/quality gate 매핑 확인
- [x] T3 배치(AT-09/11/12/14/15/23 + RT-08)와 archive/worktree/streaming/recovery/engine 계약 매핑 확인
- [x] T4 배치(AT-05)와 8-lens 최종 심사 매핑 확인
- [x] `docs/ax-v2-cli-requirements-gap.md` 정합성 반영 준비 포인트(§4/§10/§11/§12/§14/§16) 기록 완료

---

## Tier Item Count
| Tier | Test Count |
|---|---:|
| T1 | 5 AT + 4 RT |
| T2 | 14 AT + 3 RT |
| T3 | 6 AT + 1 RT |
| T4 | 1 |
| **Total** | **26 AT + 8 RT** |

---

## Policy Lock (2026-02-26)
| 항목 | 고정값 | 테스트 반영 포인트 |
|---|---|---|
| archive strict 기본값 | enabled(차단) | AT-09 + unverified 예외 플래그 경로 검증 |
| quick 승격 임계치 | `files>5` 또는 `cross-module` 또는 `core(state/codex/run)` 변경 | AT-08 |
| app-server timeout/retry | 30s / 2회 / backoff(1s,2s+jitter) | AT-11 |
| quality gate | 기본 3회, `--force`+사유 시 5회 | AT-19 |
| depth 우선순위 | `--depth` 절대 우선, 미지정 시 auto | AT-16 |

---

## Completion Log
| Date | Batch | Result | Evidence |
|---|---|---|---|
| 2026-02-26 | A-D | Draft Created | this doc |
| 2026-02-26 | A-D | Tier Checklist Reviewed & Updated | Tier Checklist Review section |
| 2026-02-26 | A-C | Revalidation + partial evidence injected | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, archive allow-unverified/idempotent 회귀 + proposal path 입력 회귀 테스트 추가 |
| 2026-02-26 | B-C | Worker-3 revalidation | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, tier gate/approval/verify-diff/compound/compaction/worktree deterministic 테스트 보강 |
| 2026-02-26 | B-C | Worker-2 final revalidation | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, verify diff/approval/compound/compaction/worktree no-side-effect 회귀 검증 |
| 2026-02-26 | B-C | Worker-1 final verification | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, tier gate/approval policy/verify diff/compound triage/compaction/worktree deterministic/no-side-effect 재확인 |

---

## Requirements Alignment Readiness (Gap Sync Prep)
- 기준 문서: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- 상태: **Complete (remaining gap-linked AT/RT revalidated)**
- 준비 체크리스트:
  - [x] AT-01~AT-26 ↔ T1~T4 배치 매핑 정합성 점검 완료
  - [x] Policy Lock 5항목이 대응 AT(08/09/11/16/19)에 연결됨
  - [x] 3/3 실행 문서의 Stage 게이트와 배치 기준 일치 확인
  - [x] 구현 실행 증거(실테스트 로그/리포트) 부분 주입

---

## Gap 1:1 Alignment (Revalidated)

### §11 ↔ AT-01~AT-14
- 수용 테스트 기본 14개는 AT-01~AT-14로 1:1 대응.

### §16 ↔ AT-15~AT-26
- 추가 12개 테스트는 AT-15~AT-26으로 1:1 대응.

### §10/§14 Must 보강 포인트
- §10.1 전이 메타필드: AT-13에 명시.
- §10.3 3-Layer Context: AT-21에 명시.
- §10.6 lifecycle 복구성: AT-11에 명시.
- §10.7 approval policy: AT-04에 명시.
- §14.8 tdd-go 기본 프로파일/스킬체인: AT-22/26에 명시.
- §10.4 compound triage/Osmani/gotcha schema는 Batch C supplemental(비-AT)로 추적.

### §4 Must 1:1 보강 매핑
- §4.1 propose: RT-01
- §4.2 plan: RT-02
- §4.3 run: RT-03
- §4.4 verify: RT-04
- §4.6 discover: RT-05
- §4.7 quick: RT-06
- §4.8 state: RT-07
- §4.9 app-server 기본 계약: RT-08

## 결론
- Test Matrix는 gap §11/§16의 26개 수용 테스트를 완전 매핑했다.
- Tier별 누락 항목은 RT-01~08로 보강해 gap §4 Must까지 1:1 추적 가능하게 만들었다.
- 남은 추적 항목은 T1/T3의 별도 제품 범위(인터뷰/스킬/streaming/worktree merge-cleanup)이며, 본 gap-closing 범위(T2/T3 핵심 코드 갭)는 완료되었다.

---

## Notes
### 실행 규칙
- 테스트 이름에 AT-ID를 포함해 추적성 보장
- 실패 시 `error_code`를 표준화하여 기록
- 동일 테스트 3회 반복 시 결과 일치해야 통과(결정성)
- depth 결정 로그에 `depth_source=user|auto`를 반드시 기록
- archive 우회 경로는 `--allow-unverified-archive` 명시 시에만 허용

### Out of Scope
- GUI 기반 테스트 자동화
- 외부 SaaS 의존 E2E
