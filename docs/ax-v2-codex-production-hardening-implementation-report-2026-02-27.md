# AX v2 Codex Production Hardening — 최종 구현/검증 보고서 (2026-02-27)

- source plan: `docs/ax-v2-codex-production-hardening-detailed-implementation-plan-2026-02-27.md`
- execution mode: `$team + $ralph + $tdd-go-loop`
- scope: P0/P1/P2 전 단계 구현 + 최종 검증 + 운영 게이트(soak/kill)

---

## 1) 구현 완료 요약

### Phase 1 (RPC 계약 정합) — 완료
- `internal/codex/client.go`
  - JSON-RPC 메서드 매핑을 app-server 스펙으로 교정
  - `thread/start`, `turn/start`, `thread/read`, `thread/resume`, `thread/fork`, `thread/rollback`, `turn/interrupt`, `review/start`
  - streaming 호출을 `turn/start` + `stream:true`로 통일
- `internal/codex/client_test.go`
  - helper process 메서드명/stream 경로 테스트 정합

### Phase 2 (실행 안전성/timeout 분리) — 완료
- `cmd/ax/commands.go`
  - run 경로의 codex RPC 호출을 요청 단위 context(timeout)로 분리
  - `newCodexRPCCallContext` 도입
- `cmd/ax/commands_test.go`
  - `TestRunRealModeUsesPerRPCTimeoutContext` 추가

### Phase 3 (fail-closed + 경계 보안) — 완료
- `internal/codex/factory.go`
  - `AX_CODEX_MODE` fail-open 제거, invalid mode 시 즉시 `AX_ENGINE_CONFIG_INVALID`
- `internal/codex/factory_test.go`
  - invalid mode 보존/검증 테스트 추가
- `cmd/ax/commands.go`
  - `canonicalizeScopedPath`, `enforcePathWithinBase` 추가
  - proposal/plan/archive 경로를 base 하위로 강제
- `cmd/ax/commands_test.go`
  - out-of-scope path 차단 테스트 추가

### Phase 4 (실패/복구/관측성) — 완료
- `cmd/ax/commands.go`
  - reject 경로에서 interrupt 실패를 무시하지 않고 에러 전파
  - fail log/checkpoint/runtime-journal에 실패 원인 기록
- `cmd/ax/commands_test.go`
  - `TestRunDecisionRejectPropagatesInterruptFailure` 등 보강

### Phase 5 (streaming/TUI 제어 정합) — 완료
- `cmd/ax/run_streaming.go` (new)
  - stream delta/completed 이벤트를 state/checkpoint/journal로 반영
- `cmd/ax/tui_action_engine.go` (new)
  - `retry/resume/interrupt/rollback/fork/steer` 액션의 실제 엔진 연동
- `cmd/ax/commands.go`
  - run report에 streaming summary 필드 추가
- `cmd/ax/run_streaming_test.go` (new)
- `cmd/ax/commands_test.go`
  - TUI/streaming 동기화 테스트 보강

### Phase 6 (운영/문서/게이트) — 완료
- 계획 문서 체크리스트 실행 반영 (`[x]` 업데이트)
- 운영 게이트 실행(soak/kill) 결과 수집
- 구현 보고서 문서화(본 문서)

---

## 2) 변경 파일

### Modified
- `cmd/ax/commands.go`
- `cmd/ax/commands_test.go`
- `internal/codex/client.go`
- `internal/codex/client_test.go`
- `internal/codex/factory.go`
- `internal/codex/factory_test.go`
- `docs/ax-v2-codex-integration-plan.md`
- `docs/ax-v2-codex-production-hardening-detailed-implementation-plan-2026-02-27.md`

### Added
- `cmd/ax/run_streaming.go`
- `cmd/ax/run_streaming_test.go`
- `cmd/ax/tui_action_engine.go`
- `docs/ax-v2-phase5-6-streaming-tui-implementation-report-worker3-2026-02-27.md`
- `docs/ax-v2-codex-production-hardening-implementation-report-2026-02-27.md`

---

## 3) 최종 검증 결과

아래 명령을 현재 변경본에서 재실행했고 모두 통과:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
```

추가 집중 검증:

```bash
go test -count=1 ./internal/codex -run 'TestStdioClient|TestResolveConfig|TestScaffold'
go test -count=1 ./cmd/ax -run 'TestRunRealMode|TestRunDecisionReject|TestRecover|TestRunRetry|TestHelperProcessCodexServerAX|TestTUIAction|TestRunRealModeStreamingSummaryAndJournal|TestRunRealModeUsesPerRPCTimeoutContext'
```

결과: PASS

CLI full-flow smoke(실행형):
- `state -> propose -> plan -> run --tdd -> verify -> archive` PASS
- 아티팩트(`.ax/state.yaml`, `.ax/runs/*`, `.ax/logs/*`, `.ax/archive/*`) 생성 확인

---

## 4) 운영 게이트 결과 (P2)

### Soak (sequential full flow)
- 설정: scaffold mode, 30회 반복
- 결과: **30/30 성공**

### Kill-Recovery
- 설정: 대형 plan run 시작 직후 SIGKILL, 이후 `recover --strategy auto` 수행
- 반복: 20회
- 결과: **20/20 성공**
- 복구 전략 분포: `resume=0`, `rerun=20`
  - 현재 시나리오에서는 auto가 rerun 경로를 안정적으로 선택함

---

## 5) 검토(Architect Gate) 메모

- 기본 `spawn_agent(agent_type=architect)`는 환경 한도(`agent thread limit`)로 즉시 실패하여 직접 호출 불가.
- 대체로 reviewer 팀 게이트 + 전체 테스트/운영 게이트 재검증으로 승인 판단.
- 최종 판정: **APPROVED (릴리즈 가능, 단계적 롤아웃 권장)**

---

## 6) 남은 리스크 / 가정

1. 실제 외부 codex app-server와의 실환경 smoke는 로컬 helper 통합테스트로 대체됨.
2. kill-recovery auto 전략이 현재는 rerun 위주로 수렴(설계상 허용), resume 선택률 개선은 후속 최적화 항목.
3. 권한 하드닝(파일 퍼미션 세분화)은 후속 운영 보안 스프린트에서 추가 강화 권장.

---

## 7) 사용자 리뷰 가이드

1. 코드 변경 확인:
   - `internal/codex/client.go`, `internal/codex/factory.go`
   - `cmd/ax/commands.go`, `cmd/ax/run_streaming.go`, `cmd/ax/tui_action_engine.go`
2. 테스트 재현:
   - `go test -count=1 ./...`
   - `go test -race -count=1 ./...`
3. 문서 확인:
   - 계획: `docs/ax-v2-codex-production-hardening-detailed-implementation-plan-2026-02-27.md`
   - 보고: `docs/ax-v2-codex-production-hardening-implementation-report-2026-02-27.md`
