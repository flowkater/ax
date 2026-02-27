# ax v2 Codex App-Server 실제 연동 구현 계획 (보강본)

- 작성일: **2026-02-26**
- 대상 범위: `ax run/recover/doctor/tui` ↔ `internal/codex` ↔ codex app-server(JSON-RPC over stdio)
- 현재 상태: `internal/codex/client.go`는 구현 완료, CLI 오케스트레이션 연결은 미완료
- 목표: **scaffold 기반 가짜 turn 흐름을 실제 Codex Thread/Turn 흐름으로 전환**하면서, 기존 사용자 경험(폴백/복구/테스트 안정성)을 유지

---

## 0) 문서 리뷰 결과 (보강 포인트)

기존 문서는 방향성이 좋았지만, 실제 구현자가 바로 착수하기엔 아래가 부족했다.

1. **Phase별 종료 조건(Gate) 불명확**
   - "언제 다음 Phase로 넘어가도 되는지" 기준이 약함.
2. **함수/파일 단위 작업 쪼개기 부족**
   - `runPlan()`에 무엇을 어디까지 넣고 분리할지 구체성이 필요.
3. **실패 시 복구 경로의 구현 체크리스트 부족**
   - `resume/fork/rerun/interrupt/rollback` 호출 순서와 상태 반영 지점 보완 필요.
4. **테스트 매트릭스가 Phase와 1:1 매핑되지 않음**
   - 구현 체크리스트 + 테스트 체크리스트 + 완료 기준을 Phase마다 분리해 보강.

아래부터는 이를 반영한 **실행 가능한 상세 계획**이다.

---

## 1) 현재 상태 진단 (코드 기준)

### 1.1 이미 완료된 기반

| 항목 | 상태 | 근거 파일 |
|---|---|---|
| `AppServerAdapter` 계약(9개 메서드) | ✅ | `internal/codex/adapter.go` |
| `StdioClient` JSON-RPC 호출 + timeout/retry/backoff | ✅ | `internal/codex/client.go` |
| stream 정규화(`normalizeStreamEvents`) | ✅ | `internal/codex/client.go` |
| helper-process 기반 codex client 테스트 | ✅ | `internal/codex/client_test.go` |

### 1.2 아직 비어있는 연결 지점

| 항목 | 상태 | 근거 파일 |
|---|---|---|
| CLI에서 codex adapter 생성/주입 | ❌ | `cmd/ax/commands.go` (`newRunCmd`, `runPlan`) |
| 실제 thread/turn 상태 영속화 | ❌ | `internal/core/state.go` (`RunState`) |
| run 단계별 실 turn 실행 루프 | ❌ | `cmd/ax/commands.go` (`runPlan`) |
| recover 전략에 codex 세션 반영 | ❌ | `cmd/ax/commands.go` (`newRecoverCmd`) |
| doctor/tui에서 codex 상태 노출 | ❌ | `cmd/ax/commands.go`, `internal/core/tui.go` |

### 1.3 핵심 전환점

```text
현재:  runPlan() -> syntheticThreadID + 가짜 Step↔Turn 표
목표:  runPlan() -> CreateThread/ResumeSession -> Step당 RunTurn/SteerTurn -> state에 실제 turn 누적
```

---

## 2) 설계 원칙 (고정)

1. **기본값 scaffold 유지**: `AX_CODEX_MODE` 기본값은 `scaffold`.
2. **DI 경계 유지**: CLI는 `AppServerAdapter`만 의존.
3. **state 하위호환 유지**: 기존 `state.yaml`은 깨지지 않아야 함.
4. **한 Step = 한 Turn**: TDD/일반 run 모두 동일 규칙 유지.
5. **실패 우선 기록**: 에러 시 먼저 `state/run log` 저장 후 반환.

---

## 3) Phase 실행 계획 (구현 상세 + 체크리스트)

## Phase 0 — Config/Factory/Scaffold Adapter

### 목표
`real/scaffold` 실행 모드를 안전하게 선택하고, CLI에서 adapter를 생성할 수 있게 한다.

### 구현 상세

#### 0-1. 설정 계약 추가
- 파일: `internal/codex/factory.go` (신규)
- 구조체:
  - `ClientConfig{ BinPath, Args, Timeout, Retries, Mode }`
- 함수:
  - `ResolveConfig() ClientConfig`
  - `NewAdapter(cfg ClientConfig) (AppServerAdapter, error)`

#### 0-2. 환경변수 파싱 규칙
- `AX_CODEX_MODE`: `real|scaffold` (기본 `scaffold`)
- `AX_CODEX_BIN`: 기본 `codex`
- `AX_CODEX_ARGS`: `strings.Fields`로 파싱
- `AX_CODEX_TIMEOUT`: 기본 `30s`, 파싱 실패 시 기본값
- `AX_CODEX_RETRIES`: 기본 `2`, 음수 금지

#### 0-3. ScaffoldAdapter 도입
- 파일: `internal/codex/scaffold.go` (신규)
- `AppServerAdapter` 전 메서드 구현:
  - `CreateThread`: synthetic thread 반환
  - `RunTurn`: synthetic turn 반환
  - lifecycle 메서드: no-op에 가까운 결정적 반환
  - `StreamTurn`: `delta -> completed` 2이벤트 고정

#### 0-4. CLI 주입 준비
- 파일: `cmd/ax/commands.go`
- `newRunCmd()`에서 `cfg := codex.ResolveConfig()` + `engine, err := codex.NewAdapter(cfg)` 생성
- `runPlan(..., engine codex.AppServerAdapter, ...)`로 시그니처 변경 준비

### 체크리스트

#### 구현
- [x] `internal/codex/factory.go` 추가
- [x] `internal/codex/scaffold.go` 추가
- [x] `cmd/ax/commands.go`에 engine 생성/전달 코드 추가

#### 테스트
- [x] `internal/codex/factory_test.go`: env 조합별 mode/timeout/retries 검증
- [x] `internal/codex/scaffold_test.go`: synthetic thread/turn 결정성 검증
- [x] `cmd/ax/commands_test.go`: `AX_CODEX_MODE=scaffold` 기본 동작 회귀 검증

#### Phase Gate
- [x] `go test ./internal/codex ./cmd/ax -run 'Factory|Scaffold|Run'` PASS
- [x] mode를 `real/scaffold`로 바꿔도 run 진입이 panic 없이 동작

---

## Phase 1 — State 스키마 확장 (Thread/Turn 영속화)

### 목표
run/recover가 codex 세션을 재사용할 수 있도록 state에 thread/turn 메타데이터를 저장한다.

### 구현 상세

#### 1-1. RunState 확장
- 파일: `internal/core/state.go`
- 추가 필드:
  - `ThreadID string \`json:"thread_id,omitempty"\``
  - `ActiveTurnID string \`json:"active_turn_id,omitempty"\``
  - `TurnHistory []TurnRef \`json:"turn_history,omitempty"\``
  - `EngineMode string \`json:"engine_mode,omitempty"\``
- 신규 타입:
  - `TurnRef{TurnID, Step, Status, StartedAt, EndedAt}`

#### 1-2. 상태 헬퍼 추가
- 파일: `internal/core/state.go`
- 권장 헬퍼:
  - `AppendTurnRef(ref TurnRef, max int)`
  - history 상한 `100` 유지(초과 시 앞에서 trim)

#### 1-3. 하위호환 규칙
- 기존 state 로드 시 신규 필드 비어도 정상
- `omitempty` 유지
- `normalize()`에서 nil slice 안전화

### 체크리스트

#### 구현
- [x] `RunState`/`TurnRef` 필드 추가
- [x] turn history 상한 유지 로직 추가
- [x] `run` 성공/실패 시 필드 초기화 정책 명시(`active_turn_id` clear 시점)

#### 테스트
- [x] `internal/core/state_test.go`: 구버전 JSON/YAML 로드 호환
- [x] `TurnHistory` 직렬화/역직렬화
- [x] 100건 초과 시 trim 검증

#### Phase Gate
- [x] `go test ./internal/core -run 'State|RunState|Turn'` PASS
- [x] 기존 fixture/테스트가 스키마 변경으로 깨지지 않음

---

## Phase 2 — runPlan 실제 Turn-by-Step 실행 연동 (핵심)

### 목표
`runPlan()`이 synthetic turn 표 생성이 아니라 실제 `AppServerAdapter` 호출 루프로 동작하도록 전환한다.

### 구현 상세

#### 2-1. 시그니처 변경
- 파일: `cmd/ax/commands.go`
```go
func runPlan(base, plan string, opts runOptions, rt runtimeContext, engine codex.AppServerAdapter, now time.Time) (string, int, error)
```

#### 2-2. Thread 확보 로직
1) `opts.resume && st.Run.ThreadID != ""` → `engine.ResumeSession(threadID)`
2) 그 외 → `engine.CreateThread(planTitle)`
3) 성공 즉시 `state.Run.ThreadID`, `state.Run.EngineMode` 저장

#### 2-3. Step 실행 루프
- 기존 `buildStepTurnMappings`는 "실행 계획 표시용"으로 유지 가능
- 실제 실행은 step loop에서 수행:
  - prompt 생성 (`internal/codex/prompt.go`)
  - decision 분기
    - `accept`: `RunTurn` 또는 `StreamTurn`
    - `steer`: `SteerTurn(threadID,lastTurnID,instruction)`
    - `reject`: 활성 turn 있으면 `InterruptTurn`
  - 성공 시 `TurnHistory append`, `ActiveTurnID clear`
  - 실패 시 `LastFailedStep/ErrorCode/ErrorSummary/RecoverHint` 저장

#### 2-4. Prompt Builder 도입
- 파일: `internal/codex/prompt.go` (신규)
- 함수:
  - `BuildStepPrompt(step stepTurnMapping, planBody string, ctx core.ContextLayers) string`
  - `BuildTDDStepPrompt(tier, phase, planBody string) string`

#### 2-5. Run Report/Observability 반영
- run report의 `Step↔Turn Mapping`을 실제 turn id로 채움
- start/end/fail observability 로그에 `thread_id/turn_id/status` 기록

### 체크리스트

#### 구현
- [x] `runPlan` 시그니처/호출부 변경 완료
- [x] thread 확보(create/resume) 분기 구현
- [x] step loop에서 real turn 호출 + state 저장 구현
- [x] `internal/codex/prompt.go` 추가
- [x] run report에 실제 turn id 반영

#### 테스트
- [x] `cmd/ax/commands_test.go`: mock adapter 기반 accept 시나리오
- [x] `cmd/ax/commands_test.go`: steer/reject 분기 호출 검증
- [x] `cmd/ax/commands_test.go`: 실패 시 `LastFailedStep` + `thread_id` 저장 검증
- [x] `internal/codex/prompt_test.go`: 프롬프트 생성 결정성 검증

#### Phase Gate
- [x] `AX_CODEX_MODE=real` + helper process로 run 1회 성공
- [x] `state.yaml`에 `thread_id`, `turn_history[*].turn_id` 실제 값 기록 확인
- [x] `go test ./cmd/ax ./internal/codex -run 'Run|Prompt'` PASS

---

## Phase 3 — Resume/Recover/Decision Lifecycle 연동

### 목표
`ax run --resume`, `ax recover --strategy auto|resume|rerun`가 실제 codex thread 상태를 사용하도록 확장한다.

### 구현 상세

#### 3-1. recover 전략 매트릭스
- 파일: `cmd/ax/commands.go` (`newRecoverCmd`)

| 조건 | 전략 |
|---|---|
| `thread_id` 없음 | rerun |
| `thread_id` 있음 + TDD enabled | resume 우선 |
| `thread_id` 있음 + TDD disabled | rerun 또는 fork (설정값으로 선택 가능) |

#### 3-2. resume 세부 흐름
1) `engine.ResumeSession(threadID)` 성공 확인
2) `TurnHistory`에서 마지막 completed step 찾기
3) 미완료 step부터 `runPlan` 재진입

#### 3-3. steer/reject/interrupt
- `--decision=steer`: 마지막 turn 기준 `SteerTurn`
- `--decision=reject`: `ActiveTurnID` 존재 시 `InterruptTurn`
- 실패 시 recover hint 표준화

#### 3-4. rollback/fork (선택 구현)
- 자동 recover에서 rerun 대신 fork를 쓰는 경우:
  - `newThread := engine.ForkSession(oldThread)`
  - `state.Run.ThreadID = newThread.ID`
- rollback 전략 채택 시:
  - `engine.RollbackTurns(threadID, targetTurnID)` 후 history truncate

### 체크리스트

#### 구현
- [x] `recover auto` 판단에 `thread_id`/`engine_mode` 반영
- [x] `run --resume` 시 `ResumeSession` 호출 및 재개 step 계산
- [x] steer/reject에서 `SteerTurn`/`InterruptTurn` 연결
- [x] (선택) fork/rollback 경로 구현

#### 테스트
- [x] resume 불가(`thread_id` 없음) 에러 메시지 검증
- [x] auto 전략이 조건별로 resume/rerun 올바르게 선택
- [x] reject 시 interrupt 호출 여부 검증
- [x] fork 수행 시 state thread id 갱신 검증

#### Phase Gate
- [x] 실패 run 후 `ax recover --strategy auto`가 예측 가능한 경로로 복구
- [x] `go test ./cmd/ax -run 'Recover|Resume|Decision'` PASS

---

## Phase 4 — 에러 매핑/관측성 표준화

### 목표
Codex JSON-RPC 에러를 ax 에러 모델로 통일하고, 운영 로그에서 thread/turn 단위 추적이 가능하도록 만든다.

### 구현 상세

#### 4-1. 에러 매핑 모듈
- 파일: `internal/codex/errors.go` (신규)
- 함수:
  - `MapCodexError(err error) (code string, message string, retryable bool)`
- 기본 매핑:
  - `-32600 -> AX_ENGINE_INVALID_REQUEST`
  - `-32601 -> AX_ENGINE_METHOD_NOT_FOUND`
  - `-32602 -> AX_ENGINE_INVALID_PARAMS`
  - `-32603 -> AX_ENGINE_INTERNAL`
  - `-32700 -> AX_ENGINE_PARSE_ERROR`

#### 4-2. run 실패 저장 규칙 통일
- `runPlan`에서 adapter 에러 발생 시:
  - `MapCodexError` 결과를 `state.Run.ErrorCode/ErrorSummary`에 저장
  - `RecoverHint`를 retry/resume 가능 여부에 따라 분기

#### 4-3. 로그 스키마 확장
- observability YAML + runtime journal JSONL 모두에 추가:
  - `thread_id`
  - `turn_id`
  - `step`
  - `duration_ms`
  - `engine_mode`

### 체크리스트

#### 구현
- [x] `internal/codex/errors.go` + 매핑 함수 구현
- [x] `runPlan` 실패 경로에서 매핑 함수 사용
- [x] observability/runtime-journal에 turn 메타 필드 기록

#### 테스트
- [x] `internal/codex/errors_test.go`: 매핑/재시도 가능 여부 검증
- [x] `cmd/ax/commands_test.go`: codex 오류 주입 시 state/log 필드 검증

#### Phase Gate
- [x] 동일 에러 입력 시 동일한 `ErrorCode`/메시지 생성(결정성)
- [x] `go test ./internal/codex ./cmd/ax -run 'Error|Observability'` PASS

---

## Phase 5 — Doctor/TUI/운영 가시성 + 롤아웃

### 목표
운영자가 현재 codex 세션 건강상태를 CLI/TUI에서 바로 볼 수 있도록 하고, real 모드 롤아웃 절차를 문서화한다.

### 구현 상세

#### 5-1. doctor runtime 확장
- 파일: `cmd/ax/commands.go` (`newDoctorCmd`)
- JSON 출력 필드 추가:
  - `engine_mode`
  - `thread_id`
  - `active_turn_id`
  - `turn_count`
  - `codex_bin`
  - `codex_reachable` (`GetThread` 기반)

#### 5-2. TUI Engine 화면 확장
- 파일: `internal/core/tui.go`, `cmd/ax/commands.go` (snapshot)
- 표시 항목:
  - thread/turn 상태
  - 최근 turn 결과
  - recover hint

#### 5-3. 롤아웃 가드
1) 기본값은 `scaffold`
2) CI/로컬 smoke에서 `real` 실험
3) 안정화 후 기본값 변경 여부 별도 결정

### 체크리스트

#### 구현
- [x] doctor JSON 계약 확장
- [x] TUI snapshot(Engine) 필드 확장
- [x] 운영 문서(환경변수/실패 대응) 업데이트

#### 테스트
- [x] `cmd/ax/root_test.go`/`commands_test.go`: doctor JSON 필드 검증
- [x] TUI snapshot 테스트에 thread/turn 필드 검증 추가
- [x] helper server 기반 smoke: `run -> recover -> doctor` 흐름 검증

#### Phase Gate
- [x] `ax doctor runtime --json`에서 codex 관련 필드 확인 가능
- [x] `go test ./cmd/ax ./internal/core -run 'Doctor|TUI'` PASS

---

## 4) 전체 의존 관계/실행 순서

```text
Phase 0 (Factory/Mode)
  -> Phase 1 (State schema)
  -> Phase 2 (runPlan real turn loop)  [핵심]
  -> Phase 3 (resume/recover/lifecycle)
  -> Phase 4 (error mapping/observability)
  -> Phase 5 (doctor/tui/rollout)
```

- Phase 0~1은 병렬 가능(충돌 주의)
- **Phase 2 전에는 real mode 품질 평가 금지**
- Phase 3~5는 Phase 2 완료 이후 병렬 분할 가능

---

## 5) 통합 테스트 전략 (Phase 매핑)

### 단위 테스트
- `internal/codex`: `factory/scaffold/prompt/errors`
- `internal/core`: `state run metadata`

### 통합 테스트 (mock adapter)
- `cmd/ax/commands_test.go`
  - run accept/steer/reject
  - resume/recover/fork/interrupt
  - state 및 로그 검증

### E2E (helper process codex server)
- `AX_CODEX_MODE=real`
- 시나리오:
  1. `propose`
  2. `plan`
  3. `run` (thread/turn 생성 확인)
  4. 실패 유도 후 `recover --strategy auto`
  5. `doctor runtime --json` 확인

### 검증 명령
```bash
go test ./...
go build ./...
```

---

## 6) 최종 완료 기준 (DoD)

- [x] real 모드에서 run 시 실제 codex thread/turn 생성 및 state 반영
- [x] `run --resume`/`recover --strategy auto`가 thread 상태 기반으로 동작
- [x] reject/steer/interrupt 경로가 adapter 호출과 state 갱신을 일치시킴
- [x] 에러 코드가 `AX_ENGINE_*`로 표준화되어 기록됨
- [x] doctor/tui에서 codex 세션 가시화 가능
- [x] scaffold 모드 기본 동작 회귀 없음
- [x] `go test ./...` + `go build ./...` PASS

---

## 7) Out of Scope

- codex app-server 자체 구현 변경
- daemon/websocket 기반 장기 연결 아키텍처
- turn 결과 자동 코드 적용(autofix loop)
- 멀티노드 codex 스케줄링


---

## 8) 실행 결과 (2026-02-27)

- 구현 완료 파일(핵심):
  - `internal/codex/factory.go`, `internal/codex/scaffold.go`, `internal/codex/errors.go`, `internal/codex/prompt.go`
  - `cmd/ax/commands.go` (run/recover/doctor/tui codex 연동)
  - `internal/core/state.go` (thread/turn history schema)
- 테스트 추가 파일:
  - `internal/codex/factory_test.go`, `internal/codex/scaffold_test.go`, `internal/codex/errors_test.go`, `internal/codex/prompt_test.go`
  - `cmd/ax/commands_test.go` (real/scaffold/recover/doctor codex 경로)
  - `internal/core/state_test.go`, `internal/core/tui_test.go`
- 최종 검증:
  - `go test ./... -count=1` PASS
  - `go test -race ./... -count=1` PASS
  - `go vet ./...` PASS
  - `go build ./...` PASS
  - smoke evidence:
    - `/tmp/ax-codex-integration-smoke-summary.json`
    - `/tmp/ax-codex-integration-smoke-summary-2.json`

- 리뷰 검증:
  - 자동 reviewer/architect 에이전트 스레드 한도(`max 6`)로 외부 아키텍트 호출이 제한되어,
    로컬 수동 아키텍처 점검 + 전수 테스트/스모크로 대체 검증 수행.
