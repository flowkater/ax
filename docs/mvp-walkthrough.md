# ax v2 CLI MVP Walkthrough

## 1) 초기화/상태 확인
```bash
ax state
```
생성 확인:
- `.ax/state.yaml`
- `.ax/{proposals,plans,archive,runs,discovery}`
- `.ax/memory/{MEMORY.md,gotchas.md}`

## 2) 제안 생성
```bash
ax propose "Build CLI MVP"
```
생성 확인:
- `.ax/proposals/<id>/proposal.md`
- `.ax/proposals/<id>/design.md`
- `.ax/proposals/<id>/tasks.md`
- `.ax/proposals/<id>/specs/`

## 3) 계획 생성
```bash
ax plan --from <proposal-id>
```
생성 확인:
- `.ax/plans/<proposal-id>-plan.md`

## 4) 실행(run)
```bash
ax run --plan <proposal-id>-plan.md
```
생성 확인:
- `.ax/runs/<plan>-run-<timestamp>.md`
- 플랜 내 미완료 task(`- [ ]`) 개수 감지/리포트

## 5) 탐색(discover)
```bash
ax discover <topic>
```
생성 확인:
- `.ax/discovery/<topic>-<timestamp>.md`
- 워크스페이스 파일명/본문 기반 최대 20건 매치 리포트

## 6) 퀵 플로우(quick)
```bash
ax quick "fix-tests"
```
동작:
1. proposal 자동 생성
2. plan 자동 생성
3. run 자동 실행

## 7) verify/archive
```bash
ax verify --proposal <proposal-id>
ax archive --proposal <proposal-id>
```
생성/이동 확인:
- `.ax/proposals/<proposal-id>/verify.md`
- `.ax/archive/<proposal-id>/`

## 8) Codex app-server 최소 연동
- `internal/codex/client.go`의 JSON-RPC stdio 클라이언트 제공
- `CreateThread`, `RunTurn`, `GetThread` 최소 메서드 구현
