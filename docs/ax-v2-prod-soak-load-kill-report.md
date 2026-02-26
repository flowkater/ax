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

---

## 9) Runtime Hardening Final Revalidation (worker-2, 2026-02-26)

### 9.1 Code hardening applied
- Runtime metadata 확장: `cluster_id`, `node_id`, `session_journal`, `session_meta`
- Root persistent flags 확장: `--cluster-id`, `--node-id` (+ env fallback `AX_CLUSTER_ID`, `AX_NODE_ID`)
- Shared/worktree concurrency semantics 강화:
  - shared/worktree 모두 session-isolated worktree 경로 사용
  - `.ax/worktrees/<proposal>/<session-id>/worktree.yaml`
- Journal/recovery hardening:
  - `runtime-journal.jsonl` start/checkpoint/completed/failed 기록
  - `.ax/runtime/checkpoints/<session-id>.json` checkpoint 저장/복원
  - `recover --strategy auto`의 resume/rerun 결정 로직 보강

### 9.2 Fresh evidence (real binary)
- Evidence root: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0`
- Summary: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/summary.json`
- Binary: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/ax`
- Binary SHA256: `3638d46c64689a553f034dd57702bb0a981e7e2223dc498633deb65fd00e5217`

Track results:
- Track-1 sequential soak: **20/20 success**
  - detail: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/track1-sequential-soak.json`
- Track-2 shared parallel load (48 jobs, 8 workers): **48/48 success**
  - detail: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/track2-shared-load.json`
- Track-3 kill-recovery (12 attempts): **12/12 success**
  - SIGKILL delivered: 12/12
  - state JSON integrity: 12/12
  - recover success: 12/12
  - detail: `/var/folders/h8/941vlqss6tqd_mfy7g77n98h0000gn/T/ax-worker2-final-pwy0h6i0/track3-kill-recovery.json`

### 9.3 Fresh baseline quality gates
- `go test -race ./...` PASS (`go-test-race.log`)
- `go vet ./...` PASS (`go-vet.log`)
- `go build ./...` PASS (`go-build.log`)

### 9.4 Final risk statement
- unresolved risks: **none**

---

## 9) Worker-3 Final Runtime Evidence Refresh (2026-02-26)

### 9.1 Environment (latest run)
- Run window (UTC): **2026-02-26T12:00:11Z ~ 2026-02-26T12:01:13Z**
- Repo: `/Users/flowkater/.superset/projects/ax` (commit `ac653fc`)
- Built binary: `/tmp/ax-worker3-evidence-20260226-210011/ax`
- Binary SHA256: `3638d46c64689a553f034dd57702bb0a981e7e2223dc498633deb65fd00e5217`
- Evidence root: `/tmp/ax-worker3-evidence-20260226-210011`

### 9.2 Quality gate logs
- `go test -race ./...` log: `/tmp/ax-worker3-evidence-20260226-210011/go-test-race.log` (**PASS**)
- `go vet ./...` log: `/tmp/ax-worker3-evidence-20260226-210011/go-vet.log` (**PASS**)
- `go build ./...` log: `/tmp/ax-worker3-evidence-20260226-210011/go-build.log` (**PASS**)

### 9.3 Real binary soak/load/kill summary
Source summary: `/tmp/ax-worker3-evidence-20260226-210011/runtime-summary.json`

- Track-1 sequential soak:
  - iterations: 12
  - success/failure: **12 / 0**
  - detail: `/tmp/ax-worker3-evidence-20260226-210011/track1-sequential.json`
- Track-2 shared parallel load:
  - jobs/workers: 24 / 6
  - success/failure: **24 / 0**
  - detail: `/tmp/ax-worker3-evidence-20260226-210011/track2-parallel.json`
- Track-3 kill-recovery:
  - attempts: 6
  - SIGKILL delivered: **6 / 6**
  - success/failure: **6 / 0**
  - detail: `/tmp/ax-worker3-evidence-20260226-210011/track3-kill-recovery.json`

### 9.4 Conclusion (latest)
- Runtime metadata/journal/checkpoint + shared session worktree isolation 변경 이후 최신 실바이너리 시나리오에서 soak/load/kill 모두 PASS.
- 현재 evidence 기준 unresolved production risk: **none**.

---

## 10) Worker-1 Runtime Revalidation Refresh (2026-02-26)

### 10.1 Environment (latest run)
- Run window (UTC): **2026-02-26T12:01:48Z ~ 2026-02-26T12:01:57Z**
- Repo: `/Users/flowkater/.superset/projects/ax` (commit `ac653fc`)
- Built binary: `/tmp/ax-prod-soak-20260226-120148/ax`
- Binary SHA256: `3638d46c64689a553f034dd57702bb0a981e7e2223dc498633deb65fd00e5217`
- Evidence root: `/tmp/ax-prod-soak-20260226-120148`

### 10.2 Quality gate logs
- `go test -race ./...` log: `/tmp/ax-prod-soak-20260226-120148/go-test-race.log` (**PASS**)
- `go vet ./...` log: `/tmp/ax-prod-soak-20260226-120148/go-vet.log` (**PASS**)
- `go build ./...` log: `/tmp/ax-prod-soak-20260226-120148/go-build.log` (**PASS**)

### 10.3 Real binary soak/load/kill summary
Source summary: `/tmp/ax-prod-soak-20260226-120148/summary.json`

- Track-1 sequential soak:
  - iterations: 20
  - success/failure: **20 / 0**
  - detail: `/tmp/ax-prod-soak-20260226-120148/track1.json`
- Track-2 shared parallel load:
  - jobs/workers: 48 / 8
  - success/failure: **48 / 0**
  - detail: `/tmp/ax-prod-soak-20260226-120148/track2.json`
- Track-3 kill-recovery:
  - attempts: 12
  - SIGKILL delivered: **12 / 12**
  - state JSON parse ok: **12 / 12**
  - success/failure: **12 / 0**
  - detail: `/tmp/ax-prod-soak-20260226-120148/track3.json`

### 10.4 Conclusion (latest)
- runtime metadata(클러스터/노드/세션) + session worktree isolation + journal/checkpoint/recover 보강 후 최신 실바이너리 재검증에서도 soak/load/kill 전 항목 PASS.
- 현재 evidence 기준 unresolved production risk: **none**.

---

## 11) Leader Final Production Gate Refresh (2026-02-26)

### 11.1 Fresh quality gates
- `go test -count=1 ./...` **PASS**
- `go test -count=1 -race ./...` **PASS**
- `go vet ./...` **PASS**
- `go build ./...` **PASS**

### 11.2 Real binary full smoke (runtime/doctor/recover 포함)
- Evidence root: `/tmp/ax-live-verify-debug3-iYIOCG`
- Summary: `/tmp/ax-live-verify-debug3-iYIOCG/live-runtime-summary.json`
- Scenario:
  - `state`
  - `propose/plan/run` (shared session)
  - `doctor runtime --json`
  - `recover --strategy auto`
  - `discover runtime --party`
  - `review --proposal`
  - `compound --audit`
  - `verify --proposal --tests pass --build pass --ac pass`
  - `archive --proposal`
  - `state --json`

### 11.3 Soak / Load / Kill refresh
- Evidence root: `/tmp/ax-live-soak-final-CVRKO7`
- Summary: `/tmp/ax-live-soak-final-CVRKO7/summary.json`
- Track-1 sequential soak: **10/10 success**
- Track-2 shared parallel load: **16/16 success**
- Track-3 kill + recover:
  - SIGKILL delivered: **6/6**
  - recover auto success: **6/6**

### 11.4 Final statement
- 최신 리더 재검증 기준에서도 soak/load/kill + runtime recover 경로 모두 정상.
- 현재 기준 unresolved production blocker: **none**.
