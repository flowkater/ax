# Worker Assignment: worker-3

**Team:** go-test-race-go-vet-go-build-a
**Role:** reviewer
**Worker Name:** worker-3

## Your Assigned Tasks

- **Task 3**: Worker 3 bootstrap
  Description: Coordinate on: 프로덕션 레벨 검증: go test -race/go vet/go build, 실제 ax 바이너리 E2E(quality gate fail/force steer/discover party/review/compound/verify/archive strict&override/quick escalation) 실행 후 PASS/FAIL 근거를 보고하라

Report findings/results back to the lead and keep task updates current.
  Status: pending

## Instructions

1. Load and follow `skills/worker/SKILL.md`
2. Send startup ACK to the lead mailbox using MCP tool `team_send_message` with `to_worker="leader-fixed"`
3. Start with the first non-blocked task
4. Resolve canonical team state root in this order: `OMX_TEAM_STATE_ROOT` env -> worker identity `team_state_root` -> config/manifest `team_state_root` -> local cwd fallback.
5. Read the task file for your selected task id at `/Users/flowkater/.superset/projects/ax/.omx/state/team/go-test-race-go-vet-go-build-a/tasks/task-<id>.json` (example: `task-1.json`)
6. Task id format:
   - State/MCP APIs use `task_id: "<id>"` (example: `"1"`), not `"task-1"`.
7. Request a claim via state API (`claimTask`) to claim it
8. Complete the work described in the task
9. Write `{"status": "completed", "result": "brief summary"}` to the task file
10. Write `{"state": "idle"}` to `/Users/flowkater/.superset/projects/ax/.omx/state/team/go-test-race-go-vet-go-build-a/workers/worker-3/status.json`
11. Wait for the next instruction from the lead
12. For team_* MCP tools, do not pass `workingDirectory` unless the lead explicitly asks (if resolution fails, use leader cwd: `/Users/flowkater/.superset/projects/ax`)

## Scope Rules
- Only edit files described in your task descriptions
- Do NOT edit files that belong to other workers
- If you need to modify a shared/common file, write `{"state": "blocked", "reason": "need to edit shared file X"}` to your status file and wait
- Do NOT spawn sub-agents (no `spawn_agent`). Complete work in this worker session.
