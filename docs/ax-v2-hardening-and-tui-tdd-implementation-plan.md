# ax v2 하드닝 + TUI TDD 구현 계획

- plan_id: `ax-v2-hardening-tui-tdd-plan-v1`
- status: `execution-complete-verified`
- source: `docs/ax-v2-hardening-and-tui-requirements.md`
- stack: `Go CLI + Go TUI(Bubble Tea 계열)`
- test_command_red: `go test -count=1 ./...`
- test_command_green: `go test -count=1 ./...`
- format_command: `gofmt -w $(git ls-files '*.go')`
- integration_test_command: `go test -count=1 -tags=integration ./...`

---

## Overview
본 문서는 `ax`의 하드닝 + TUI 요구사항을 **실행 가능한 TDD 체크리스트**로 변환한다.  
실행 순서는 **Phase 1 → 6**, 검증 Tier는 **T1 → T4**, 각 항목은 **Red → Green → Refactor** 단위로 독립 수행한다.

---

## Requirements Digest

### 1) Entities / Objects
- `state.yaml` (phase, progress, error/recover metadata, runtime metadata)
- `verify.md`, `verify.json` (verdict/evidence/failed checks)
- `runtime-journal.jsonl` + 상태 요약 정보
- lock 파일/lock 메타(stale 판정 정보 포함)
- run 복구 정보(`last_failed_step`, `error_code`, `error_summary`, `recover_hint`)
- app-server thread/turn/session/lifecycle 구조체
- TUI ViewModel(Screen A~E)

### 2) Actions / Use-cases
- `run`, `run --resume`, `run --retry <n>`
- `verify` (PASS/FAIL/CONDITIONAL_PASS)
- `state --json`, `state --locks`
- app-server lifecycle: Resume/Fork/Rollback/Steer/Interrupt
- `tui` 조회/제어(read-only 기본 + 위험 액션 confirm)
- `tui --snapshot`(headless 스냅샷)

### 3) Rules / Constraints
- verify는 동일 입력에서 결정성 유지
- state 갱신은 atomic write 보장
- lock 실패 시 timeout 내 재시도 후 실패 처리
- stale lock 복구 규칙 강제
- retry 초과 시 `blocked` 전이
- CLI SoT 유지, TUI는 조회/제어 계층
- TUI는 core service 재사용(중복 로직 금지)

### 4) API / Interface Contracts
- verify output schema:
  - `verdict`
  - `evidence.tests/build/criteria`
  - `failed_checks[]`
- run 실패 메타 schema:
  - `last_failed_step`, `error_code`, `error_summary`, `recover_hint`
- observability log schema:
  - `session_id`, `proposal_id`, `plan_id`, `phase`, `step`, `thread_id`, `turn_id`, `status`, `duration_ms`
- app-server error mapping:
  - `AX_ENGINE_*`로 표준화 + 사용자 메시지 분리

### 5) Error / Edge Cases
- verify evidence 누락/불일치
- run resume 지점 불명확/불가능
- lock 획득 경합/lock 파일 고아(stale)
- transport timeout/중단(Interrupt)/rollback 실패
- TUI 비정상 종료 후 재진입
- 위험 액션 confirm bypass 시도

---

## Ambiguity Interview Gate (Locked)

아래 6개 항목은 사용자 승인으로 잠금 확정했다.

1. **`CONDITIONAL_PASS` 임계값**
   - `tests=pass`, `build=pass` 필수
   - `criteria 충족률 >= 80%`
   - `failed_checks`는 `minor`만 허용 (`major/critical` 존재 시 FAIL)
2. **retry/backoff 기본값**
   - 기본 재시도 2회(총 3회 시도)
   - backoff `1s → 2s`, jitter `±20%`
   - `--retry <n>` 허용 범위 `0..5`
   - 초과 시 `blocked` 전이
3. **stale lock timeout**
   - heartbeat 간격 5s
   - 30s 무갱신 시 stale
   - 동일 호스트 PID 미존재 시 즉시 stale 처리
4. **TUI 위험 액션 범위**
   - 상태 변경/실행 트리거 액션 전체 confirm 필수
   - Screen B: retry/resume/강제 재실행
   - Screen E: interrupt/rollback/fork/steer
5. **lifecycle 실패 복구 우선순위**
   - `InterruptTurn` → `GetThread` 재확인 → `RollbackTurns(last_safe_checkpoint)` → `ResumeSession` → 필요 시 `ForkSession`
6. **`ax tui --snapshot` 포맷**
   - 기본 `json`
   - 옵션 `--format json|md`
   - CI/자동화 권장값 `json`

---

## Phase/Tier Checklist

### Phase 1: Contract/Interface/DTO <!-- T1:auto -->
- [x] verify schema 계약 테스트 추가(`verify.json` 필수 필드/enum)
- [x] verify.md + verify.json 동시 생성 계약 테스트 추가
- [x] run 실패 메타 필드 계약 테스트 추가(`last_failed_step/error_code/error_summary/recover_hint`)
- [x] observability 필수 필드 계약 테스트 추가
- [x] error taxonomy prefix 계약 테스트 추가(`AX_INPUT/STATE/ENGINE/VERIFY/ARCHIVE`)
- [x] TUI Screen A~E ViewModel DTO 계약 테스트 추가

### Phase 2: Core Rule/Domain Logic <!-- T2:review -->
- [x] verify verdict 결정 엔진 결정성 로직 구현 및 회귀 테스트 추가
- [x] verify diff 요약(`regressed/improved`) 로직 구현
- [x] retry policy 엔진(최대 횟수/backoff/blocked 전이) 구현
- [x] `run --resume` 재개 포인트 계산 로직 구현
- [x] lock stale 판정/복구 규칙 구현
- [x] 사용자 메시지/내부 상세 로그 분리 규칙 구현

### Phase 3: Application Orchestration <!-- T2:review -->
- [x] run 실패 시 state 저장 오케스트레이션 구현
- [x] `ax run --resume` 커맨드 경로 통합 및 시나리오 테스트 작성
- [x] `ax run --retry <n>` override 커맨드 경로 구현(요구사항 should)
- [x] verify 파이프라인에서 verify.md/json + diff + failed_checks 일관성 보장
- [x] blocked 전이 후 재실행 가드/해제 절차 구현

### Phase 4: Data/Integration <!-- T3:auto -->
- [x] state atomic write(임시파일+rename) 경로 표준화 및 경쟁 테스트 추가
- [x] lock 획득 재시도(timeout) 및 동시 run 경합 테스트 추가
- [x] `ax state --locks` 디버그 출력 구현(요구사항 should)
- [x] runtime-journal 누적 + 최근 요약 조회 명령 구현
- [x] app-server timeout/transport error mapping/retry-safe 경로 구현
- [x] app-server lifecycle 5메서드 구현 + 통합 테스트 작성
- [x] streaming(delta/completed) run 연동 구현(요구사항 should)

### Phase 5: Delivery Surface (CLI + TUI) <!-- T4:auto -->
- [x] Screen A Dashboard 최소 구현(phase/progress/current refs/failure/logs)
- [x] Screen B Runs 최소 구현(list/detail/recover hint/retry-resume trigger)
- [x] Screen C Verify 최소 구현(verdict/failed checks/criteria/compare)
- [x] Screen D Archive 최소 구현(list/metadata/artifact view)
- [x] Screen E Engine 최소 구현(thread/turn/session/interrupt/steer/fork/rollback 제어)
- [x] read-only 기본 정책 + 위험 액션 확인 프롬프트 강제 구현
- [x] refresh interval 설정(기본 1s) + key binding help 상시 노출 구현
- [x] 비정상 종료 후 재진입 안정성 테스트 추가
- [x] `ax tui --snapshot` 최소 구현(요구사항 should)

### Phase 6: E2E/Regression/Performance <!-- T4:auto -->
- [x] verify 결정성 테스트(동일 입력 반복) 통과
- [x] run resume/retry E2E 통과(실패 유도 포함)
- [x] lock 경합 + stale lock 복구 E2E 통과
- [x] app-server timeout/interrupt/rollback E2E 통과
- [x] TUI 모델/키/confirmation/스냅샷 테스트 통과
- [x] full flow E2E 통과(`state→propose→plan→run→verify→archive`, 실패 후 resume 포함)
- [x] 품질 게이트 통과(`go test ./...`, `go build ./...`)

---

## Tier Item Count
| Tier | Items |
|---|---:|
| T1 | 6 |
| T2 | 11 |
| T3 | 7 |
| T4 | 16 |
| **Total** | **40** |

---

## Completion Log
| Date | Phase | Tier | Result | Evidence |
|---|---|---|---|---|
| 2026-02-27 | Planning | T1~T4 | Draft Created | this doc |
| 2026-02-27 | Planning | T1~T4 | Decision Lock Confirmed (6 items) | user-approved values reflected |
| 2026-02-27 | Phase 1~3 | T1~T2 | verify.json 계약/실패 메타/retry-block/run resume-retry 경로 구현 + 테스트 추가 | `cmd/ax/commands.go`, `cmd/ax/commands_test.go` |
| 2026-02-27 | Phase 4 | T3 | atomic state write + lock metadata/heartbeat/stale 감지 + state lock/journal 디버그 출력 구현 | `internal/core/state.go`, `internal/core/lock.go`, `internal/core/lock_test.go`, `cmd/ax/commands.go` |
| 2026-02-27 | Phase 5 | T4 | `ax tui`(snapshot/action-guard) 및 Screen A~E snapshot view model/markdown renderer 구현 | `cmd/ax/root.go`, `cmd/ax/root_test.go`, `cmd/ax/commands.go` |
| 2026-02-27 | Phase 6 | T4 | 최종 검증 및 실바이너리 스모크 통과 | `go test -count=1 ./...`, `go test -count=1 -race ./...`, `go vet ./...`, `go build ./...`, `/tmp/ax-hardening-loop-final-4p0Gw4/summary.json` |
| 2026-02-27 | Phase 5/6 보강 | T4 | Bubble Tea 기반 interactive full-screen TUI(Screen A~E 전환/키도움말/확인프롬프트) 구현 + 전수 검증/실행 스모크 갱신 | `cmd/ax/tui_interactive.go`, `cmd/ax/tui_interactive_test.go`, `cmd/ax/commands.go`, `go.mod`, `/tmp/ax-tui-full-final2-summary.json` |

---

## Notes

### Business Rules
- CLI가 SoT이며 TUI는 동일 core service 경유
- verify는 판정 근거(evidence/failed_checks) 없으면 실패
- 복구 경로(resume/retry)는 state metadata 없으면 시작 자체를 차단

### Out of Scope
- GUI(웹/데스크톱) 제품화
- daemon/WS 전체 아키텍처 전환
- 멀티노드 분산 스케줄러

### Decision Lock (Confirmed)
- Conditional Pass: tests/build pass + criteria>=80% + only minor failed checks
- Retry/backoff: default retry=2 (total 3), 1s/2s, jitter ±20%, `--retry 0..5`
- stale lock: heartbeat 5s, stale at 30s, dead pid immediate stale
- 위험 액션 confirm: 상태 변경/실행 트리거 전부
- lifecycle 복구 우선순위: Interrupt → GetThread → Rollback → Resume → Fork
- `tui --snapshot`: default json, `--format json|md`

### Execution Notes
- Screen A~E는 Bubble Tea 기반 **interactive full-screen TUI**로 동작하며, `ax tui --snapshot`(json|md) headless 경로와 공존한다.
- 위험 액션(`retry/resume/interrupt/rollback/fork/steer`)은 read-only 기본 정책 하에서 확인 프롬프트(y/N) 없이는 실행되지 않는다.
- `ax state --locks`, `ax state --journal`로 운영 디버깅 경로를 제공한다.
