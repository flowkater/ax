# ax v2 하드닝 + TUI 구현 요구사항

- 작성일: 2026-02-27
- 대상: `ax` CLI (Go)
- 목적: 현재 베타 퀄리티를 프로덕션 준비 수준으로 끌어올리고, TUI 요구사항을 명확화한다.

---

## 1. 배경

현재 `ax`는 다음을 충족한다.
- propose → plan → run → verify → archive E2E 동작
- `go test ./...`, `go build ./...` 통과
- 상태/로그/아카이브 메타데이터 기록

하지만 운영 신뢰성과 사용자 경험을 위해 하드닝이 필요하다.

---

## 2. 목표 상태

1) **신뢰 가능한 verify 판정**
- PASS / FAIL / CONDITIONAL PASS 판정이 재현 가능하고 근거가 명확해야 한다.

2) **실패 복구 가능한 실행 엔진**
- run 실패 시 resume/retry 경로가 명시적으로 동작해야 한다.

3) **병렬 실행 안정성**
- lock/state 경쟁 상황에서도 데이터 무결성 보장.

4) **운영 관찰 가능성 강화**
- 장애 원인, 재시도 이력, phase 전이를 사람이 빠르게 추적 가능.

5) **TUI 도입**
- CLI를 보조하는 실시간 상태/제어 UI를 제공하되, CLI를 우선 SoT로 유지.

---

## 3. 하드닝 요구사항 (CLI)

## 3.1 Verify 판정 신뢰성

### Must
- verify 결과에 아래 필드 필수:
  - `verdict`: PASS | FAIL | CONDITIONAL_PASS
  - `evidence.tests`
  - `evidence.build`
  - `evidence.criteria`
  - `failed_checks[]`
- 동일 입력(동일 proposal/plan/artifact)에서 verify 결과 결정성이 유지되어야 한다.
- verify 보고서(`verify.md`)와 기계 파싱 파일(`verify.json`)을 동시에 생성한다.

### Should
- 직전 verify 대비 diff 요약(`regressed`, `improved`) 제공.

---

## 3.2 Run 복구/재시도

### Must
- run 실패 시 `state.yaml`에 아래 저장:
  - `last_failed_step`
  - `error_code`
  - `error_summary`
  - `recover_hint`
- `ax run --resume` 지원: 마지막 실패 step부터 재개.
- retry 정책:
  - 최대 재시도 횟수
  - backoff 전략
  - 초과 시 `blocked` 전이

### Should
- `ax run --retry <n>` 수동 override 지원.

---

## 3.3 Lock/State 무결성

### Must
- state 갱신은 atomic write(임시파일+rename)로 처리.
- lock 획득 실패 시 즉시 종료 대신 timeout 내 재시도.
- stale lock 감지 및 복구 규칙 포함.

### Should
- lock debug 명령(`ax state --locks`) 제공.

---

## 3.4 Error Taxonomy

### Must
- 에러코드 표준화:
  - `AX_INPUT_*`
  - `AX_STATE_*`
  - `AX_ENGINE_*`
  - `AX_VERIFY_*`
  - `AX_ARCHIVE_*`
- 사용자 메시지와 내부 상세(로그) 분리.

### Should
- `docs/errors.md` 생성 및 코드↔원인↔복구 방법 매핑.

---

## 3.5 Observability

### Must
- 실행 단위 로그 필수 필드:
  - `session_id`, `proposal_id`, `plan_id`, `phase`, `step`, `thread_id`, `turn_id`, `status`, `duration_ms`
- `runtime-journal.jsonl` 누적 + 최근 요약 명령 제공.

### Should
- `ax state --json` 제공(자동화 연동용).

---

## 3.6 App-server 연동 하드닝

### Must
- 현재 최소 메서드(CreateThread/RunTurn/GetThread) 신뢰성 강화:
  - timeout
  - transport error mapping
  - retry-safe 동작
- 세션 라이프사이클 확장:
  - ResumeSession
  - ForkSession
  - RollbackTurns
  - SteerTurn
  - InterruptTurn

### Should
- turn streaming(delta/completed) 이벤트 소비를 `run`과 연동.

---

## 4. TUI 요구사항

## 4.1 범위
- 목표: CLI 보조 인터페이스
- 원칙: **CLI가 진실의 원천(SoT)**, TUI는 상태 조회/제어 계층

## 4.2 핵심 화면

### Screen A: Dashboard
- 현재 phase
- 진행률
- 현재 proposal/plan
- 최근 실패/경고
- 최근 5개 실행 로그

### Screen B: Runs
- 실행 목록(시간, 상태, duration)
- 선택 시 step별 상세/에러/복구 힌트
- retry/resume 액션 트리거

### Screen C: Verify
- 최근 verify verdict
- failed checks
- criteria 충족률
- compare(직전 대비)

### Screen D: Archive
- archive 목록
- metadata 요약
- artifact 열람

### Screen E: Engine
- thread/turn 상태
- active session
- interrupt/steer/fork/rollback 제어

---

## 4.3 TUI 동작 요구사항

### Must
- read-only 기본, 위험 액션은 확인 프롬프트 필수.
- refresh interval configurable (기본 1s).
- 키 바인딩 도움말 항상 표시.
- 비정상 종료 후 재진입 시 state 손상 없어야 함.

### Should
- 텍스트 검색/filter
- severity 색상 강조

---

## 4.4 TUI 기술 요구사항

### Must
- Go 기반 TUI 라이브러리(Bubble Tea 계열 권장) 사용.
- 내부 데이터는 CLI와 동일 파일/상태 스키마 사용.
- TUI 액션은 내부적으로 동일 core service를 호출(중복 로직 금지).

### Should
- headless 모드에서도 동일 기능 일부 제공(`ax tui --snapshot`).

---

## 5. 테스트 요구사항

## 5.1 하드닝 테스트
- verify 결정성 테스트
- run resume/retry 테스트
- lock 경합 테스트(동시 run)
- stale lock 복구 테스트
- 에러코드 매핑 테스트
- app-server timeout/interrupt/rollback 테스트

## 5.2 TUI 테스트
- 모델(state->view) 매핑 테스트
- 키 이벤트 처리 테스트
- 위험 액션 confirmation 테스트
- 스냅샷 테스트(핵심 화면)

## 5.3 E2E
- state→propose→plan→run→verify→archive
- 실패 유도 후 resume까지 포함한 E2E 1개 이상

---

## 6. 완료 기준 (DoD)

다음 전부 충족 시 완료:
1. 하드닝 Must 항목 구현
2. TUI Screen A~E 최소 구현
3. 테스트 통과:
   - `go test ./...`
   - `go build ./...`
4. 실사용 스모크:
   - CLI E2E 성공
   - TUI 진입/조회/핵심 액션 성공
5. 문서 업데이트:
   - walkthrough
   - error mapping
   - 운영 가이드

---

## 7. 우선순위

### P0 (즉시)
- verify 판정 신뢰성
- run resume/retry
- lock/state atomic/stale recovery
- 에러코드 표준화

### P1
- app-server lifecycle 확장
- observability 강화
- TUI Dashboard/Runs/Verify

### P2
- TUI Archive/Engine 제어 고도화
- compare/replay/analytics

---

## 8. 비범위 (이번 문서 기준)
- GUI(웹/데스크톱 앱) 전체 제품화
- daemon/WS 전체 아키텍처 전환
- 멀티노드 분산 스케줄러
