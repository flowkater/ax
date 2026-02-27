# ax

`ax`는 propose → plan → run → verify → archive 워크플로우를 지원하는 CLI입니다.

## Build

```bash
go build -o ./bin/ax .
./bin/ax --help
```

## Quick Start

```bash
# 1) 제안 생성
./bin/ax propose "codex app-server 연동 안정화"

# 2) 계획 생성 (proposal id/path 지정)
./bin/ax plan --from <proposal-id-or-path>

# 3) 실행 (plan id/path 지정)
./bin/ax run --plan <plan-id-or-path>

# 4) 검증
./bin/ax verify --proposal <proposal-id-or-path> --tests pass --build pass --ac pass

# 5) 아카이브
./bin/ax archive --proposal <proposal-id-or-path>
```

실행 중 중단/장애가 나면:

```bash
./bin/ax recover --strategy auto
```

## 주요 명령

- `ax propose <title>`: 제안서 생성
- `ax plan --from <proposal>`: 계획서 생성
- `ax run --plan <plan>`: 계획 실행
- `ax verify --proposal <proposal> --tests|--build|--ac <pass|fail|unknown>`: 검증 결과 기록
- `ax archive --proposal <proposal>`: 완료 제안 아카이브
- `ax state`: 현재 상태 확인
- `ax tui`: 인터랙티브 TUI / `--snapshot` 스냅샷 출력
- `ax doctor runtime --json`: 런타임 진단
- `ax quick <task>`: 빠른 작업 흐름

상세 옵션은 각 명령에서 확인:

```bash
./bin/ax <command> --help
```

## Codex App Server 연동 환경변수

- `AX_CODEX_MODE`: `real` | `scaffold` (기본 `scaffold`)
- `AX_CODEX_BIN`: Codex 실행 바이너리 경로 (기본 `codex`)
- `AX_CODEX_ARGS`: Codex 실행 인자
- `AX_CODEX_TIMEOUT`: 호출 timeout (예: `30s`)
- `AX_CODEX_RETRIES`: 재시도 횟수

예시:

```bash
AX_CODEX_MODE=real AX_CODEX_TIMEOUT=45s ./bin/ax run --plan <plan-id>
```

## 런타임 관련 글로벌 플래그

모든 명령에서 사용 가능:

- `--runtime-mode` (`single|shared|worktree|auto`)
- `--session-id`
- `--cluster-id`
- `--node-id`

## 산출물 위치

기본적으로 작업 산출물은 `.ax/` 아래에 저장됩니다.

- `.ax/proposals/`
- `.ax/plans/`
- `.ax/runs/`
- `.ax/archive/`
- `.ax/logs/`
- `.ax/state.yaml`

