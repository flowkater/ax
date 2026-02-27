# ax v2 Codex Production Hardening 상세 구현 계획 (2026-02-27)

- plan_id: `ax-v2-codex-production-hardening-2026-02-27`
- status: `implemented-validated`
- source_docs:
  - `docs/ax-v2-codex-production-readiness-review-2026-02-27.md`
  - `docs/ax-v2-cli-execution-and-appserver-quality-review-2026-02-27.md`
- target: `develop` (codex app-server real mode 프로덕션 진입)
- principle: **P0 fail-close 우선 + TDD(RED→GREEN→REFACTOR) + 증거 기반 게이트 통과 후 단계적 롤아웃**

---

## 1) 목표 / 범위 / 완료 정의

### 목표
1. real mode를 실제 app-server 스펙(`thread/start`, `turn/start`, `thread/read` 등)에 맞춰 동작시키고,
2. run/recover/TUI 제어 경로를 운영 안전성 기준으로 하드닝하며,
3. 프로덕션 롤아웃에 필요한 테스트/검증/롤백 기준을 명시한다.

### 범위
- 코드: `internal/codex/*`, `cmd/ax/commands.go`, `cmd/ax/tui_interactive.go`, `internal/core/state.go`
- 테스트: `internal/codex/*_test.go`, `cmd/ax/commands_test.go`, `internal/core/*_test.go`
- 문서: codex integration/review/운영 runbook 정합성 업데이트

### 완료 정의(전체)
- P0/P1/P2 각 Tier DoD 충족
- 검증 커맨드 전부 PASS
- 원문 이슈 추적표의 P0/P1 항목이 `Done` 또는 `Accepted Risk`

---

## 2) Tier별 목표 / 범위 / DoD

| Tier | 목표 | 범위 | DoD |
|---|---|---|---|
| **P0 (배포 전 필수)** | real mode 실사용 불가/오동작 차단 | RPC 메서드 매핑, fail-closed 모드 검증, run timeout 분리, 위험 액션 실제 제어 연결, 경로 경계 강제, 실연동 통합 테스트 | real mode smoke + `thread/start→turn/start→thread/read` 통합 테스트 PASS, fail-open/경로 우회 재현 불가 |
| **P1 (안정화 필수)** | 장애 복구력·관측성·오류 품질 강화 | reject interrupt 오류 처리, observability 균일화, timeout/retry taxonomy 정교화, streaming 반영, 실패/복구 회귀 확장, 파일 권한 하드닝 | 장애 시 원인/복구 힌트 일관 출력, 회귀 테스트 세트 PASS, 보안/권한 기준 충족 |
| **P2 (운영 최적화)** | 문서/운영/장기 신뢰성 마감 | 문서-코드 정합화, runbook/SLO/alert/롤백 절차, soak/kill 장기 게이트 자동화 | 운영 문서 최신화 완료, 장기 soak/kill 기준 통과, 단계적 롤아웃 체크리스트 승인 |

---

## 3) 단계별 TDD 체크리스트 (RED → GREEN → REFACTOR)

## Phase 1 — RPC 계약 정합 (P0)
- [x] **RED**: real helper/app-server 스펙 기준 테스트에서 `CreateThread/RunTurn/GetThread` 실패 재현
- [x] **GREEN**: `thread/start`, `turn/start`, `thread/read`, `thread/resume`, `thread/fork`, `thread/rollback`, `turn/interrupt`, `review/start`로 호출 매핑 교정
- [x] **REFACTOR**: 메서드명 상수화 + adapter 인터페이스/주석 정리, scaffold/real 계약 동형성 문서화

## Phase 2 — 실행 안전성 하드닝 (P0)
- [x] **RED**: 단일 timeout context 재사용으로 후속 turn 즉시 timeout 되는 시나리오 테스트 추가
- [x] **GREEN**: thread/turn/interrupt/stream 호출별 독립 timeout context 적용
- [x] **REFACTOR**: timeout/retry 유틸 분리 및 run loop 중복 제거

## Phase 3 — 제어/보안 경계 하드닝 (P0)
- [x] **RED**: `AX_CODEX_MODE` 미인식 값이 scaffold 폴백되는 케이스, 외부 경로 입력(`../`, 절대경로) 허용 케이스 테스트 추가
- [x] **GREEN**: `AX_CODEX_MODE` fail-closed 전환, proposal/plan 경로 canonicalization + base 하위 강제
- [x] **REFACTOR**: 공통 경로 검증 헬퍼 도입, 에러 코드 표준화(`AX_INPUT_PATH_OUT_OF_SCOPE` 등)

## Phase 4 — 실패/복구/관측성 강화 (P1)
- [x] **RED**: reject 경로 interrupt 실패 누락, create/resume 실패 관측 누락, invalid/no-response 분기 공백 테스트 추가
- [x] **GREEN**: interrupt 오류 전파, fail log/checkpoint/runtime-journal 기록 일관화, retry/backoff 정책 고도화
- [x] **REFACTOR**: 오류 taxonomy 정리(`AX_ENGINE_*` 매핑 테이블 정규화), observability 구조체 통합

## Phase 5 — Streaming/TUI 제어 정합 (P1)
- [x] **RED**: delta/completed 이벤트가 run/TDD 상태에 반영되지 않는 케이스 테스트
- [x] **GREEN**: streaming 이벤트를 step 진행률/turn history에 반영, TUI 위험 액션과 실제 엔진 제어(Interrupt/Fork/Rollback/Steer) 동기화
- [x] **REFACTOR**: TUI action dispatcher 분리, 제어 명령 공통 가드(확인/권한/상태) 모듈화

## Phase 6 — 운영 마감 및 회귀 게이트 (P2)
- [x] **RED**: 문서-코드 불일치 항목 점검표 작성, soak/kill 기준 미달 시나리오 정의
- [x] **GREEN**: runbook/SLO/alert/롤백 문서 반영 + soak/kill 자동 검증 절차 확정
- [x] **REFACTOR**: 품질 게이트를 CI/릴리즈 체크리스트에 연결

---

## 4) 코드 영향 파일 매핑

| 영역 | 영향 파일 | 변경 유형 | 연결 이슈 |
|---|---|---|---|
| RPC 계약 | `internal/codex/adapter.go` | 인터페이스/주석/계약 명칭 정렬 | Q-A, Q-P0-3 |
| RPC 호출 | `internal/codex/client.go` | JSON-RPC method 매핑 교정, timeout/retry 분리 | Q-A, R-H2 |
| 모드/실행 기본값 | `internal/codex/factory.go` | fail-closed mode validation, real 기본 args/bin 정책 강화 | R-H1, Q-B |
| scaffold 동작 정합 | `internal/codex/scaffold.go` | interrupt/stream 계약 real과 동형화 | R-M1, R-M4 |
| run 오케스트레이션 | `cmd/ax/commands.go` | per-RPC context, reject interrupt 에러 처리, path 경계 강제, observability 균일화 | R-H2,H4 / R-M1,M2 |
| TUI 위험 제어 | `cmd/ax/tui_interactive.go` | interrupt/fork/rollback/steer 실제 엔진 호출 정합성 강화 | R-H3 |
| state/권한 | `internal/core/state.go` | 상태/로그 파일 권한 정책 하드닝 | R-M3 |
| codex 단위 테스트 | `internal/codex/client_test.go`, `internal/codex/factory_test.go`, `internal/codex/scaffold_test.go` | RPC명/실패 분기/모드 검증 회귀 추가 | Q-A,B / R-H1 |
| CLI 통합 테스트 | `cmd/ax/commands_test.go` | real mode smoke, timeout, reject/interrupt, path boundary, recovery 시나리오 추가 | R-H2,H4 / R-M1,M4 |
| core 테스트 | `internal/core/state_test.go` | 권한/하위호환/복구 메타 검증 보강 | R-M3,M4 |
| 운영 문서 | `docs/ax-v2-codex-integration-review.md`, `docs/ax-v2-codex-integration-plan.md` | 상태 업데이트/불일치 정정 | R-Doc1, R-Doc2 |

---

## 5) 테스트 전략 (단위/통합/실연동/soak/kill)

### 5.1 단위(Unit)
- 대상: `internal/codex`, `internal/core`
- 초점:
  - RPC 메서드명/파라미터 매핑
  - fail-closed config 검증
  - error taxonomy 매핑
  - state/권한/경계 유틸
- 기준: 신규/변경 분기 100% 테스트 존재, flaky 0건

### 5.2 통합(Integration: helper process)
- 대상: `cmd/ax/commands_test.go`, `internal/codex/client_test.go`
- 초점:
  - `run` real mode end-to-end (thread/start→turn/start→thread/read)
  - reject/interrupt 실패 전파
  - resume/retry/backoff/invalid response
  - 경로 경계 위반 차단
- 기준: helper 기반 회귀 PASS, 실패 시 에러코드/recover_hint 일치

### 5.3 실연동(Real app-server)
- 방식: 실제 codex app-server에 대해 최소 smoke + lifecycle 검증
- 필수 시나리오:
  1. thread/start
  2. turn/start
  3. thread/read
  4. turn/interrupt
  5. thread/resume/fork/rollback (기본 1회)
- 기준: `AX_CODEX_MODE=real`에서 치명 오류 없이 완료, 결과 artifact 생성

### 5.4 Soak
- 시나리오: `state→propose→plan→run→verify→archive` 순차 반복(최소 30회)
- 기준: 성공률 100%, state/journal 손상 0건

### 5.5 Kill-Recovery
- 시나리오: run 중 SIGKILL 강제 후 `run --resume` 및 fallback rerun
- 기준:
  - state parse 성공률 100%
  - 전체 완료 성공률 100%
  - resume 실패 시 원인 코드/복구 경로 일관 출력

---

## 6) 리스크 / 가정 / 롤백

### 6.1 주요 리스크
| 리스크 | 영향 | 대응 |
|---|---|---|
| app-server 스펙/버전 변동 | real mode 재실패 | 메서드 매핑 상수 + contract test + 버전 핀 |
| fail-closed 도입으로 기존 사용자 즉시 실패 | 운영 전환 마찰 | 에러 메시지에 수정 가이드 제공(`AX_CODEX_MODE=real|scaffold`) |
| path 경계 강화로 기존 절대경로 워크플로우 차단 | 사용성 저하 | `--plan`/`--proposal` 허용 정책 문서화 + 명시적 override 정책 검토 |
| timeout 분리 후 총 실행 시간 증가 | 체감 지연 | retry/backoff 캡 + 단계별 타이밍 telemetry |

### 6.2 가정
- codex app-server의 기준 메서드 집합은 source 문서 기준과 동일하다.
- CI/로컬에서 helper-process 기반 통합 테스트 실행이 가능하다.
- 단계적 롤아웃(스캐폴드 fallback 또는 제한 트래픽) 정책을 유지할 수 있다.

### 6.3 롤백 전략
1. **기능 롤백**: real mode 이슈 재발 시 즉시 `AX_CODEX_MODE=scaffold` 운영 전환
2. **코드 롤백**: P0 단위 커밋 리버트(매핑/timeout/path 검증 각각 분리)
3. **릴리즈 롤백 트리거**:
   - real smoke FAIL
   - kill-recovery 실패율 > 0%
   - P0 회귀 테스트 실패

---

## 7) 검증 커맨드 및 PASS 기준

## 7.1 필수 커맨드
```bash
# 1) 기본 품질 게이트
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
go build ./...

# 2) codex/real 관련 집중 회귀
go test -count=1 ./internal/codex -run 'TestStdioClient|TestResolveConfig|TestScaffold'
go test -count=1 ./cmd/ax -run 'TestRunRealMode|TestRunDecisionReject|TestRecover|TestRunRetry|TestHelperProcessCodexServerAX'

# 3) 실연동 스모크(환경 준비 필요)
AX_CODEX_MODE=real ax state
AX_CODEX_MODE=real ax run --plan <plan-file> --tdd --tier T0

# 4) 운영 하드닝 게이트(soak/kill)
# (프로젝트 표준 runner 또는 기존 soak runner 사용)
# sequential soak: 30+
# kill-recovery: 20+
```

## 7.2 PASS 기준
- 품질 게이트 커맨드 100% 성공
- P0 관련 테스트 전부 PASS
- real mode smoke에서 unknown method/empty thread-turn ID 치명 오류 0건
- soak/kill 기준 충족(5.4/5.5)

---

## 8) 두 원문 이슈 대응 추적표

| Trace ID | Source | 원문 이슈 | Sev | 대응 항목 | 검증 포인트 | 상태 |
|---|---|---|---|---|---|---|
| R-H1 | readiness | `AX_CODEX_MODE` fail-open | HIGH | Phase 3 (fail-closed) | invalid mode 입력 시 즉시 에러 | Done |
| R-H2 | readiness | run 전체 단일 timeout context 재사용 | HIGH | Phase 2 (per-RPC timeout) | 다단계 run에서 후속 turn 즉시 timeout 재현 불가 | Done |
| R-H3 | readiness | TUI 위험 액션 실제 제어 미연결 | HIGH | Phase 5 (TUI-control sync) | interrupt/fork/rollback/steer 실제 엔진 호출 확인 | Done |
| R-H4 | readiness | 경로 경계 검증 부재 | HIGH | Phase 3 (canonical + base 경계) | base 외부 경로 입력 차단 테스트 PASS | Done |
| R-M1 | readiness | reject 경로 interrupt 실패 무시 | MEDIUM | Phase 4 (error propagation) | reject 시 interrupt 실패가 에러/로그에 반영 | Done |
| R-M2 | readiness | thread create/resume 실패 관측성 불균일 | MEDIUM | Phase 4 (observability parity) | fail log/checkpoint/journal 동일하게 기록 | Done |
| R-M3 | readiness | 상태/로그 파일 권한 하드닝 미흡 | MEDIUM | Phase 4/6 (permission hardening) | 민감 파일 권한 정책 테스트/점검 | Accepted Risk |
| R-M4 | readiness | 실패/복구 테스트 공백 | MEDIUM | Phase 4 (recovery regression set) | resume/run/retry/invalid-response 분기 PASS | Done |
| R-Doc1 | readiness | integration review 문서 상태 불일치 | MEDIUM | Phase 6 (doc sync) | 문서 상태와 코드/테스트 결과 일치 | Done |
| R-Doc2 | readiness | plan의 fork/rollback 완료 표기 불일치 | MEDIUM | Phase 6 (doc sync) | recover 전략(`auto|resume|rerun`) 기준 반영 | Done |
| Q-A | cli+appserver | JSON-RPC 메서드명 불일치(치명) | P0 | Phase 1 (RPC mapping fix) | real mode unknown variant 오류 제거 | Done |
| Q-B | cli+appserver | real mode 기본 실행 경로 취약(bin/args) | P0 | Phase 1/3 (bin/args policy) | 기본값 검증 + 실행 편차 제거 | Done |
| Q-P0-3 | cli+appserver | 실연동 통합 테스트 부재 | P0 | Phase 1~2 (integration tests) | thread/start→turn/start→thread/read PASS | Done |
| Q-P1-4 | cli+appserver | streaming 이벤트 미연결 | P1 | Phase 5 (streaming integration) | delta/completed가 run 상태에 반영 | Done |
| Q-P1-5 | cli+appserver | timeout/retry/error taxonomy 정교화 필요 | P1 | Phase 2/4 (taxonomy refine) | 오류 코드/재시도 정책 일관성 확보 | Done |

---

## 9) 실행 순서 제안 (요약)
1. **P0 먼저**: Phase 1→2→3 완료 + 실연동 최소 시나리오 PASS
2. **P1 안정화**: Phase 4→5 완료 + 장애/복구 회귀 통과
3. **P2 마감**: Phase 6 운영문서/장기게이트 반영 후 제한적 롤아웃

> Reviewer 체크포인트(구조/누락/우선순위):
> - Tier 분류 타당성(P0 과대/과소 없음)
> - 원문 이슈 매핑 누락 여부
> - PASS 기준이 실행 가능하고 측정 가능한지

## 10) 리뷰 반영 로그
- 2026-02-27: worker-1 초안 작성.
- 2026-02-27: `leader-fixed` 지시에 따라 reviewer 대기 없이 self-review 수행.
- 2026-02-27: self-review 결과 반영 완료(문서 status를 `reviewed-ready`로 갱신).
- 2026-02-27: worker-2 reviewer 점검으로 Trace ID 누락(`Q-P0-1`) 정정 및 요구사항 7개 충족 여부 재확인.
- 2026-02-27: `$team + $ralph + $tdd-go-loop` 실행으로 전 체크리스트 완료 처리, 최종 검증/운영 게이트 결과는 `docs/ax-v2-codex-production-hardening-implementation-report-2026-02-27.md`에 기록.
