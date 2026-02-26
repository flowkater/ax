# ax v2 CLI 고도화 TDD 구현 계획 (3/3) — Execution & Final Review

- plan_id: `ax-v2-tdd-exec-review-v1`
- status: `implementation-reviewed-complete`
- source: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- execution_mode: `phase-gated`
- reviewer_mode: `8-lens + go/no-go`

---

## Overview
Master Plan(1/3)과 Test Matrix(2/3)를 실제 구현 순서로 실행하기 위한 운영 절차 및 최종 승인 기준을 정의한다.

---

## Requirements Digest (운영/검증 관점)
- 단계별 게이트 없이 다음 단계 진입 금지
- 결과 주장 전 증거(테스트 로그, 빌드 로그, 산출물 경로) 필수
- Critical 이슈 1건 이상이면 No-Go
- 상태/로그/archive는 감사 가능해야 함
- gap §4/§10/§11/§12/§14/§16 요구사항을 Stage 게이트에 1:1 매핑

---

## Sequential Execution Checklist

### Stage 1 — P0/P1 (Spec & State)
- [x] 상태머신/스키마/context_chain 구현
- [x] 전이 메타필드(`transitioned_at`, `trigger_command`) + 3-Layer Context 로딩 규칙 반영
- [x] propose/plan/discover/party/interview + builtin-skill-registry 필수 섹션/계약 보장
- [x] Batch A(T1) 테스트 통과
- [x] RT-01/02/05/07 통과(§4.1/4.2/4.6/4.8 누락 보정)

### Stage 2 — P2 (Run/TDD Core)
- [x] `run --tdd` + T0→T1→T2 tier gate + reject/steer
- [x] quick 승격 + run resume + quality gate
- [x] approval policy 분기 + tdd-go 기본 엔진 프로파일 검증
- [x] Batch B(T2) 테스트 통과
- [x] RT-03/04/06 통과(§4.3/4.4/4.7 누락 보정)

### Stage 3 — P3/P4 (Ops & Engine)
- [x] verify/archive/review/compound 완성
- [x] app-server 기본 3메서드(CreateThread/RunTurn/GetThread) + stdio JSON-RPC client 검증
- [x] app-server lifecycle(Resume/Fork/Rollback/Steer/Interrupt) + streaming + approval + compaction
- [x] compound triage(FixCandidate/Document/Noise) + Osmani filter + gotcha schema/decay/audit 검증
- [x] Batch C(T3), Batch D(T4) 테스트 통과
- [x] RT-08 통과(§4.9 누락 보정)
- [x] runtime hardening 최종 통합(cluster/node/session metadata, shared/worktree session isolation, runtime journal/checkpoint + recover resume) + 실부하 검증 통과

### Stage 4 — Final E2E
- [x] propose→plan→run→verify→archive 1회 성공
- [x] `go test ./...` + `go build ./...` 통과
- [x] `go test -race ./...` + `go vet ./...` 통과
- [x] 실제 바이너리 runtime-mode(shared/worktree) + recover/doctor smoke 통과
- [x] 릴리즈 후보 산출물 고정
- [x] 1/3~3/3 문서 Gap 1:1 alignment 결과 첨부

---

## 8-Lens Final Review Checklist
1. Correctness
2. Reliability / Recovery
3. Test Adequacy
4. Observability
5. Security / Safety
6. Maintainability
7. Performance
8. UX / CLI Clarity

판정 규칙:
- Critical: 0건 필수
- Major: 0건 필수
- Minor: 다음 스프린트 이월 가능

---

## Go / No-Go Criteria

### Go
- AT-01~AT-26 통과
- `go test ./...` + `go build ./...` 통과
- 8-lens Critical/Major 0건
- archive metadata 완전성 검증 완료
- compaction 정책 트리거/로그 검증 완료

### No-Go
- verify 근거 누락
- illegal state transition 허용
- step↔turn 매핑 불일치
- app-server 단절 복구 실패

---

## Tier Checklist Review (문서 반영 완료)
- [x] T1 게이트(Stage 1 / Batch A + RT-01/02/05/07)와 gap §4/§14 매핑 재검토
- [x] T2 게이트(Stage 2 / Batch B + RT-03/04/06)와 run/tdd/reject-steer/quality gate/verify 후속액션 매핑 재검토
- [x] T3 게이트(Stage 3 / Batch C + RT-08)와 archive/worktree/streaming/recovery/compaction/app-server 기본 계약 매핑 재검토
- [x] T4 게이트(Stage 3 / Batch D)와 8-lens 심사 기준 매핑 재검토
- [x] `docs/ax-v2-cli-requirements-gap.md` 반영 준비 섹션(§4/§10/§11/§12/§14/§16) 정렬 완료

---

## Decision Lock (승인 반영 완료: 2026-02-26)
1. archive strict 기본정책: **enabled(차단)**  
   - 예외는 `--allow-unverified-archive` 플래그로만 허용
2. quick 승격 임계치: **하이브리드 규칙**  
   - 변경 파일 수 `>5` 또는 cross-module 변경 또는 core 경로(`state/codex/run`) 변경
3. timeout/retry 기본값: **30초 / 2회 / backoff(1s,2s+jitter)**
4. quality gate 제한: **기본 3회**, `--force` + 사유 기록 시 5회까지 허용
5. depth override 정책: **`--depth` 절대 우선**, 미지정 시 자동 분석

---

## Completion Log
| Date | Stage | Review Result | Notes |
|---|---|---|---|
| 2026-02-26 | Planning | Conditional Go (문서 기준) | 정책 5건 고정 반영 완료 |
| 2026-02-26 | Documentation Review | Tier Review Ready | T1~T4 checklist 재검토 + gap 정합성 반영 준비 완료 |
| 2026-02-26 | Worker-1 Revalidation | Conditional Go (구현 기준) | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, archive allow-unverified/idempotent + proposal path 입력 + archived 이후 신규 proposal 회귀 수정 |
| 2026-02-26 | Worker-3 Revalidation | Conditional Go (보강 구현) | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, tier gate/approval/verify-diff/compound/compaction/worktree deterministic 보강 |
| 2026-02-26 | Worker-2 Final Revalidation | Go (잔여 갭 해소) | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, verify diff/approval policy/compound triage/compaction/worktree no-side-effect 최종 검증 |
| 2026-02-26 | Worker-1 Final Verification | Go (재검증) | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, tier gate/approval/verify diff/compound/compaction/worktree deterministic/no-side-effect 재확인 |
| 2026-02-26 | Worker-2 Runtime Hardening Final | Go (production readiness) | `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/summary.json`) |
| 2026-02-26 | Worker-3 Runtime Hardening Final | Go (runtime gaps closed) | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/tmp/ax-worker3-evidence-20260226-210011/runtime-summary.json`) |
| 2026-02-26 | Worker-1 Runtime Hardening Revalidation | Go (production-ready) | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/tmp/ax-prod-soak-20260226-120148/summary.json`) |
| 2026-02-26 | Worker-1 Runtime/Recover/Doctor Smoke Refresh | Go (runtime contract reconfirmed) | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary runtime-mode(shared)+doctor runtime JSON+recover auto 포함 smoke 시나리오 PASS (`/tmp/ax-worker1-revalidation-20260226-121531-GiZxMm/runtime-smoke-summary.json`) |
| 2026-02-26 | Worker-3 Final Runtime Gate | Go (final release gate revalidated) | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary runtime-mode(shared/worktree) + recover auto(rerun/resume) + doctor runtime JSON + E2E smoke PASS (`/tmp/ax-worker3-final-20260226-211549/runtime-smoke-summary.json`) |
| 2026-02-26 | Leader Final Production Gate | Go (final confirmation) | `go test -count=1 ./...` PASS, `go test -count=1 -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, full runtime smoke PASS (`/tmp/ax-live-verify-debug3-iYIOCG/live-runtime-summary.json`), soak/load/kill PASS (`/tmp/ax-live-soak-final-CVRKO7/summary.json`) |

---

## Requirements Alignment Readiness (Gap Sync Prep)
- 기준 문서: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- 상태: **Complete (stage gates revalidated for remaining code gaps)**
- 준비 체크리스트:
  - [x] 1/3, 2/3, 3/3 문서의 Tier 용어/게이트 기준 통일
  - [x] Decision Lock 5항목 값 일치 확인
  - [x] T1~T4 checklist ↔ AT 매핑/근거 섹션 정리
  - [x] 구현 실행 증거(테스트 로그/빌드 로그/E2E) 주입
  - [x] runtime hardening 운영 검증 증거(soak/load/kill + race/vet/build + runtime-mode/recover/doctor smoke) 최신 반영 (`/tmp/ax-worker3-final-20260226-211549/runtime-smoke-summary.json`)

---

## Gap 1:1 Alignment (Final Gate Revalidation)

| Gap 기준 | 실행 게이트 | 승인 기준 |
|---|---|---|
| §4.1/§4.2/§4.6/§4.8 (propose/plan/discover/state Must) | Stage 1 | RT-01/02/05/07 + Batch A(T1) 통과 |
| §4.3/§4.4/§4.7 (run/verify/quick Must) | Stage 2 | RT-03/04/06 + Batch B(T2) 통과 |
| §4.9 (app-server 최소 계약) | Stage 3 | RT-08 + AT-11 통과 |
| §10.1~§10.3 (상태/컨텍스트) | Stage 1 + Batch A(T1) | AT-07/13/20/25/26 통과 |
| §10.7~§10.8 + §14.1/3/5/6 (run/tdd/approval/depth/quality gate) | Stage 2 + Batch B(T2) | AT-01/02/03/04/06/08/10/16/17/18/19/21/22/24 통과 |
| §10.4/§10.6 + §14.2 + §12 (compound/engine/worktree/ops) | Stage 3 + Batch C(T3) | AT-09/11/12/14/15/23 + supplemental 체크 통과 |
| §10.5 + Final QA | Stage 3~4 + Batch D(T4) | AT-05 + 8-lens Critical/Major 0건 |
| §11 + §16 수용테스트 전체 | Stage 4 | AT-01~AT-26 전수 통과 |

## 결론
- 3/3 실행 문서는 gap 기준을 Stage 게이트/Go-No-Go 규칙으로 재정렬해 1:1 검증 흐름을 고정했다.
- 누락됐던 항목(§4 Must 항목, 전이 메타필드, 3-Layer Context, lifecycle 5메서드, compound 세부 규칙, tdd-go 프로파일)을 RT+AT 게이트로 반영했다.
- 실증 로그(테스트/빌드/E2E/real soak-load-kill)는 재검증 완료되었고, 본 gap-closing 범위의 Stage 2/3 필수 체크박스 및 runtime hardening 리스크는 모두 해소되었다.

---

## Notes
### Business Rules
- 모든 완료 주장은 검증 로그와 함께 기록
- `docs/mvp-walkthrough.md`는 단계 완료마다 동기화

### Out of Scope
- GUI/daemon/ws
- 멀티 프로젝트 통합 스케줄러
