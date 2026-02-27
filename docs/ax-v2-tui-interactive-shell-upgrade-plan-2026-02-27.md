# ax v2 TUI Interactive Command Shell 고도화 계획 (2026-02-27)

- 작성일: **2026-02-27**
- source:
  - `docs/ax-v2-codex-integration-plan.md`
  - `cmd/ax/tui_interactive.go`
  - `cmd/ax/tui_action_engine.go`
- 목적: 현재 “상태 조회 + 키기반 액션” 중심의 TUI를, codex/claude code/opencode 스타일의 **명령 입력형 인터랙티브 쉘**로 확장

---

## 1) 현재 구현 확인 (As-Is)

1. **인터랙티브 화면 구조는 이미 존재**
   - Screen A~E 전환, 주기 refresh, key binding 동작.
   - 근거: `cmd/ax/tui_interactive.go`

2. **위험 액션은 실제 Codex adapter 호출로 연결**
   - `retry/resume/interrupt/rollback/fork/steer`가 `performTUIEngineAction`에서 `AppServerAdapter` lifecycle API를 호출.
   - 근거: `cmd/ax/tui_action_engine.go`

3. **하지만 ‘자유 입력 command line’은 없음**
   - 현재는 키 입력 기반 액션만 가능하고, 사용자 텍스트를 입력/편집/전송하는 입력 박스가 없음.
   - 근거: `cmd/ax/tui_interactive.go` (`updateKey`, `View`)

4. **steer는 고정 문구**
   - 사용자가 steer instruction을 직접 입력하지 못하고 상수 문구를 사용.
   - 근거: `cmd/ax/tui_action_engine.go` (`defaultTUISteerInstruction`)

---

## 2) To-Be 목표

- TUI 안에서 하단 command line으로 명령을 입력하고 즉시 Codex thread/turn으로 전송
- plain text 입력은 `RunTurn`으로, slash-command는 lifecycle/제어 명령으로 매핑
- 응답을 live stream(가능 시 `StreamTurn`)으로 transcript pane에 표시
- 기존 snapshot/action 경로(`ax tui --snapshot`, `ax tui --action`)는 하위호환 유지

---

## 3) UX/명령 모델 (초안)

### 3.1 입력 규칙
- `텍스트` (슬래시 없음): 현재 thread로 `RunTurn`
- `/run <prompt>`: 명시적 `RunTurn`
- `/steer <instruction>`: 현재/직전 turn 대상 `SteerTurn`
- `/interrupt`: 현재 active turn `InterruptTurn`
- `/resume`: `ResumeSession`
- `/fork`: `ForkSession`
- `/rollback [turn_id]`: 지정 turn 또는 active/last turn 기준 `RollbackTurns`
- `/new-thread <title>`: `CreateThread` 후 해당 thread로 포커스
- `/use-thread <thread_id>`: 현재 thread 포커스 전환
- `/help`: 명령 도움말 표시

### 3.2 레이아웃
- 상단: Screen A~E 탭/상태
- 중앙: transcript/log pane (user/assistant/system/stream delta)
- 하단: 입력창(command line) + 상태라인(connected/thread/mode/latency)

---

## 4) 구현 단계 (Phase 6)

### 4.1 Phase 6-1. 입력 컴포저/명령 파서
- [ ] `cmd/ax/tui_interactive.go`에 입력 버퍼/커서/submit key(Enter) 추가
- [ ] `cmd/ax/tui_command_parser.go`(신규): plain/slash-command 파싱
- [ ] 오류 입력 시 사용자 친화 에러 메시지(unknown command, missing arg)

### 4.2 Phase 6-2. command dispatcher + engine bridge
- [ ] `cmd/ax/tui_command_dispatcher.go`(신규): 파서 결과 → `AppServerAdapter` 호출 매핑
- [ ] `retry/resume/interrupt/fork/rollback/steer` 기존 로직은 dispatcher에서 재사용
- [ ] RPC별 독립 timeout context 적용(요청 단위)

### 4.3 Phase 6-3. transcript/stream 반영
- [ ] `StreamTurn` 우선, 실패 시 `RunTurn` fallback
- [ ] stream delta를 TUI pane에 실시간 렌더하고 completed 이벤트에서 turn history/state 반영
- [ ] `.ax/logs/runtime-journal.jsonl`에 `tui_command`, `thread_id`, `turn_id`, `duration_ms`, `status` 기록

### 4.4 Phase 6-4. 상태/보안/하위호환
- [ ] 입력/히스토리 persistence 정책 확정 (`.ax/runtime/tui-history-<session>.jsonl` 권장)
- [ ] 민감 입력 마스킹 정책(토큰/시크릿 패턴) 적용
- [ ] `ax tui --snapshot`/`ax tui --action` 회귀 테스트 보장

---

## 5) 테스트 계획 (Phase 6 게이트)

### 5.1 단위
- [ ] parser: plain/slash/arg/에러 케이스
- [ ] dispatcher: command → adapter 호출 매핑 결정성
- [ ] transcript reducer: stream delta/completed 누적 규칙

### 5.2 통합
- [ ] helper-process real mode에서 `/run`, `/steer`, `/interrupt`, `/fork`, `/rollback` 성공/실패 경로
- [ ] 입력-command 실행 후 state(`thread_id`, `active_turn_id`, `turn_history`) 및 journal 검증
- [ ] race 테스트(`go test -race`)에서 입력/refresh 동시성 안정성 검증

### 5.3 Phase Gate
- [ ] `ax tui`에서 텍스트 입력 3회 이상 연속 전송(plain + slash 혼합) 성공
- [ ] 실패 명령 1회 이상에서 에러코드/복구 힌트/저널 기록 일치
- [ ] 기존 `tui --snapshot` 및 `tui --action` 회귀 PASS

---

## 6) Out of Scope (이번 증분)

- 로컬 OS shell 명령 실행(`!cmd`) 직접 지원
- 웹소켓/daemon 기반 상시 연결
- 다중 사용자 동시 transcript 협업
