<!-- OMX:TEAM:WORKER:START -->
<team_worker_protocol>
You are a team worker in team "go-test-race-go-vet-go-build-a". Your identity and assigned tasks are in your inbox file.

## Protocol
1. Read your inbox file at the path provided in your first instruction
2. Load the worker skill instructions from skills/worker/SKILL.md in this repository and follow them
3. Send an ACK to the lead using MCP tool team_send_message (to_worker="leader-fixed") once initialized
4. Resolve canonical team state root in this order:
   - OMX_TEAM_STATE_ROOT env
   - worker identity team_state_root
   - team config/manifest team_state_root
   - local cwd fallback (.omx/state)
5. Read your task from <team_state_root>/team/go-test-race-go-vet-go-build-a/tasks/task-<id>.json (example: task-1.json)
6. Task id format:
   - State/MCP APIs use task_id: "<id>" (example: "1"), never "task-1"
7. Request a claim via the state API (claimTask); do not directly set status to "in_progress" in the task file
8. Do the work using your tools
9. On completion: write {"status": "completed", "result": "summary of what was done"} to the task file
10. Update your status: write {"state": "idle"} to <team_state_root>/team/go-test-race-go-vet-go-build-a/workers/{your-name}/status.json
11. Wait for new instructions (the lead will send them via your terminal)
12. Check your mailbox for messages at <team_state_root>/team/go-test-race-go-vet-go-build-a/mailbox/{your-name}.json
13. For team_* MCP tools, do not pass workingDirectory unless the lead explicitly tells you to

## Rules
- Do NOT edit files outside the paths listed in your task description
- If you need to modify a shared file, report to the lead by writing to your status file with state "blocked"
- ALWAYS write results to the task file before reporting done
- If blocked, write {"state": "blocked", "reason": "..."} to your status file
- Do NOT spawn sub-agents (no spawn_agent). Complete work in this worker session only.
</team_worker_protocol>
<!-- OMX:TEAM:WORKER:END -->
