# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
go build -o jira8 .        # build binary
go vet ./...                   # lint
go test ./...                  # run all tests
go test ./internal/client/...  # run tests for a single package
```

Requires a `~/.jira.yaml` with a `token` field (or `JIRA_TOKEN` env var) to run against the real Jira instance.

## Architecture

This is a Go CLI tool targeting **Jira Server 8.7.1 REST API v2** (`https://jira.amplia.es/jira`). It has two modes: interactive CLI and MCP server (stdio transport for AI agent integration).

### Key layers

- **`cmd/root.go`** — Cobra root command. `PersistentPreRunE` loads config, creates the HTTP client, and stores both in `cmd/app.State` (a package-level singleton to avoid circular imports between `cmd` and `cmd/issue`).
- **`cmd/app/`** — Shared state holder (`State` struct with Config, Client, Output). All subcommands access it via `app.Get()`.
- **`cmd/issue/`** — One file per subcommand (list, view, create, edit, transition). `format.go` has all terminal rendering helpers using lipgloss.
- **`cmd/mcp.go`** — MCP server using `mark3labs/mcp-go`. Registers 13 tools that reuse the same `client.Client`. Tool errors return `mcp.NewToolResultError()` (tool-level), not Go errors (transport-level).
- **`internal/client/jira.go`** — Single HTTP client with Bearer auth, 15s timeout, 429 retry (up to 3 attempts), and Jira error parsing into `APIError`. All API methods go through the private `do()` helper. `BuildJQL()` is shared between CLI and MCP.
- **`internal/config/`** — Viper-based config loading: flags > env vars > `~/.jira.yaml`.
- **`internal/models/`** — Jira API request/response structs. `EditIssueRequest.Fields` is `map[string]any` for partial updates.

### Jira Server 8 specifics

- User references use `{"name": "username"}`, **not** `accountId` (that's Jira Cloud).
- Description is plain text / wiki markup, **not** ADF.
- Auth supports both **Bearer token** (`token` field) and **Basic Auth** (`user` + `password` fields). Basic Auth is the default for Jira Server without PAT enabled.
- Transitions require two calls: GET transitions to resolve name→ID, then POST.
- `--assignee me` in list uses JQL `currentUser()` (no extra API call); in create/edit it calls `/rest/api/2/myself` to resolve the username.

## Development rules

- **CLI ↔ MCP parity is mandatory.** Every CLI command/flag must have a corresponding MCP tool and vice versa. When adding a new feature, implement both CLI and MCP in the same commit/PR.
- Comments in code should include AI agent context: describe what each MCP tool does and what parameters it accepts clearly, so AI agents can discover and use them effectively.

## Git conventions

- **No `Co-Authored-By: Claude` trailers** in commit messages.
