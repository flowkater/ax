# ax v2 Production Soak/Load/Kill-Recovery Report (worker-3)

## 1) Environment
- Run window (UTC): **2026-02-26T11:19:54Z ~ 2026-02-26T11:20:27Z**
- Repo: `/Users/flowkater/.superset/projects/ax` (commit `b1ad4aa`)
- OS/Arch: `darwin/arm64`
- Go: `go1.24.7`
- Built binary (real ax executable): `/tmp/ax-prod-soak-20260226-201954/ax`
- Binary SHA256: `80af8fa6f9655a180c817a7be1bb0474c34e1dc50e53c8dd7239aff588a2ee92`
- Raw evidence root: `/tmp/ax-prod-soak-20260226-201954`

## 2) Commands Executed (Evidence)
### 2.1 Build actual ax binary
```bash
go build -o /tmp/ax-prod-soak-20260226-201954/ax .
```
Result: `rc=0`, binary size `6,241,090 bytes`.

### 2.2 Automated soak/load/kill execution
```bash
/tmp/ax_prod_soak_runner.py
```
Primary outputs:
- `/tmp/ax-prod-soak-20260226-201954/summary.json`
- `/tmp/ax-prod-soak-20260226-201954/track1/*.json`
- `/tmp/ax-prod-soak-20260226-201954/track2/*.json`
- `/tmp/ax-prod-soak-20260226-201954/track3/*.json`

### 2.3 Final re-validation
```bash
go test -race ./...
go build ./...
```

---

## 3) Track-1 Soak (30 sequential full E2E)
Flow used each iteration:
`propose -> plan -> run(--review-count 4 --force --decision steer --steer ...) -> discover --party -> review -> compound --audit -> verify -> archive`

- Temp dir: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-track1-soak-pic82u_3`
- Target: 30
- Completed: 30
- Success: 30
- Failure: 0
- Success rate: **100%**

Sample evidence (iteration 1):
- `propose: created 20260226-201955-track1-soak-01`
- `plan: created 20260226-201955-track1-soak-01-plan.md`
- `run: created ... (tasks=1)`
- `archive: moved 20260226-201955-track1-soak-01`

Conclusion: sequential full workflow was stable for 30/30 runs.

---

## 4) Track-2 Load (6 parallel workers, 60 minimal E2E)
Flow used per job:
`state --json -> propose -> plan -> run -> verify -> archive`

Test mode: **single shared temp dir + 6 parallel workers** (contention intentionally induced).

- Temp dir: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-track2-load-hvqeloyd`
- Target jobs: 60
- Completed: 60
- Success: 30
- Failure: 30
- Success rate: **50.0%**

Failure classification:
- `phase_transition_conflict`: 29
- `missing_file_path`: 1

Stage distribution of failures:
- `propose`: 23
- `verify`: 7

Race/lock/file conflict observations:
- phase transition conflicts: **29**
- file path collision/missing path events: **1**
- explicit lock contention errors: **0**
- explicit state JSON corruption errors: **0**

Failure samples:
- `Error: ERR_INVALID_PHASE_TRANSITION: planning -> proposal`
- `Error: ERR_INVALID_PHASE_TRANSITION: archived -> verification`
- `Error: stat .../.ax/proposals/20260226-201956-track2-load-shared-title: no such file or directory`

Conclusion: under shared-directory parallel load, global phase/state contention is the dominant failure mode.

---

## 5) Track-3 Kill-Recovery (SIGKILL during run, 20 attempts)
Method per attempt:
1. `propose -> plan`
2. Inflate plan file (~20MB) to lengthen `run` execution window
3. start `ax run ... --tdd --loop --depth deep`, then force `SIGKILL`
4. check state integrity via `ax state --json`
5. attempt `run --resume` (with force+steer)
6. if needed, fallback rerun
7. `verify -> archive`

- Temp dir: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-track3-kill-5ilt1fgp`
- Target attempts: 20
- Completed: 20
- SIGKILL delivered (`rc=-9`): 20/20
- State integrity (`ax state --json` valid): 20/20
- `run --resume` success: 13/20
- rerun success (including fallback): 20/20
- verify success: 20/20
- archive success: 20/20
- Overall success criterion (integrity + successful completion): **20/20 (100%)**

Resume failure samples observed:
- `Error: resume requested but no previous tdd state` (1)
- `Error: tier progression gate blocked: all tiers already completed` (6)

Conclusion: even with forced SIGKILL during run, state remained readable and all 20 attempts recovered to successful completion via resume or rerun.

---

## 6) Final Verification Evidence
### go test -race ./...
```text
?    github.com/flowkater/ax [no test files]
ok   github.com/flowkater/ax/cmd/ax (cached)
ok   github.com/flowkater/ax/internal/codex (cached)
ok   github.com/flowkater/ax/internal/core (cached)
```
Result: `rc=0`

### go build ./...
Result: `rc=0`

---

## 7) Final Summary
- **Track-1 (soak)**: stable (30/30)
- **Track-2 (parallel load)**: 30/60 success, failures mostly phase transition contention
- **Track-3 (kill-recovery)**: integrity and completion recovery confirmed (20/20)
- **Final baseline checks**: `go test -race ./...` and `go build ./...` both passed.

---

## 8) Runtime Hardening Revalidation (leader, 2026-02-26)

### 8.1 Implemented hardening
- Runtime mode support: `--runtime-mode single|shared|worktree|auto`
- Runtime session support: `--session-id`
- Global state write lock: `.ax/locks/state.lock` (file lock + timeout)
- Shared/worktree mode global phase transition relaxation (global phase becomes advisory)
- Early run checkpoint persistence for crash/kill recovery
- Worktree session isolation path in worktree mode:
  - `.ax/worktrees/<proposal>/<session-id>/worktree.yaml`

### 8.2 Revalidation evidence
- Load(shared mode) re-test:
  - jobs: 80, workers: 8
  - success: 80, failure: 0
  - evidence dir: `/tmp/ax-shared-load-final-LEt07m`
- Kill-recovery(shared mode) re-test:
  - attempts: 20
  - SIGKILL delivered: 20/20
  - state JSON parse ok: 20/20
  - `run --resume` success: 20/20
  - final verify/archive success: 20/20
  - evidence dir: `/tmp/ax-kill-final-gfsfrv`
  - binary: `/tmp/ax_prod_final_1772106239`

### 8.3 Baseline quality gates
- `go test ./...` PASS
- `go test -race ./...` PASS
- `go vet ./...` PASS
- `go build ./...` PASS

### 8.4 Conclusion update
- Shared workspace parallel contention issue(phase transition conflict)는 runtime hardening으로 해소됨.
- 현재 기준에서 soak/load/kill-recovery 모두 프로덕션 운영 가능한 수준으로 재검증 완료.
