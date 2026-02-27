# AX CodexApp Server 연동 프로덕션 준비도 종합 리뷰 (2026-02-27)

## 1) 목적
현재 `ax`의 CodexApp Server 연동이 실제 운영(프로덕션)에서 안전하게 사용 가능한지 코드/테스트/문서 관점으로 종합 점검한다.

## 2) 리뷰 범위
- 코드: `cmd/ax/commands.go`, `cmd/ax/tui_interactive.go`, `internal/codex/*`, `internal/core/*`
- 테스트: `cmd/ax/commands_test.go`, `internal/codex/*_test.go`, `internal/core/*_test.go`
- 문서: `docs/ax-v2-codex-integration-plan.md`, `docs/ax-v2-codex-integration-review.md`

## 3) 최종 판정
- **현재 판정: REQUEST CHANGES (프로덕션 즉시 배포 비권장)**
- 사유: 운영 안정성/보안/제어면에서 **blocking 이슈**가 남아 있음.

## 4) 핵심 이슈 (우선순위)

### HIGH (배포 전 필수)
1. **`AX_CODEX_MODE` fail-open 동작**
   - 미인식 값도 scaffold로 조용히 폴백되어 real 연동 실패를 숨길 수 있음.
   - 근거: `internal/codex/factory.go:36-40, 67-69`

2. **run 전체에서 단일 timeout context 재사용**
   - 긴 실행에서 후속 RPC가 즉시 `DeadlineExceeded`로 실패할 수 있음.
   - 근거: `cmd/ax/commands.go:1392-1396, 1401-1403, 1680-1707`

3. **TUI 위험 액션의 실제 엔진 제어 미연결**
   - interrupt/fork/rollback/steer가 상태/저널 기록 중심이며 실제 제어와 괴리 가능.
   - 근거: `cmd/ax/commands.go:808-835`, `cmd/ax/tui_interactive.go:30-37`

4. **경로 경계 검증 부재(외부 경로 접근 위험)**
   - proposal/plan 입력이 base 하위로 강제되지 않음.
   - 근거: `cmd/ax/commands.go:3212-3227, 3272-3277, 2203`

### MEDIUM
1. **reject 경로에서 Interrupt 실패 무시**
   - `_, _ = engine.InterruptTurn(...)`로 오류가 유실됨.
   - 근거: `cmd/ax/commands.go:1582`

2. **thread 생성/재개 실패 시 관측성 기록 불균일**
   - 일부 실패 경로에서 fail log/checkpoint/journal 누락.
   - 근거: `cmd/ax/commands.go:1405-1420`

3. **권한/보안 하드닝 미흡**
   - 상태/로그 파일 권한이 `0644` 중심.
   - 근거: `internal/core/state.go:314`, `cmd/ax/commands.go:2968, 3111`

4. **테스트 공백 (실패/복구 계열)**
   - resume 실패, turn RPC 실패, retry/backoff, invalid/no-response 분기, interrupt 미승인 분기 보강 필요.

## 5) 테스트 리뷰 요약
- 정상 플로우 검증은 강한 편(스레드 생성/턴 실행/스트리밍/state 저장/recover 기본 경로).
- 실패/복구/네트워크 변동성 시나리오가 상대적으로 부족.
- 참고 커버리지(리뷰 시점):
  - `internal/codex`: 76.1%
  - `internal/core`: 72.7%
  - `cmd/ax`: 71.7%

## 6) 문서-코드 정합성 이슈
1. `docs/ax-v2-codex-integration-review.md`의 일부 P0/P1 상태가 코드 최신 상태와 불일치.
2. `docs/ax-v2-codex-integration-plan.md`에서 fork/rollback 완료 체크와 실제 recover 전략(`auto|resume|rerun`) 사이 불일치.

## 7) 이번 변경에서 반영된 개선
아래 항목은 코드에 반영됨(현재 워킹트리 변경 포함):
1. **real 모드 빈 thread/turn ID 하드 실패 처리**
   - `cmd/ax/commands.go`
2. **turn 단위 state 저장/체크포인트 강화**
   - `cmd/ax/commands.go`
3. **scaffold unknown turn interrupt 실패 처리 + 테스트 추가**
   - `internal/codex/scaffold.go`, `internal/codex/scaffold_test.go`

## 8) 프로덕션 진입 최소 조건 (1~2주)
1. `AX_CODEX_MODE` **fail-closed** 전환(미인식 값 즉시 에러).
2. RPC별 독립 timeout context 적용 + 회귀 테스트.
3. TUI 위험 액션을 실제 엔진 제어와 정합되게 연결.
4. 경로 canonicalization 및 base 경계 강제.
5. 실패/복구 테스트 세트 확장(Resume 실패, Run 실패, Retry/Backoff, Invalid response).
6. 운영 문서(runbook/SLO/알람/롤백 전략) 최신화.

## 9) 결론
현 단계는 **“기능 구현은 진행됨 + 운영 하드닝 미완료”** 상태다.  
즉시 대규모 프로덕션 롤아웃보다는, 상기 HIGH 이슈 해소 후 제한적 트래픽/단계적 롤아웃을 권장한다.
