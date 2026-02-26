# ax v2 CLI Walkthrough (Updated: 2026-02-26)

## 1) 초기화/상태
```bash
ax state
ax state --json
ax --runtime-mode shared --session-id sess-demo state
```
- `.ax/state.yaml` (phase/current/context/tdd/progress/runtime)
- `.ax/skills/registry.yaml` (builtin skill contract)
- `.ax/locks/state.lock` (상태 업데이트 직렬화)

## 2) Propose → Plan
```bash
ax propose "Build CLI MVP"
ax plan --from .ax/proposals/<proposal-id>/proposal.md
```
- proposal 필수 섹션 + `tasks.md` 체크리스트 10개+
- plan 필수 섹션(phase/files/tests/rollback/task mapping)

## 3) Run (TDD/Approval/Quality Gate)
```bash
ax run --plan .ax/plans/<proposal-id>-plan.md --tdd --loop --depth deep --approval-policy on-failure
ax --runtime-mode worktree --session-id sess-a run --plan .ax/plans/<proposal-id>-plan.md
```
- 로그: `.ax/logs/*-{start,end,fail,compaction}.yaml`
- 리포트: `.ax/runs/*-run-*.md`
- Step↔Turn 1:1 매핑 표 포함
- quality gate: 기본 3회, 초과 시 `--force --force-reason`
- worktree:
  - single/shared: `.ax/worktrees/<proposal-id>/worktree.yaml`
  - worktree mode: `.ax/worktrees/<proposal-id>/<session-id>/worktree.yaml`
  - `--no-worktree`로 비활성

## 4) Discover / Review / Compound
```bash
ax discover "<topic>" --party
ax review --proposal .ax/proposals/<proposal-id>/proposal.md
ax compound --audit
```
- discover: 문제/옵션/트레이드오프/리스크/추천안 + Party(Architect/User/QA)
- review: 8-lens 리뷰 산출물
- compound: FixCandidate/Document/Noise triage + decay audit

## 5) Verify → Archive
```bash
ax verify --proposal .ax/proposals/<proposal-id>/proposal.md --tests pass --build pass --ac pass
ax archive --proposal .ax/proposals/<proposal-id>/proposal.md
```
- verify: PASS/FAIL/CONDITIONAL PASS + Evidence + Unmet + Next Actions + verify diff
- archive(strict 기본): verify.md 없으면 차단
- 예외: `--allow-unverified-archive`
- archive 시 metadata + worktree snapshot 생성, 원 worktree 정리

## 6) Quick Escalation
```bash
ax quick "hotfix" --files-changed 7
```
- 임계치(`files>5` 또는 cross-module/core-touch) 초과 시 자동 승격:
  - proposal + plan 생성 후 표준 플로우로 유도

## 7) Codex app-server integration
- JSON-RPC stdio client
- 기본 3메서드: `CreateThread`, `RunTurn`, `GetThread`
- lifecycle: `ResumeSession`, `ForkSession`, `RollbackTurns`, `SteerTurn`, `InterruptTurn`
- streaming: `StreamTurn` (`delta`, `completed`; completed 누락 시 자동 보정)

## 8) 운영 복구 / 진단
```bash
ax recover --strategy auto
ax recover --strategy resume
ax doctor runtime
ax doctor runtime --json
```
- `recover`: 중단/충돌 이후 runtime/session 상태를 implementation phase로 재정렬
- `doctor runtime`: runtime mode, session, lock 파일, state health 요약 출력
