# ax 실행 검증 + Codex app-server 연동 품질 리뷰

- 작성일: 2026-02-27
- 대상 브랜치: `develop`
- 범위: 실제 실행 스모크 + app-server real mode 연동 품질 점검

---

## 1) 실행 검증 결과

### 1.1 빌드/테스트
- `go test ./...` ✅ 통과
- `go build ./...` ✅ 통과

### 1.2 CLI E2E 스모크
실행 순서:
- `ax state`
- `ax propose "v2 quality review scenario"`
- `ax plan --from <proposal-id>`
- `ax run --plan <plan-file> --tdd --tier T0`
- `ax verify --proposal <proposal-id> --tests pass --build pass --ac pass`
- `ax archive --proposal <proposal-id>`

결과:
- `.ax/state.yaml` 생성/갱신 ✅
- `.ax/runs/*` 실행 아티팩트 생성 ✅
- `.ax/logs/*` 시작/종료 로그 생성 ✅
- `.ax/archive/*/archive-metadata.yaml` 생성 ✅

### 1.3 TUI 스모크
- `ax tui --snapshot --format md` ✅ 정상 출력
- `ax tui --action interrupt` (확인 없이) → `--confirm` 요구 에러 ✅

판정:
- CLI/TUI 기본 동작 품질은 **베타+ 수준**

---

## 2) Codex app-server real mode 품질 점검

## 2.1 점검 방법
- `AX_CODEX_MODE=real` 설정 후 `ax run` 실행
- bin/args 조합을 바꿔 실서버 연결 동작 확인

## 2.2 발견 이슈

### 이슈 A: JSON-RPC 메서드명 불일치 (치명)
현재 구현 호출:
- `CreateThread`
- `RunTurn`
- `GetThread`

실제 app-server 기대 메서드:
- `thread/start`
- `turn/start`
- `thread/read`
- (그 외 `thread/resume`, `thread/fork`, `thread/rollback`, `turn/interrupt`, `review/start` 등)

실제 오류 메시지:
- `Invalid request: unknown variant 'CreateThread' ...`

영향:
- **real mode 실사용 불가**

### 이슈 B: real mode 기본 실행 경로 취약
- `AX_CODEX_BIN` 미지정 환경에서 `codex` 탐색 실패 가능
- app-server용 기본 args 정책이 불명확

영향:
- 환경별 실행 편차 발생

---

## 3) 품질 판정

- Scaffold mode: ✅ 통과
- Real app-server mode: ❌ 미통과

최종 판정:
- 현재 연동은 "스캐폴드 기반 개발"에는 충분하나,
- "실제 codex app-server" 연동 품질은 **프로덕션 기준 미달**.

---

## 4) 보강 요구사항 (우선순위)

## P0 (즉시)
1. JSON-RPC 메서드 매핑 전면 수정
   - `CreateThread` -> `thread/start`
   - `RunTurn` -> `turn/start`
   - `GetThread` -> `thread/read`
   - lifecycle/제어 메서드도 app-server 스펙 네이밍으로 통일

2. real mode 실행 기본값 고정
   - 기본 bin 탐색/검증 로직 강화
   - 기본 args에 `app-server` 명시

3. 실연동 통합 테스트 추가
   - 최소 1회 `thread/start -> turn/start -> thread/read` 검증

## P1
4. streaming(delta/completed) 이벤트를 run/TDD step에 연결
5. timeout/retry/error taxonomy를 app-server 응답 기준으로 정교화

---

## 5) 다음 액션

1) P0 수정 브랜치 생성
2) 메서드 매핑 교정 + 통합 테스트 작성 (TDD)
3) real mode 스모크 재검증
4) 결과를 본 문서에 재기록(FAIL -> PASS 전환 근거 포함)
