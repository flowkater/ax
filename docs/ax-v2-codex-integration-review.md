# ax v2 Codex App-Server 연동 품질 리뷰

- 리뷰일: 2026-02-27
- 대상: Phase 0~5 codex 연동 구현 전체
- 리뷰 범위: `internal/codex/*`, `cmd/ax/commands.go`, `internal/core/state.go`, 테스트 전체
- 총평: **7.5/10 — 아키텍처 우수, 수정 2건 필요**

---

## 1. Phase별 구현 상태

| Phase | 항목 | 상태 | 근거 파일 |
|-------|------|------|-----------|
| **0** | Factory (`ResolveConfig` + `NewAdapter`) | ✅ | `internal/codex/factory.go` |
| **0** | ScaffoldAdapter (9개 메서드 완전 구현) | ✅ | `internal/codex/scaffold.go` |
| **0** | CLI 주입 (`newRunCmd` → `engine` 생성) | ✅ | `cmd/ax/commands.go` |
| **1** | RunState 확장 (ThreadID/ActiveTurnID/TurnHistory/EngineMode) | ✅ | `internal/core/state.go` |
| **1** | TurnHistory 상한 100건 trim | ✅ | `internal/core/state.go` (`AppendTurnRef`) |
| **1** | 하위호환 (omitempty, normalize 안전화) | ✅ | `internal/core/state_test.go` |
| **2** | runPlan() DI 시그니처 변경 | ✅ | `cmd/ax/commands.go` |
| **2** | Thread 확보 (CreateThread/ResumeSession 분기) | ✅ | `cmd/ax/commands.go:1399-1404` |
| **2** | Turn-by-Step 실행 루프 (RunTurn/SteerTurn/StreamTurn) | ✅ | `cmd/ax/commands.go:1620-1758` |
| **2** | Prompt Builder (TDD/standard 모드) | ✅ | `internal/codex/prompt.go` |
| **2** | Run Report에 실제 turn ID 반영 | ✅ | `cmd/ax/commands.go` |
| **3** | Recover → ThreadID 기반 전략 판단 | ✅ | `cmd/ax/commands.go:441-488` |
| **3** | Steer → SteerTurn 연결 | ✅ | `cmd/ax/commands.go:1629-1638` |
| **3** | Reject → InterruptTurn 연결 | ✅ | `cmd/ax/commands.go:1527-1587` |
| **4** | 에러 매핑 (JSON-RPC → AX_ENGINE_*) | ✅ | `internal/codex/errors.go` |
| **4** | Observability (thread/turn/duration_ms) | ✅ | `cmd/ax/commands.go` (runtime journal) |
| **5** | Doctor codex 필드 (engine_mode/thread_id/codex_reachable) | ✅ | `cmd/ax/commands.go` |
| **5** | TUI Engine 화면 확장 | ✅ | `cmd/ax/tui_interactive.go`, `internal/core/tui.go` |

---

## 2. 아키텍처 평가

### 2.1 DI 경계 — 우수

```
newRunCmd()
  → codex.ResolveConfig()     // 환경변수 resolve
  → codex.NewAdapter(cfg)     // scaffold 또는 StdioClient 반환
  → runPlan(..., engine, ...) // 인터페이스만 의존
```

- 글로벌 변수 없음
- 테스트에서 mock adapter 주입 가능
- scaffold ↔ real 전환이 코드 변경 없이 env var만으로 가능

### 2.2 State 영속화 — 우수

```
RunState {
    ThreadID      // 실제 codex thread ID
    ActiveTurnID  // 현재 실행 중 turn
    TurnHistory   // bounded(100) turn 기록
    EngineMode    // "scaffold" | "real"
}
```

- omitempty로 하위호환 유지
- resume 시 ThreadID 존재 여부로 ResumeSession/CreateThread 분기
- recover auto 판단에 ThreadID + LastFailedStep + ActiveTurnID 3개 조건 사용

### 2.3 실행 루프 — 우수

```
CreateThread/ResumeSession → threadID 확보
  → for each step:
      accept  → RunTurn() 또는 StreamTurn()
      steer   → SteerTurn(lastTurnID, instruction)
      reject  → InterruptTurn() + 중단
      실패    → MapCodexError() + state 저장 + 반환
  → 성공 → state 최종 저장
```

- decision 분기가 adapter 메서드와 1:1 매핑
- StreamTurn은 real 모드의 마지막 step에서만 사용 (안정성 우선)
- 실패 시 state 먼저 저장 후 에러 반환 (실패 우선 기록 원칙 준수)

---

## 3. 발견된 이슈

### Issue #1 — Real 모드에서 synthetic ID 폴백 (심각도: HIGH)

**위치**: `cmd/ax/commands.go:1424`, `cmd/ax/commands.go:1725-1731`

**현상**: real 모드에서 codex가 빈 ID를 반환하면 synthetic ID로 무조건 폴백한다.

```go
// 현재 코드 (thread)
threadID := strings.TrimSpace(thread.ID)
if threadID == "" {
    threadID = syntheticThreadID(st.Runtime.SessionID + "-" + planID)
}

// 현재 코드 (turn)
turnID := strings.TrimSpace(turn.ID)
if turnID == "" {
    turnID = fmt.Sprintf("turn-%03d", i+1)
}
```

**영향**:
- 이후 `RunTurn()`이 잘못된 thread에서 실행됨
- resume 시 codex 서버가 해당 thread를 모름
- state에 synthetic/real ID가 혼재되어 디버깅 불가

**수정안**:

```go
// thread
threadID := strings.TrimSpace(thread.ID)
if threadID == "" {
    if strings.EqualFold(st.Run.EngineMode, "real") {
        return "", 0, errors.New("real mode adapter returned empty thread ID")
    }
    threadID = syntheticThreadID(st.Runtime.SessionID + "-" + planID)
}

// turn
turnID := strings.TrimSpace(turn.ID)
if turnID == "" {
    if strings.EqualFold(st.Run.EngineMode, "real") {
        return "", 0, fmt.Errorf("real mode adapter returned empty turn ID at step %s", step.Step)
    }
    turnID = fmt.Sprintf("turn-%03d", i+1)
}
```

**작업량**: 10분
**테스트 추가**: real 모드 빈 ID 반환 시 에러 검증 1건

---

### Issue #2 — Turn 간 State 미저장 (심각도: MEDIUM)

**위치**: `cmd/ax/commands.go:1620-1758` (turn 루프)

**현상**: turn 성공 시 `st.AppendTurnRef()`는 메모리만 업데이트하고 `st.Save()`를 호출하지 않는다. 최종 성공 시에만 한 번 저장한다.

**시나리오**:
1. Turn 1 성공 → 메모리에만 TurnHistory 추가
2. Turn 2 성공 → 메모리에만 TurnHistory 추가
3. Turn 3 시작 → 프로세스 crash (SIGKILL)
4. 복구 시: TurnHistory에 Turn 1, 2 결과 유실. ActiveTurnID는 Turn 3이지만 실제로는 미실행

**영향**:
- resume 시 Turn 1, 2를 다시 실행 (중복 작업)
- ActiveTurnID와 TurnHistory 불일치

**수정안**:

```go
// turn 성공 후 즉시 저장
st.AppendTurnRef(core.TurnRef{...}, 100)
st.Run.ActiveTurnID = ""
if err := st.Save(base); err != nil {
    return "", 0, err
}
```

**트레이드오프**: 매 turn마다 disk I/O 발생. atomicWriteFile 사용 중이므로 안전하지만 느려질 수 있음. 대안으로 N턴마다 저장(예: 3턴)하는 방식도 가능.

**작업량**: 5분
**테스트 추가**: 중간 turn 후 state reload하여 TurnHistory 확인 1건

---

### Issue #3 — Scaffold vs Real InterruptTurn 동작 불일치 (심각도: LOW)

**위치**: `internal/codex/scaffold.go:157-158` vs `internal/codex/client.go:111-112`

**현상**:

```go
// scaffold: unknown turn → 성공 (no-op)
func (s *ScaffoldAdapter) InterruptTurn(...) (*InterruptResult, error) {
    return &InterruptResult{Interrupted: true, ...}, nil  // 항상 성공
}

// StdioClient: Interrupted=false → 에러
if !out.Interrupted {
    return nil, errors.New("interrupt was not acknowledged by app-server")
}
```

**영향**: scaffold 모드 테스트는 항상 통과하지만, real 서버에서 동일 시나리오가 실패할 수 있음.

**수정안**: scaffold에서 존재하지 않는 turn에 대해 `Interrupted: false` 반환 옵션 추가, 또는 동작 차이를 문서화.

**작업량**: 5분

---

## 4. 테스트 커버리지 평가

### 4.1 영역별 점수

| 영역 | 점수 | 비고 |
|------|------|------|
| Factory env var 조합 | 7/10 | invalid timeout(음수)/negative retry 미검증 |
| Scaffold 인터페이스 | 8/10 | 9개 메서드 기본 검증, 동시성 테스트 없음 |
| Scaffold 동시성 | 5/10 | mutex 존재하나 concurrent access 테스트 없음 |
| runPlan mock adapter E2E | 9/10 | accept/steer/reject/resume 모두 검증 |
| 에러 매핑 | 6/10 | 표준 5개 코드 검증, `-32000~-32099` 범위 미검증 |
| State 하위호환 | 8/10 | round-trip + trim 100건 검증 |
| Prompt 빌더 | 7/10 | TDD/standard 기본, empty input/special char 미검증 |
| Doctor codex 필드 | 8/10 | JSON 출력에 신규 필드 검증 |
| Recover thread 기반 전략 | 8/10 | ThreadID 유무별 분기 검증 |

### 4.2 누락된 테스트 시나리오

#### P0 (Issue 수정 시 함께 추가)

- [ ] real 모드에서 빈 thread ID 반환 시 에러 검증
- [ ] real 모드에서 빈 turn ID 반환 시 에러 검증
- [ ] turn 중간 crash 후 state reload 시 TurnHistory 일관성

#### P1

- [ ] ScaffoldAdapter concurrent access (goroutine 10개 동시 RunTurn)
- [ ] JSON-RPC 에러코드 `-32000~-32099` 범위 매핑 검증
- [ ] Factory: `AX_CODEX_TIMEOUT=-5s`, `AX_CODEX_RETRIES=-1` 방어
- [ ] InterruptTurn: scaffold에서 존재하지 않는 turn 동작 검증

#### P2

- [ ] Prompt 빌더: empty planBody, Unicode step name
- [ ] StreamTurn: 빈 이벤트 배열, completed 누락 시 정규화
- [ ] client.go: context deadline 만료 중 turn 실행 인터럽트

---

## 5. 코드 스타일 평가

### 일관성 있는 부분

- 함수 네이밍: `BuildStepPrompt`, `MapCodexError` — 기존 패턴과 일치
- JSON 태그: `json:"field_name,omitempty"` — state.go 패턴과 일치
- 테스트 네이밍: `TestFunctionName_Scenario` — Go 관례 준수
- 에러 반환: 실패 우선 기록 원칙 준수 (state 저장 → 에러 반환)

### 개선 필요한 부분

| 항목 | 현상 | 권장 |
|------|------|------|
| scaffold 에러 문자열 | `"AX_ENGINE_THREAD_NOT_FOUND: %s"` 직접 사용 | `MapCodexError()` 통합 또는 상수 분리 |
| context 무시 | scaffold에서 `ctx` 파라미터 미사용 | 주석으로 의도 명시 (`// ctx unused in scaffold mode`) |
| timeout=0 동작 | 무한 대기 허용 | 최소값 강제 또는 문서화 |

---

## 6. 수정 우선순위 요약

| 우선순위 | 이슈 | 심각도 | 작업량 | 상태 |
|----------|------|--------|--------|------|
| **P0** | Real 모드 빈 ID 폴백 차단 (thread + turn) | HIGH | 10분 | 미수정 |
| **P1** | Turn 간 state 저장 추가 | MEDIUM | 5분 | 미수정 |
| **P2** | InterruptTurn scaffold/real 동작 불일치 문서화 | LOW | 5분 | 미수정 |
| **P1** | 누락 테스트 시나리오 보강 (§4.2 P1 목록) | MEDIUM | 30분 | 미수정 |

---

## 7. 결론

### 강점

1. **DI 아키텍처**: `AppServerAdapter` 인터페이스 경계가 깔끔하여 scaffold/real 전환이 env var 하나로 가능
2. **실행 루프 설계**: decision(accept/steer/reject) → adapter 메서드 1:1 매핑이 명확
3. **에러 복구 경로**: 실패 시 state 먼저 저장 + retryable 분류 + recover hint 생성
4. **하위호환**: 기존 state.yaml 깨지지 않음 (omitempty)
5. **테스트**: mock adapter 기반 E2E가 주요 경로를 검증

### 약점

1. **Real 모드 방어 부족**: 빈 ID 폴백이 silent corruption 유발 가능
2. **Turn 간 내구성**: 중간 crash 시 TurnHistory 유실
3. **에러 체계 이원화**: scaffold 직접 문자열 vs errors.go 매핑 함수 병존
4. **동시성 테스트 부재**: mutex는 있으나 검증되지 않음

### 프로덕션 준비도

P0 수정(빈 ID 차단) 완료 시 **프로덕션 투입 가능**. P1(turn 간 저장)은 안정성 향상이지만 블로커는 아님.
