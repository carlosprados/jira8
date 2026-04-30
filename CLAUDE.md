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
- **`cmd/mcp.go`** — MCP server using `mark3labs/mcp-go`. Registers all tools and wires up resources and prompts. Tool errors return `mcp.NewToolResultError()` (tool-level), not Go errors (transport-level).
- **`cmd/mcp_resources.go`** — Read-only `jira://` resources and resource templates (priorities, project types, project statuses, individual issue). Handlers return `application/json`.
- **`cmd/mcp_prompts.go`** — Reusable conversational prompts (`triage_issue`, `create_bug_report`, `epic_breakdown`) that pre-load Jira context and produce structured `PromptMessage`s. No CLI counterpart (see paridad rules below).
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

### Functional parity between CLI and MCP (mandatory)

**Functional parity is mandatory**: every piece of Jira functionality must be reachable from both CLI and MCP. If a feature exists only on one surface, the other surface is broken by definition.

The canonical carrier of that functionality is the pair **CLI subcommand ↔ MCP tool**:

- Every CLI subcommand (e.g. `issue create`) must have a corresponding MCP tool (`jira_create_issue`) and vice versa.
- Flag names and MCP tool parameter names should match (e.g. `--type` ↔ `type`, `--max` ↔ `max`). Unified in release v1.0.0; don't reintroduce divergence.
- When adding a new feature, implement both CLI subcommand and MCP tool in the same commit/PR.

### Resources and Prompts — MCP-only, with a safety net

Resources and Prompts are MCP-specific primitives that do not map cleanly to a CLI (a slash command that emits conversational blobs for an LLM has no meaning in a non-LLM terminal). They are exempt from the "bidirectional" parity rule, **but** with one non-negotiable constraint:

- **No functionality may exist only as a Resource or Prompt.** Every Resource or Prompt must be backed by the same data that a CLI subcommand + MCP tool can already produce. Resources and Prompts are UX sugar (discoverability, pre-fetched context, fewer tool round-trips), not new capabilities.

Example: `jira://issues/{key}` resource and `triage_issue` prompt both rely on the same underlying `GetIssue` call that `issue view` and `jira_get_issue` expose. If an LM Studio user (no resources, no prompts) can still `jira_get_issue` and reconstruct the same workflow, we're good.

When adding a Resource or Prompt, check:
1. Is the underlying functionality already reachable via CLI + MCP tool? If no, add it there first.
2. Does the Resource/Prompt expose anything that the tool can't? If yes, move that capability into a tool.

### Other

- Comments in code should include AI agent context: describe what each MCP tool / resource / prompt does and what parameters it accepts clearly, so AI agents can discover and use them effectively.
- Client support matrix (as of v1.0.0): Claude Code supports tools/resources/prompts; Gemini CLI supports tools/prompts (resources experimental); LM Studio supports tools only. Designs must degrade gracefully across all three.

## Git conventions

- **No `Co-Authored-By: Claude` trailers** in commit messages.
