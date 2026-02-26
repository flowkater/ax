# ax v2 CLI 요구사항 정리서 (Gap 중심)

- 작성일: 2026-02-26
- 대상 범위: **CLI only**
- 목적: 구현 계획 이전에, 현재 부족한 점과 반드시 구현해야 할 요구사항을 명확히 고정

---

## 1) 배경 / 문제 정의

현재 ax v2 CLI는 명령어 골격과 파일 생성 플로우는 존재하지만, 실제로 "바로 개발에 투입 가능한 품질"에는 미달한다.
핵심 문제는 다음 3가지다.

1. **문서 품질 부족**: propose/plan/verify 산출물이 템플릿 수준(TBD, pending)이라 실행 가능성이 낮음
2. **실행 품질 부족**: run/discover/quick이 실전 운영(실패 복구, 상태 전이, 근거 기록) 기준 미흡
3. **엔진 통합 미완**: codex app-server 연동이 계약/스텁 중심이고 Thread/Turn 실전 제어가 부족

---

## 2) 현재 상태 진단 (As-Is)

### 2.1 동작하는 것
- CLI 명령 노출(8개)
- `.ax` 기본 디렉토리 생성
- propose -> plan -> verify -> archive 플로우 자체는 실행 가능
- 기본 테스트/빌드 통과

### 2.2 부족한 것
- 문서 산출물의 정보 밀도/정확성/검증가능성 부족
- 상태 관리(`state.yaml`)의 운영 정보 부족(phase, 진행률, 최근 실패 등)
- run/discover/quick의 실동작 정의가 약함
- app-server 통합 레벨이 낮아 실제 오케스트레이션 엔진으로 쓰기 어려움

---

## 3) 목표 상태 (To-Be)

ax v2 CLI는 다음을 만족해야 한다.

1. **Spec-First 실행 가능성**: proposal/plan/tasks가 곧 구현 체크리스트로 사용 가능
2. **검증 가능한 verify**: PASS/FAIL 판정에 근거(테스트/빌드/시나리오)가 포함
3. **운영 가능한 quick/run/discover**: 상태 전이, 로그, 실패 복구 정책 포함
4. **최소 실사용 가능한 app-server 연동**: Thread/Turn 기본 제어 및 에러 처리
5. **재현성**: 동일 입력이면 동일 구조 산출물/상태를 생성

---

## 4) 기능별 요구사항 (Must/Should)

## 4.1 `propose`

### Must
- proposal 생성 시 아래 필수 섹션 포함:
  - Problem / Goal
  - Scope / Out of Scope
  - Acceptance Criteria
  - Risks / Assumptions
- `tasks.md`는 최소 10개 이상의 실행 가능한 체크 항목으로 자동 분해
- proposal ID와 state 연동 (`current.proposal`)

### Should
- proposal 생성 시 입력 문장으로부터 도메인 키워드 자동 추출
- 관련 디렉토리 후보(영향 범위) 자동 제안

## 4.2 `plan`

### Must
- `--from` proposal 기반으로 생성
- plan에 다음 필수 포함:
  - 구현 단계(Phase)
  - 파일/모듈 변경 후보
  - 테스트 전략
  - 롤백/리스크 대응
- `tasks.md`와 plan step 간 매핑 ID 유지

### Should
- 난이도(quick/normal/deep) 추정 필드 포함

## 4.3 `run`

### Must
- 입력 plan 검증(존재/형식)
- 실행 시작/종료/실패 로그 아티팩트 생성
- `state.yaml` phase 전이 반영(implementation)
- 실패 시 재시도 가능 상태 저장(마지막 실패 단계, 원인)

### Should
- `--dry-run` 지원
- 실행 단위(step) 재개(`resume`) 지원

## 4.4 `verify`

### Must
- verify 결과에 **판정** 포함: PASS / FAIL / CONDITIONAL PASS
- 판정 근거 필수:
  - 테스트 실행 결과
  - 빌드 결과
  - acceptance criteria 충족 여부 체크
- 미충족 항목 리스트업 + 다음 액션 자동 생성

### Should
- 이전 verify 대비 diff 제공

## 4.5 `archive`

### Must
- archive 이동 시 `archive-metadata.yaml` 생성:
  - proposal_id
  - archived_at
  - final_status
  - artifact_paths
  - verification_result
- archive 전 verify 존재 여부 점검(없으면 경고 또는 차단 옵션)

### Should
- archive index 자동 업데이트

## 4.6 `discover`

### Must
- discovery 문서 필수 섹션:
  - 문제 재정의
  - 옵션 A/B/C
  - 트레이드오프
  - 리스크
  - 추천안
- state에 discovery phase 기록

### Should
- party mode 관점 필드(Architect/User/QA)

## 4.7 `quick`

### Must
- quick 수행 시 plan 없이 실행 가능하되, 최소 로그/결과 아티팩트 남김
- quick 종료 후 state 복원/정리
- quick 실패 시 standard flow로 승격 안내

### Should
- quick에서 변경 파일 수 임계치 초과 시 자동 중단 후 plan 유도

## 4.8 `state`

### Must
- 현재 phase, current proposal/plan, 최근 실행 결과, 마지막 오류 표시
- 사람이 읽기 쉬운 출력 + 기계 파싱 가능한 파일 유지

### Should
- 진행률(%) 계산 로직 제공

## 4.9 Codex app-server 연동

### Must
- stdio JSON-RPC client 최소 구현
- 최소 메서드 정상 동작:
  - CreateThread
  - RunTurn
  - GetThread
- timeout/error 매핑 정책
- mock 기반 단위 테스트

### Should
- turn streaming event 수신 구조(delta/completed)
- retry/backoff 전략

---

## 5) 비기능 요구사항

### 품질
- `go test ./...` 항상 통과
- `go build ./...` 항상 통과
- 핵심 명령 스모크 테스트 통과

### 운영
- 실패 원인/재시도 지점이 로그에서 명확히 보여야 함
- 문서 산출물은 사람이 즉시 읽고 실행 가능해야 함

### 일관성
- 모든 산출물 템플릿에 공통 메타데이터(id, created_at, status)
- 파일명/디렉토리 규칙 고정

---

## 6) 완료 기준 (DoD)

다음 전부 만족 시 "완료"로 본다.

1. A~E 요구사항 구현 완료
2. propose->plan->run->verify->archive E2E 1회 성공
3. verify에 PASS/FAIL 판정 및 근거 포함
4. archive-metadata.yaml 생성 확인
5. `go test ./...` + `go build ./...` 통과
6. 문서(`docs/mvp-walkthrough.md`) 최신 반영

---

## 7) Out of Scope (이번 범위 제외)

- GUI/daemon/ws
- 고급 멀티에이전트 자동 분산 스케줄러
- 완전한 장기 메모리 최적화
- 에디터 확장 통합

---

## 8) 의사결정 필요 항목

1. verify 없는 archive를 기본 차단할지 (strict mode)
2. quick의 자동 승격 임계치(파일 수/리스크 기준)
3. app-server timeout 기본값 및 retry 횟수
4. 상태 파일 포맷(JSON vs YAML 고정)

---

## 9) 다음 단계

이 문서 확정 후, 구현 계획서는 아래 형식으로 작성한다.
- API/CLI 스펙
- 모듈 구조
- 테스트 케이스 목록
- 단계별(Phase) 체크리스트
- 예상 소요 시간

---

## 10) Opus 병렬 리뷰 반영 (필수 추가 요구사항)

아래 항목은 v2 원기획 대비 누락으로 판단되어 **Must**로 승격한다.

### 10.1 상태 머신 요구사항
- `state.yaml`은 아래 유한 상태를 명시적으로 관리해야 한다.
  - `idle -> discovery -> proposal -> planning -> implementation -> verification -> archived`
- 유효하지 않은 상태 전이는 거부해야 하며, 에러 메시지를 표준화한다.
- 각 전이마다 `transitioned_at`, `trigger_command`를 기록해야 한다.

### 10.2 Context Chain 요구사항
- `discover/propose/plan/run/verify`가 생성한 산출물 경로를 `state.yaml.context_chain[]`에 누적한다.
- 후속 명령은 context_chain을 입력 컨텍스트로 자동 로드해야 한다.

### 10.3 3-Layer Dynamic Context 요구사항
- Layer 1: Protocol (루트, 항상 로드, 길이 제한)
- Layer 2: Task-Scoped (영향 디렉토리 기반 선택 로드)
- Layer 3: Session Memory (gotcha/session 히스토리)
- 컨텍스트 선택 로직은 "영향 경로 기반"으로 재현 가능해야 한다.

### 10.4 Compound/Triage/Decay 요구사항
- `ax compound`를 정식 범위에 포함한다.
- triage 분류: `FixCandidate | Document | Noise`
- Osmani 필터 적용: 코드에서 발견 가능한 정보는 Noise로 폐기.
- gotcha는 `id, text, added, last_relevant, decay_after(기본 90d), fix_candidate` 필드를 가져야 한다.
- `ax compound --audit`는 decay 대상/아카이브 후보를 보고해야 한다.

### 10.5 Review 요구사항
- `ax review`를 정식 범위에 포함한다.
- 최소 8-lens 리뷰 결과(심각도 포함)를 산출물로 남겨야 한다.

### 10.6 Engine Session Lifecycle 요구사항
- app-server 연동은 최소 메서드(3개)에서 확장되어야 한다.
- Must 메서드:
  - `ResumeSession`
  - `ForkSession`
  - `RollbackTurns`
  - `SteerTurn`
  - `InterruptTurn`
- Thread/Turn 영속성과 제어 가능성을 검증 테스트로 보장해야 한다.

### 10.7 Streaming/Approval/Compaction 요구사항
- streaming event(`delta`, `completed`) 수신은 Should가 아니라 Must.
- phase별 approval 정책(`never`, `on-failure`, `unless-allow-listed`, `always`) 필요.
- 컨텍스트 임계치 도달 시 thread compaction 정책을 명시해야 한다.

### 10.8 Adaptive Depth 요구사항 승격
- quick/normal/deep/explore는 메타 정보가 아니라 **실제 라우팅 제어 로직**이어야 한다.
- 사용자 override(`--depth`)와 자동 분석 충돌 시 우선순위 규칙을 명시해야 한다.

---

## 11) 수용 테스트(Acceptance) 필수 목록 추가

다음 테스트가 없으면 완료로 간주하지 않는다.

1. TDD 루프 계약 테스트 (Red->Green->Refactor 순서 강제)
2. tier progression gate 테스트
3. TDD step <-> Turn 1:1 매핑 테스트
4. reject/steer 왕복 테스트
5. 8-lens 리뷰 완전성 테스트
6. verify 판정 결정성 테스트(PASS/FAIL/CONDITIONAL PASS)
7. `plan --from` 실패 경로 테스트(존재하지 않는 proposal)
8. `quick` 임계치 초과 시 표준 플로우 승격 테스트
9. `archive` strict mode 테스트(verify 없는 경우)
10. run crash 후 resume 테스트
11. app-server 프로세스 단절 복구 테스트
12. archive completeness/metadata schema/idempotency 테스트
13. state.yaml round-trip/파싱 테스트
14. observability 테스트(실행로그/에러로그 필수 필드)

---

## 12) 운영 요구사항 (Ops Readiness)

- 실패 시 사용자/운영자가 즉시 파악 가능한 에러 분류 코드 필요
- 실행 로그에는 `step`, `phase`, `thread_id`, `turn_id`, `error_code`를 남겨야 함
- 중단/재시작 후에도 `state.yaml` + context_chain으로 복구 가능해야 함
- archive는 감사(audit) 가능한 메타데이터를 강제해야 함

---

## 13) 우선순위 재정렬 (즉시 구현 기준)

P0 (필수)
- 상태머신 + context_chain
- run/discover/quick 실동작 + 복구 로직
- verify 판정 근거화
- archive metadata/무결성

P1 (핵심 고도화)
- app-server 세션 생명주기(Resume/Fork/Rollback/Steer/Interrupt)
- streaming 처리
- approval 정책

P2 (운영/복리화)
- compound triage/decay
- review 8-lens
- compaction

---

## 14) v2 기획서 대비 누락 요구사항 (JARVIS 감사 2026-02-26)

> 아래 항목은 옵시디언 `ax - 코딩 에이전트 코디네이터 CLI v2.md` 기획서에 설계되었으나
> 본 gap 문서에서 누락된 것들이다.

### 14.1 TDD 루프 상세 요구사항 (Must)

v2 기획서의 핵심 실행 모델이나, §4.3 `run`에 TDD 모드가 명시되지 않았다.

- `ax run --tdd` 플래그를 Must로 추가한다.
- TDD 루프: **Red → Green → Refactor** 순서를 강제해야 한다.
- Tier 기반 점진 실행:
  - T0: 핵심 경로 (스모크 테스트)
  - T1: 엣지 케이스
  - T2: 리팩토링 + 성능
- 각 TDD step은 Codex Turn 1개에 1:1 매핑되어야 한다.
- tier 간 progression gate: 이전 tier 통과 없이 다음 tier 진입 차단.
- `state.yaml`에 `tdd.current_tier`, `tdd.current_step`, `tdd.completed_steps`, `tdd.total_steps` 필드 관리.

### 14.2 Git Worktree 격리 요구사항 (Must)

v2 기획서에서 실행 격리 방법으로 설계되었으나 gap 문서에 없다.

- `ax run` 실행 시 Git Worktree를 생성하여 격리된 환경에서 작업해야 한다.
- worktree 경로: `.ax/worktrees/{proposal-id}/`
- 성공 시 메인 브랜치에 머지, 실패 시 worktree 정리.
- `--no-worktree` 플래그로 비활성화 가능 (기본은 활성).

### 14.3 Adaptive Depth 자동 라우팅 로직 (Must)

§10.8에서 승격은 되었으나, 실제 분류 로직이 구체화되지 않았다.

- `TaskComplexity` 분류기를 구현해야 한다:
  - **Quick**: 1~2 파일, 단순 수정 (타이포, config 변경)
  - **Normal**: 3~10 파일, 기능 추가
  - **Deep**: 10+ 파일, 아키텍처 변경, 크로스 모듈
  - **Explore**: 불확실, "뭘 만들지" 정의 안 됨
- 분류 기준:
  1. 태스크 설명 키워드 분석 ("수정"→Quick, "리팩토링"→Deep, "설계"→Explore)
  2. 영향 받는 파일 수 추정 (코드베이스 정적 분석)
  3. 크로스 모듈 의존성 여부
- 사용자 override 우선순위: `--depth` 플래그 > 자동 분석.
- 자동 분석 결과를 사용자에게 표시 후 확인 받는 인터랙티브 모드 지원 (Should).

### 14.4 Party Mode / Multi-Persona Brainstorm 요구사항 (Should → Must)

§4.6 `discover`에 "party mode 관점 필드"로 Should 처리되었으나, v2 기획서에서는 핵심 기능.

- `ax discover --party` 실행 시 최소 3개 페르소나 시뮬레이션:
  - **Architect**: 기술 구조, 확장성, 의존성 관점
  - **User**: 사용자 경험, 엣지 케이스 관점
  - **QA**: 테스트 가능성, 실패 시나리오 관점
- 각 페르소나 의견을 별도 섹션으로 산출물에 기록.
- 트레이드오프 매트릭스 자동 생성.

### 14.5 `ax run` reject/steer 인터랙션 요구사항 (Must)

v2 기획서의 TDD 루프에서 에이전트 산출물을 사용자가 제어하는 핵심 메커니즘.

- 각 Turn 완료 후 사용자에게 `accept / reject / steer` 선택지 제공.
- **reject**: 해당 Turn 결과를 폐기하고 재실행. 사유를 기록.
- **steer**: 추가 지시를 Turn에 주입하여 방향 수정.
- reject/steer 히스토리를 `state.yaml`에 기록하여 추후 compound에서 학습 자료로 활용.

### 14.6 Quality Gate (리뷰 횟수 제한) 요구사항 (Must)

v2 기획서의 실행 단계에 명시된 안전장치.

- `ax run --tdd` 실행 중 tier당 최대 리뷰 횟수 제한 (기본 3회).
- 제한 초과 시:
  1. 사용자에게 경고 + 수동 개입 요청
  2. `--force` 플래그로 무시 가능
- 무한 Red→Green 루프 방지 목적.

### 14.7 Context Injection 영향 경로 분석 요구사항 (Must)

§10.3에서 "영향 경로 기반"이라 했으나 구체 로직이 없다.

- `task.analyzeAffectedDirectories()` 구현:
  1. 태스크 설명에서 파일/모듈명 추출
  2. proposal의 `specs/` + `design.md`에서 언급된 경로 수집
  3. git diff 기반 최근 변경 경로 참조
- 영향 경로 목록을 `state.yaml.context_chain`에 기록.
- Layer 2 CLAUDE.md 검색 범위를 영향 경로로 한정.

### 14.8 내장 스킬 시스템 요구사항 (Must)

ax는 외부 의존 없이 아래 스킬을 자체 내장해야 한다.

#### 내장 스킬 목록

| 스킬 | 용도 | 사용 시점 |
|------|------|----------|
| `openspec` | Spec-Driven 문서 생성 (proposal/specs/design/tasks) | `propose`, `plan`, `verify`, `archive` |
| `superpowers` | 멀티 관점 분석 (brainstorm, critique, party mode) | `discover`, `propose --party`, `review` |
| `tdd-plan` | TDD 계획 수립 (tier 분해, 테스트 케이스 설계) | `plan --tdd` |
| `tdd-go` | 스텝바이스텝 TDD 실행 (단일 Red→Green→Refactor) | `run --tdd` |
| `tdd-go-loop` | 티어별 TDD 자동 루프 (구현→리팩토링→리뷰 반영) | `run --tdd --loop` |
| `interview` | 인터랙티브 질문으로 요구사항 구체화 | `propose`, `plan`, `discover` |

#### `tdd-go` 상세 요구사항 (Must)

스텝바이스텝 실행 모드. 엔진: **gpt-5.3-spark** (빠른 반복에 최적).

- 각 스텝마다:
  1. Red: 실패하는 테스트 작성
  2. Green: 최소 구현으로 통과
  3. Refactor: 정리
- **diff 자동 출력이 핵심**:
  - 각 스텝 완료 후 `git diff --stat` + `git diff` 전체 출력
  - diff에 대한 **로직 설명** 자동 생성 ("왜 이렇게 바뀌었는지")
  - diff에 대한 **코드 리뷰 코멘트** 자동 생성 (잠재 이슈, 개선점)
- 사용자가 각 스텝을 확인 후 다음 스텝 진행 (accept/reject/steer)
- `state.yaml`에 현재 스텝 위치 기록 → 중단 후 재개 가능

#### `tdd-go-loop` 상세 요구사항 (Must)

티어별 자동 루프 모드. 가이드(plan) 대로 T0→T1→T2 순차 실행.

- 각 티어 완료 후:
  1. 자동 리팩토링 패스
  2. **자세한 코드 리뷰 보고서** 생성:
     - 아키텍처 적합성
     - 테스트 커버리지 분석
     - 네이밍/패턴 일관성
     - 성능 우려사항
     - 다음 티어 진입 전 수정 권고사항
  3. 리뷰 반영 패스 (권고사항 자동 적용 또는 사용자 선택)
- 티어 간 progression gate:
  - 리뷰 보고서에서 critical 이슈 0건이어야 다음 티어 진입
  - critical 이슈 존재 시 수정 후 재리뷰
- 전체 완료 후 종합 리뷰 보고서 + compound 자동 트리거

#### `interview` 스킬 요구사항 (Must)

`propose`와 `plan` 과정에서 인터랙티브 질문으로 요구사항을 구체화.

- `ax propose "기능명"` 실행 시:
  1. 기능 설명에서 모호한 부분 자동 탐지
  2. 사용자에게 구조화된 질문 제시 (최소 3~5개):
     - 스코프 경계 ("X는 포함인가 제외인가?")
     - 엣지 케이스 ("Y 상황에서 어떻게 동작해야 하는가?")
     - 우선순위 ("A와 B 중 먼저 필요한 것은?")
  3. 답변을 proposal.md에 자동 반영
- `ax plan --from proposal` 실행 시:
  1. proposal의 acceptance criteria에서 구현 모호점 질문
  2. 기술적 트레이드오프 선택지 제시
  3. 답변을 plan에 반영
- `ax discover` 실행 시에도 interview 적용 가능
- `--no-interview` 플래그로 비활성화 가능

#### 스킬 로딩 아키텍처

```
.ax/skills/
├── builtin/           # ax 바이너리에 임베드 (go:embed)
│   ├── openspec/
│   ├── superpowers/
│   ├── tdd-plan/
│   ├── tdd-go/
│   ├── tdd-go-loop/
│   └── interview/
├── custom/            # 사용자 추가 스킬
└── registry.yaml      # 스킬 매핑 (명령 → 스킬 체인)
```

- 내장 스킬은 `go:embed`로 바이너리에 포함 (외부 파일 불필요).
- `registry.yaml`로 명령별 스킬 체인 정의:
  ```yaml
  propose:
    skills: [interview, openspec]
  plan:
    skills: [interview, openspec, tdd-plan]
  run:
    skills: [tdd-go]        # --loop 시 tdd-go-loop
  discover:
    skills: [interview, superpowers]
  review:
    skills: [superpowers]
  ```
- 커스텀 스킬은 `custom/`에 추가하면 자동 인식.

---

## 15) 의사결정 필요 항목 (추가)

5. TDD tier 기본 구성 (T0/T1/T2 vs 커스텀)
6. Git Worktree 기본 활성화 여부 (소규모 프로젝트에선 오버헤드)
7. Party Mode 페르소나 목록 고정 vs 사용자 커스텀
8. reject/steer 최대 횟수 (무한 루프 방지)
9. Quality Gate 기본 리뷰 횟수 제한값 (3회 vs 5회)
10. tdd-go 기본 엔진을 gpt-5.3-spark로 고정할지, 사용자 설정 허용할지
11. tdd-go-loop에서 리뷰 반영을 자동 적용 vs 사용자 확인 후 적용
12. interview 질문 수 기본값 (3~5개 vs 동적)
13. 내장 스킬 업데이트 방식 (바이너리 재빌드 vs 외부 오버라이드)

---

## 16) 수용 테스트 추가 (§11 보완)

15. Git Worktree 생성/머지/정리 테스트
16. Adaptive Depth 자동 분류 정확성 테스트 (5개 시나리오)
17. reject→재실행→accept 풀 사이클 테스트
18. steer 주입 후 Turn 방향 변경 검증 테스트
19. Quality Gate 횟수 초과 차단 테스트
20. Party Mode 3-persona 산출물 완전성 테스트
21. Context Injection 영향 경로 일관성 테스트 (동일 입력→동일 경로)
22. tdd-go 스텝별 diff 출력 + 로직 설명 + 코드 리뷰 자동 생성 테스트
23. tdd-go-loop 티어 완료→리뷰 보고서→리뷰 반영→다음 티어 풀 사이클 테스트
24. tdd-go-loop critical 이슈 시 progression gate 차단 테스트
25. interview 질문 생성→답변 수집→proposal 반영 E2E 테스트
26. 내장 스킬 6개 로딩 + registry.yaml 매핑 정합성 테스트

---

## 17) 최종 재검증 (2026-02-26, Worker-3)

### 17.1 이번 보강으로 닫힌 코드 갭
- TDD tier progression gate(T0→T1→T2) + state persistence/테스트 반영
- approval policy 엔진(`never/on-failure/unless-allow-listed/always`) run 경로/테스트 반영
- verify previous-vs-current diff 산출(`verify-diff.md` + verify 본문 참조) 반영
- compound triage 실구현(FixCandidate/Document/Noise) + gotcha schema 필드 반영 + `--audit` decay/archive 후보 보고 반영
- context_chain 임계치 초과 시 compaction 트리거 + 로그 반영/테스트 검증
- worktree 경로를 plan `from:` 기준으로 결정(결정성) + reject 실패 경로 부작용 방지(불필요 worktree 미생성)

### 17.2 재검증 증거
- `go test ./...` **PASS** (2026-02-26)
- `go build ./...` **PASS** (2026-02-26)
- CLI E2E `propose→plan→run→verify→archive` **PASS** (2026-02-26)

### 17.3 잔여 오픈 항목(갭 클로징 범위)
- 없음 (본 gap 문서의 Must 범위는 코드/테스트/E2E 기준으로 재검증 완료)

---

## 18) 최종 재검증 보강 (2026-02-26, Worker-1)

### 18.1 체크리스트 동기화
- 1/3, 2/3, 3/3 문서의 T2~T3 체크리스트가 코드/테스트 결과와 일치함을 재확인
- tier progression gate(T0→T1→T2), approval policy, verify diff, compound triage/audit, compaction trigger, worktree 결정성/실패 부작용 방지 항목 반영 상태 재점검 완료

### 18.2 실행 증거 (재검증)
- `go test ./...` **PASS** (2026-02-26)
- `go build ./...` **PASS** (2026-02-26)
- CLI E2E `propose→plan→run→verify→archive` **PASS** (2026-02-26)

---

## 18) 최종 재검증 Addendum (2026-02-26, Worker-2)

### 18.1 재검증/보강 포인트
- approval policy 판단 로직/사유 표준화 + run 보고서 반영 검증
- verify diff 포맷(Added/Removed 라인 포함) + verify 본문 `Previous Verify Diff` 섹션 회귀 검증
- compound gotcha schema 파싱/triage/audit 회귀 검증
- context compaction 저장 시 체인 길이 상한 유지 + compaction 로그 생성 회귀 검증
- worktree 경로 결정성(`plan from`) 및 reject 실패 경로 무부수효과 재검증

### 18.2 실행 증거
- `go test ./...` **PASS**
- `go build ./...` **PASS**
- CLI E2E `propose→plan→run→verify→archive` **PASS**

---

## 19) 최종 구현 클로징 재검증 (2026-02-26, Leader)

### 19.1 추가 반영 사항
- `run` 보고서에 Step↔Turn 1:1 매핑 테이블 실구현(결정성 turn id 포함)
- observability 로그에 synthetic thread/turn id 연동
- `archive` 시 worktree lifecycle 마감(merged/cleaned snapshot 생성 + 원 worktree 정리)
- streaming completed 누락 시 자동 보정(normalize) 로직 및 테스트 추가
- builtin skill registry에 `tdd-go-loop` 체인 반영 + 계약 테스트 추가
- `docs/mvp-walkthrough.md` 최신 실행 절차로 갱신

### 19.2 최종 실행 증거 (2026-02-26)
- `go test -v ./...` **PASS**
- `go build ./...` **PASS**
- 실동작 E2E (실제 `ax` 바이너리 직접 실행) **PASS**
  - 경로: `/tmp/ax-v2-e2e-final3-AbNBgC`
  - 시나리오: `state→propose→plan→run(quality gate fail/force)→discover --party→review→compound --audit→verify→archive(strict fail/allow override)→quick escalation→state --json`

### 19.3 Gap 정합성 결론
- `docs/ax-v2-tdd-implementation-master-plan.md`
- `docs/ax-v2-tdd-test-matrix.md`
- `docs/ax-v2-tdd-execution-and-final-review.md`

위 3개 실행 문서와 본 gap 문서의 Must 요구사항을 대조한 결과, 현재 코드/테스트/실행 결과는 gap 기준과 정합함.
