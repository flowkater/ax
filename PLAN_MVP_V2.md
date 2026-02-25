# ax v2 CLI MVP 착수 플랜

## 목표
- 범위: CLI 전용 (daemon/gui 제외)
- 핵심 루프: `propose -> plan -> run -> verify -> archive`
- 운영 명령: `discover`, `quick`, `state`, `compound`

## MVP 범위 (이번 구현)
1. CLI 커맨드 골격
   - `ax propose <title>`
   - `ax plan --from <proposal>`
   - `ax run --plan <plan>`
   - `ax verify --proposal <proposal>`
   - `ax archive --proposal <proposal>`
   - `ax discover <topic>`
   - `ax quick <task>`
   - `ax state`
2. 상태/산출물 구조
   - `.ax/state.yaml`
   - `.ax/proposals/<id>/{proposal.md,specs/,design.md,tasks.md}`
   - `.ax/plans/`
   - `.ax/archive/`
   - `.ax/memory/{MEMORY.md,gotchas.md}`
3. Context Policy
   - Rule/Why/Enforcement/Scope 포맷
   - Non-discoverable + TTL 규칙 정의
4. Codex app-server 어댑터 스텁
   - JSON-RPC 인터페이스/타입
   - Thread/Turn 개념 모델
   - 실제 호출은 다음 phase

## 병렬 Codex 실행 단위
- Stream A (Core CLI): 명령/라우팅/상태 파일 생성
- Stream B (Spec Pipeline): propose/plan/verify/archive 파일 생성기
- Stream C (Context+Compound): 규칙 triage 포맷 + decay 메타 구조 + 문서

## 완료 기준
- `ax --help`에 8개 명령 표시
- `ax propose`, `ax plan`, `ax state` 실행 시 파일 생성 확인
- 예제 워크플로 1회 실행 문서화 (`docs/mvp-walkthrough.md`)

## 제외 범위
- daemon/ws
- editor integration
- 실시간 스트리밍 UI
- 고급 review 8-lens 자동화(다음 단계)
