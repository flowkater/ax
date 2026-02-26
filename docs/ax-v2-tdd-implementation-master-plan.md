# ax v2 CLI 고도화 TDD 구현 계획 (1/3) — Master Plan

- plan_id: `ax-v2-tdd-master-plan-v1`
- status: `implementation-reviewed-complete`
- source: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- stack: `Go CLI`
- test_command_red: `go test -v ./...`
- test_command_green: `go test -v ./...`
- format_command: `gofmt -w $(git ls-files '*.go')`
- integration_test_command: `go test -v -tags=integration ./...`

---

## Overview
본 문서는 gap 요구사항을 실제 구현 가능한 TDD 단계로 고정한다. 구현은 **P0→P4**, 테스트는 **T1→T4**, 실행은 **Red→Green→Refactor**로 진행한다.

> 용어 구분 고정  
> - **검증 Tier**: T1~T4 (문서/테스트/릴리즈 게이트)  
> - **TDD 실행 Tier**: T0~T2 (`run --tdd` 내부 루프)

---

## Requirements Digest

### 1) Entities / Objects
- `state.yaml` (phase, context_chain, current refs, last_error, tdd progress, transitioned_at, trigger_command)
- Proposal / Plan / Discover / Verify / Archive artifacts
- TDD state (`current_tier`, `current_step`, `completed_steps`, `total_steps`)
- Worktree session (`.ax/worktrees/{proposal-id}`)
- Engine session/thread/turn + streaming event (`delta`, `completed`)
- Review(8-lens), Compound(gotcha triage/decay)
- 3-Layer Context (`Protocol`, `Task-Scoped`, `Session Memory`)

### 2) Actions / Use-cases
- `propose`, `plan --from`, `discover --party`, `run --tdd`, `quick`, `verify`, `archive`, `review`, `compound --audit`, `state`
- 사용자 제어 루프: `accept / reject / steer`
- 장애 복구 루프: crash 후 `resume`, app-server 재연결

### 3) Rules / Constraints
- 상태머신 전이 강제: `idle→discovery→proposal→planning→implementation→verification→archived`
- 불법 전이 거부 + 표준 에러코드
- `context_chain[]` 누적/자동 로드 + 영향경로 기반 결정성 유지
- 3-Layer Context 선택 로직은 동일 입력 시 동일 결과를 보장해야 함
- `run --tdd`는 T0→T1→T2 progression gate를 강제해야 함
- Tier gate: 이전 tier 통과 전 다음 tier 금지
- Quality gate: tier당 리뷰 횟수 기본 3회, 초과 시 `--force` + 사유 기록으로 최대 5회
- Archive strict: verify 없는 archive 기본 차단(`enabled`)

### 4) API / Interface Contracts
- CLI 계약: 필수 섹션/산출물/메타데이터 고정
- 내장 스킬 계약: `openspec`, `superpowers`, `tdd-plan`, `tdd-go`, `tdd-go-loop`, `interview`
- app-server RPC Must:
  - `CreateThread`, `RunTurn`, `GetThread`
  - `ResumeSession`, `ForkSession`, `RollbackTurns`, `SteerTurn`, `InterruptTurn`
- approval policy: `never | on-failure | unless-allow-listed | always`
- `tdd-go` 기본 엔진 프로파일: `gpt-5.3-spark` (사용자 오버라이드 정책 별도)

### 5) Error / Edge Cases
- `plan --from` 대상 없음
- quick 임계치 초과 시 승격
- run crash 후 재개
- app-server 단절/timeout/retry/backoff
- verify 판정 충돌(PASS/FAIL/CONDITIONAL PASS)
- archive metadata idempotency

---

## Phase/Tier Checklist

### Phase 1: Contract/Interface/DTO <!-- T1:auto -->
- [x] 상태 스키마(version, 필수 필드) 계약 테스트 작성
- [x] 상태 전이 허용/거부 테이블 테스트 작성
- [x] 상태 전이 메타필드(`transitioned_at`, `trigger_command`) 계약 테스트 작성
- [x] CLI 입력/출력 계약(`plan --from`, `state`) 테스트 작성
- [x] `propose` 필수 섹션 + `tasks.md >= 10` + `current.proposal` 연동 계약 테스트 작성
- [x] `plan --from` 산출물 필수 섹션(Phase/파일후보/테스트전략/롤백) + tasks-step ID 매핑 계약 테스트 작성
- [x] 문서 템플릿 필수 섹션(propose/plan/discover/party/interview) + 내장 스킬 registry 계약 검증 테스트 작성
- [x] 3-Layer Context(Protocol/Task/Session) 로딩 우선순위 계약 테스트 작성

### Phase 2: Core Rule/Domain Logic <!-- T2:review -->
- [x] 상태머신 전이기 구현 + 에러코드 표준화
- [x] context 영향경로 분석기 구현(`task.analyzeAffectedDirectories()`)
- [x] depth 분류기(quick/normal/deep/explore) 구현
- [x] verify 판정 엔진(PASS/FAIL/CONDITIONAL PASS) 구현
- [x] tier progression gate(T0→T1→T2) 강제 로직 구현
- [x] 리뷰 횟수 제한(quality gate 기본 3회, force 시 최대 5회) 로직 구현
- [x] TDD Tier(T0/T1/T2) 진행 상태(`state.yaml.tdd.*`) 갱신 로직 구현
- [x] approval policy 결정 엔진(`never/on-failure/unless-allow-listed/always`) 구현

### Phase 3: Application Orchestration <!-- T2:review -->
- [x] `run --tdd` step 오케스트레이터 구현
- [x] step↔turn 1:1 매핑 저장/검증 구현
- [x] run 입력 plan 형식 검증 + 시작/종료/실패 아티팩트 로그 생성 구현
- [x] `accept/reject/steer` + phase별 approval policy 인터랙션 루프 구현
- [x] run crash 복구(`--resume`) 구현
- [x] quick 실행 후 정리/승격 로직 구현
- [x] verify 미충족 항목 + 다음 액션 자동생성 + verify diff 출력 경로 구현

### Phase 4: Data/Integration <!-- T3:auto -->
- [x] worktree 생성/머지/정리 및 `--no-worktree` 구현
- [x] archive strict + metadata schema/idempotency 구현
- [x] compound triage(FixCandidate/Document/Noise) + Osmani filter + gotcha schema/decay/audit 구현
- [x] app-server stdio JSON-RPC client + 기본 3메서드(CreateThread/RunTurn/GetThread) 구현
- [x] app-server RPC lifecycle 메서드(`Resume/Fork/Rollback/Steer/Interrupt`) 확장 구현
- [x] streaming 수신/완료 집계 + retry/backoff 구현
- [x] compaction trigger 구현
- [x] runtime hardening 최종 보강(cluster/node/session metadata, shared/worktree session-isolated worktree, runtime journal + checkpoint/resume 복구)

### Phase 5: Delivery Surface <!-- T4:auto -->
- [x] CLI 명령별 사용자 메시지/에러 분류 출력 통일
- [x] review 8-lens 산출물/심각도 포맷 구현
- [x] `docs/mvp-walkthrough.md` 실행 절차 최신화

### Phase 6: E2E/Regression/Performance <!-- T4:auto -->
- [x] 필수 수용테스트 26개 자동화 완료
- [x] `go test ./...`, `go build ./...` 상시 통과
- [x] propose→plan→run→verify→archive E2E 1회 통과

---

## Tier Checklist Review (문서 반영 완료)
- [x] T1 체크리스트를 gap §4/§11/§14/§16 기준으로 재검토하고 propose/plan 계약 누락 반영
- [x] T2 체크리스트를 run/tdd/approval/quality gate + verify 후속액션 요구사항 기준으로 재검토
- [x] T3 체크리스트를 worktree/archive/streaming/compaction + app-server 기본 3메서드 기준으로 재검토
- [x] T4 체크리스트를 review/delivery/regression 기준으로 재검토
- [x] `docs/ax-v2-cli-requirements-gap.md` 정합성 반영 준비 포인트(§4, §10, §11, §12, §14, §16) 표기 완료

---

## Tier Item Count
| Tier | Items |
|---|---:|
| T1 | 8 |
| T2 | 14 |
| T3 | 6 |
| T4 | 6 |
| **Total** | **34** |

---

## Completion Log
| Date | Phase | Tier | Result | Evidence |
|---|---|---|---|---|
| 2026-02-26 | Planning | T1-T4 | Draft Created | this doc |
| 2026-02-26 | Documentation Review | T1-T4 | Checklist Reviewed & Updated | Tier Checklist Review section |
| 2026-02-26 | Worker-1 Review | T1~T4 | Critical/Major revalidation + fixes applied | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS, archive allow-unverified override/idempotent + proposal path 입력 + archived 이후 신규 proposal 회귀 수정 |
| 2026-02-26 | Worker-3 Review | T2~T3 | Tier gate/approval/verify-diff/compound/compaction/worktree deterministic 보강 + 재검증 | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS |
| 2026-02-26 | Worker-2 Final Revalidation | T2~T3 | approval/verify-diff/compound/compaction/worktree/no-side-effect 및 회귀 최종 정리 | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS |
| 2026-02-26 | Worker-1 Final Verification | T2~T3 | tier gate/approval/verify-diff/compound/compaction/worktree determinism/no-side-effect 재확인 | `go test ./...` PASS, `go build ./...` PASS, CLI E2E PASS |
| 2026-02-26 | Worker-2 Runtime Hardening Final | T3 | runtime metadata/journal/checkpoint + shared concurrency semantics + load/kill 안정성 재검증 | `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/summary.json`) |
| 2026-02-26 | Worker-3 Runtime Hardening Final | T2~T3 | runtime metadata(cluster/node/session), runtime checkpoint 복구, shared session worktree 격리, recover auto/resume 정책 보강 | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/tmp/ax-worker3-evidence-20260226-210011/runtime-summary.json`) |
| 2026-02-26 | Worker-1 Runtime Hardening Revalidation | T2~T3 | cluster/node/session metadata 플래그·환경변수 반영, shared/worktree 세션 격리, checkpoint 복구 보강, journal/doctor 진단 정합성 재검증 | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, real binary soak/load/kill PASS (`/tmp/ax-prod-soak-20260226-120148/summary.json`) |
| 2026-02-26 | Worker-1 Runtime/Recover/Doctor Smoke Refresh | T2~T3 | 실제 `ax` 바이너리로 runtime-mode(shared)/doctor runtime JSON/recover auto/runs→archive 재검증 | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, smoke `state→propose→plan→run --tdd --loop→doctor runtime --json→recover --strategy auto→discover --party→review→compound --audit→verify→archive→state --json` PASS (`/tmp/ax-worker1-revalidation-20260226-121531-GiZxMm/runtime-smoke-summary.json`) |
| 2026-02-26 | Worker-3 Final Runtime Gate | T2~T4 | 최종 게이트(go/race/vet/build + smoke/recover/doctor/runtime-mode) 실바이너리 재검증 | `go test ./...` PASS, `go test -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, shared/worktree runtime-mode + recover auto(rerun/resume) + doctor runtime JSON + E2E smoke PASS (`/tmp/ax-worker3-final-20260226-211549/runtime-smoke-summary.json`) |
| 2026-02-26 | Leader Final Production Gate | T2~T4 | 최신 코드 기준 final gate 재실행(quality + full runtime smoke + soak/load/kill) | `go test -count=1 ./...` PASS, `go test -count=1 -race ./...` PASS, `go vet ./...` PASS, `go build ./...` PASS, full smoke PASS (`/tmp/ax-live-verify-debug3-iYIOCG/live-runtime-summary.json`), soak/load/kill PASS (`/tmp/ax-live-soak-final-CVRKO7/summary.json`) |

---

## Requirements Alignment Readiness (Gap Sync Prep)
- 기준 문서: `docs/ax-v2-cli-requirements-gap.md` (§4, §10, §11, §12, §14, §16)
- 상태: **Complete (remaining code gaps revalidated)**
- 준비 체크리스트:
  - [x] T1~T4 구현 체크리스트와 gap Must 요구사항 매핑 정리
  - [x] Policy/Decision Lock 5항목과 충돌 항목 없음 확인
  - [x] 2/3(Test Matrix), 3/3(Execution) 문서로 넘길 Tier 게이트 기준 고정
  - [x] 구현 실행 증거(테스트/빌드/E2E 로그) 주입
  - [x] runtime hardening 실증 근거(soak/load/kill + race/vet/build + runtime-mode/recover/doctor smoke) 최신화 (`/tmp/ax-worker3-final-20260226-211549/runtime-smoke-summary.json`)

---

## Gap 1:1 Alignment (Revalidated)

| Gap 기준 | 본 문서 반영 위치 | Tier |
|---|---|---|
| §4.1 propose Must(필수 섹션, tasks>=10, state 연동) | Phase 1 계약 테스트 | T1 |
| §4.2 plan Must(필수 섹션, tasks-step 매핑) | Phase 1 계약 테스트 | T1 |
| §4.3/§4.4 run·verify Must(로그/판정근거/후속액션) | Phase 3 구현 체크리스트 | T2 |
| §4.9 app-server 최소 3메서드 + JSON-RPC client | Phase 4 구현 체크리스트 | T3 |
| §10.1 상태머신 + 전이 메타필드 | Entities, Phase 1/2 체크리스트 | T1/T2 |
| §10.2 context_chain 누적/자동 로드 | Rules/Constraints, Phase 2 | T2 |
| §10.3 3-Layer Dynamic Context | Entities, Phase 1 계약 테스트 | T1 |
| §10.4 compound triage/decay + Osmani + gotcha schema | Phase 4 | T3 |
| §10.5 review 8-lens | Entities, Phase 5 | T4 |
| §10.6 app-server lifecycle 5메서드 | API/Contracts, Phase 4 | T3 |
| §10.7 streaming/approval/compaction | API/Contracts, Phase 2/3/4 | T2/T3 |
| §10.8 adaptive depth 우선순위 | Rules/Constraints, Phase 2 | T2 |
| §11 + §16 수용테스트 1~26 | Phase 6 + 2/3 Test Matrix 연동 | T1~T4 |
| §14 누락 요구사항(run --tdd, worktree, party/interview, reject/steer, quality gate, built-in skills) | Actions/Use-cases, Phase 1~4 | T1~T3 |

## 결론
- 1/3 Master Plan은 gap 문서 기준 필수 요구사항과 Tier(T1~T4) 책임을 재정렬했다.
- 기존 누락(전이 메타필드, 3-Layer Context, T0~T2 명시, Osmani/gotcha schema, lifecycle 5메서드)을 체크리스트에 반영했다.
- 증거 주입은 완료되었고, 본 문서 범위 기준 미해결 체크박스/잔여 런타임 갭은 없다.

---

## Notes

### Business Rules
- 동일 입력 → 동일 상태/산출물 구조 유지
- 로그 필수 필드: `step, phase, thread_id, turn_id, error_code`
- verify 결과는 근거(테스트/빌드/AC 체크)를 반드시 포함

### Out of Scope
- GUI/daemon/ws
- 고급 멀티에이전트 자동 분산 스케줄러
- 에디터 확장

### Decision Lock (2026-02-26)
1. archive strict 기본값: **차단(enabled)**  
   - 예외는 `--allow-unverified-archive` 플래그로만 허용
2. quick 승격 임계치: **하이브리드 규칙**  
   - 변경 파일 수 `> 5` 또는 cross-module 변경 또는 core 경로(`state/codex/run`) 변경 시 승격
3. app-server timeout/retry: **30초 + 2회 재시도**  
   - backoff: 1s, 2s + jitter
4. depth 우선순위: **`--depth` 절대 우선**  
   - 미지정 시 자동 분석 사용, 로그에 `depth_source=user|auto` 기록
5. quality gate 제한: **기본 3회**  
   - 초과 시 차단, `--force` + 사유 기록 시 5회까지 허용
