# AX v2 Codex Production Hardening — Worker-3 Implementation Report (Phase 5/6)

- date: 2026-02-27
- worker: `worker-3`
- scope: Phase 5/6 중심 (`streaming ↔ run/TDD 상태 반영`, `TUI 위험 액션 ↔ 실제 엔진 제어 정합`, 검증/보고)
- task: team `3-executor-2-reviewer-go-loop` / task `3`

## 1) 구현 범위 요약

리더 지시에 따라 **reject 경로 확장 변경은 중단**하고, 아래 범위에 집중했다.

1. **run streaming 이벤트 반영 강화**
   - `turn/start(stream=true)` 이벤트(delta/completed)를 run state/checkpoint/runtime-journal에 반영.
   - run report에 streaming 요약(`stream_delta_events`, `stream_completed_events`) 추가.

2. **TUI 위험 액션의 실제 엔진 호출 정합성 강화**
   - `retry/resume/interrupt/rollback/fork/steer` 액션 시 Codex adapter를 통해 실제 RPC 호출.
   - 성공/실패 시 `state.run.*` / `last_error` / runtime-journal(`action`, `action_failed`) 반영.

3. **Phase 5/6 테스트 보강**
   - streaming 소비 유틸 단위 테스트 추가.
   - run report/journal streaming 반영 검증 테스트 추가.
   - TUI steer/interrupt/fork/rollback 제어 정합 테스트 추가.

## 2) 변경 파일 (worker-3 scope)

- `cmd/ax/run_streaming.go` (new)
- `cmd/ax/tui_action_engine.go` (new)
- `cmd/ax/run_streaming_test.go` (new)
- `cmd/ax/commands.go` (streaming summary + TUI engine action 통합)
- `cmd/ax/commands_test.go` (Phase5/6 정합 테스트 보강)

## 3) 핵심 구현 상세

### 3.1 streaming -> run/TDD 상태 동기화
- `consumeRunStreamEvents(...)` 추가:
  - delta/completed 이벤트 타입 정규화
  - 이벤트마다 state 저장 + runtime checkpoint 갱신 + journal 기록(`stream_delta`, `stream_completed`)
  - completed 이벤트 누락 시 실패 처리
- run 완료 보고서에 streaming 통계 기록 추가:
  - top-level fields
    - `stream_delta_events`
    - `stream_completed_events`
  - `## Streaming Summary` 섹션

### 3.2 TUI 제어 -> 실제 엔진 호출
- `performTUIEngineAction(...)` 추가:
  - retry: `RunTurn`
  - resume: `ResumeSession`
  - interrupt: `InterruptTurn`
  - rollback: `RollbackTurns`
  - fork: `ForkSession`
  - steer: `SteerTurn`
- `executeTUIAction(...)`에서 엔진 액션 호출 결과를 state/journal에 반영:
  - 성공: `stage=action`
  - 실패: `stage=action_failed`, mapped error 기록

## 4) 검증 실행 결과

### 4.1 Phase5/6 타깃 테스트 (PASS)

```bash
go test -count=1 ./cmd/ax -run 'TestRunRealModeStreamingSummaryAndJournal|TestTUIActionSteerUpdatesStateWithEngineCall|TestTUIActionInterruptFailureSetsMappedError|TestTUIActionForkAndRollbackSynchronizeEngineState|TestConsumeRunStreamEventsPersistsCountsAndJournal|TestConsumeRunStreamEventsFailsWithoutCompletedEvent|TestConsumeRunStreamEventsUpdatesTDDCurrentStep|TestConsumeRunStreamEventsWritesCheckpoint|TestNormalizeRunStreamEventType|TestTUISnapshotAndGuardedAction|TestRunRealModeUsesPerRPCTimeoutContext'
```

- result: `ok github.com/flowkater/ax/cmd/ax`

```bash
go test -count=1 -race ./cmd/ax -run 'TestRunRealModeStreamingSummaryAndJournal|TestTUIActionSteerUpdatesStateWithEngineCall|TestTUIActionInterruptFailureSetsMappedError|TestTUIActionForkAndRollbackSynchronizeEngineState|TestConsumeRunStreamEventsPersistsCountsAndJournal|TestConsumeRunStreamEventsFailsWithoutCompletedEvent|TestConsumeRunStreamEventsUpdatesTDDCurrentStep|TestConsumeRunStreamEventsWritesCheckpoint|TestNormalizeRunStreamEventType|TestTUISnapshotAndGuardedAction|TestRunRealModeUsesPerRPCTimeoutContext'
```

- result: `ok github.com/flowkater/ax/cmd/ax`

### 4.2 전체 검증 커맨드 실행 결과(현재 워크트리 기준)

```bash
go test -count=1 ./...
```
- result: **FAIL**
- failing test: `TestRunDecisionRejectPropagatesInterruptFailure`
- observed message: `expected reject interrupt failure message, got: run rejected by decision gate`

```bash
go test -count=1 -race ./...
```
- result: **FAIL**
- failing test: `TestRunDecisionRejectPropagatesInterruptFailure`

```bash
go vet ./...
```
- result: PASS

```bash
go build ./...
```
- result: PASS

```bash
go test -count=1 ./internal/codex -run 'TestStdioClient|TestResolveConfig|TestScaffold'
```
- result: PASS

```bash
go test -count=1 ./cmd/ax -run 'TestRunRealMode|TestRunDecisionReject|TestRecover|TestRunRetry|TestHelperProcessCodexServerAX'
```
- result: **FAIL** (`TestRunDecisionRejectPropagatesInterruptFailure`)

## 5) 남은 리스크 / handoff

1. **Reject interrupt propagation 경로**는 현재 전체 게이트 실패의 직접 원인.
   - 리더 지시에 따라 worker-3는 reject 관련 `commands.go` 확장을 중단하고 Phase5/6 범위에 집중함.
   - 해당 실패 해결은 reject 담당 변경과 함께 통합 정리가 필요.

2. 본 보고서의 Phase5/6 타깃 범위(streaming/TUI)는 race 포함 PASS로 확인됨.

## 6) 결과물 경로

- implementation report: `docs/ax-v2-phase5-6-streaming-tui-implementation-report-worker3-2026-02-27.md`
