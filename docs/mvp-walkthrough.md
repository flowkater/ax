# ax v2 CLI MVP Walkthrough

## 1) 초기화/상태 확인
```bash
ax state
```
생성 확인:
- `.ax/state.yaml`
- `.ax/{proposals,plans,archive}`
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

## 4) 이후 단계(스텁)
```bash
ax run --plan <plan-id-or-path>
ax verify --proposal <proposal-id-or-path>
ax archive --proposal <proposal-id-or-path>
ax discover <topic>
ax quick <task>
```
