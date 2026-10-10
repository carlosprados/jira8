# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
go build -o jira8 .        # build binary
go vet ./...                   # lint
go test ./...                  # run all tests
go test ./internal/client/...  # run tests for a single package
```

Requires a `~/.jira.yaml` with `url` and either `token` or `user` + `password` (or the `JIRA_*` env vars) to run against the real Jira instance.

## Architecture

This is a Go CLI tool targeting **Jira Server 8.7.1 REST API v2** (`https://jira.amplia.es/jira`), plus the Agile API (`/rest/agile/1.0`) for board ranking only. It is a CLI only: agents drive it from the shell, guided by the embedded skill (`internal/skill/SKILL.md`). The MCP server was removed in v2.0.0.

### Key layers

- **`cmd/root.go`** — Cobra root command. `PersistentPreRunE` loads config, creates the HTTP client, and stores both in `cmd/app.State` (a package-level singleton to avoid circular imports between `cmd` and `cmd/issue`).
- **`cmd/app/`** — Shared state holder (`State` struct with Config, Client, Output; all subcommands access it via `app.Get()`) plus shared helpers: text input from flag/file/stdin, `OutputJSON`, name matching, Markdown rendering of read results and `TrimHistory`.
- **`cmd/issue/`** — One file per subcommand (list, view, create, edit, transition(s), rank, link, comment-*, worklog-*, attachment). `format.go` has all terminal rendering helpers using lipgloss.
- **`cmd/epic/`**, **`cmd/project/`** — Epic CRUD + children, and read-only project metadata (types, statuses, priorities).
- **`cmd/skill.go`** + **`internal/skill/`** — `jira8 skill install|show|path`. `SKILL.md` is embedded with `go:embed`, so the installed skill always matches the binary; the `skill` subtree skips config loading.
- **`internal/client/jira.go`** — Single HTTP client with Bearer or Basic auth, 15s timeout, 429 retry (up to 3 attempts), and Jira error parsing into `APIError`. All API methods go through the private `do()` helper. `BuildJQL()` builds the `issue list` query.
- **`internal/markup/`** — Markdown ↔ Wiki Markup converters behind `--markdown`. `WikiToMarkdown` is line-based with a fence state machine; inline code, links, mentions and images are parked behind placeholders before the emphasis passes. Test against real PHO text when changing it: Jira content is far messier than the unit cases.
- **`internal/config/`** — Viper-based config loading: flags > env vars > `~/.jira.yaml`.
- **`internal/models/`** — Jira API request/response structs. `EditIssueRequest.Fields` is `map[string]any` for partial updates.

### Jira Server 8 specifics

- User references use `{"name": "username"}`, **not** `accountId` (that's Jira Cloud).
- Description is plain text / wiki markup, **not** ADF.
- Auth supports both **Bearer token** (`token` field) and **Basic Auth** (`user` + `password` fields). Basic Auth is the default for Jira Server without PAT enabled.
- Transitions require two calls: GET transitions to resolve name→ID, then POST.
- `--assignee me` in list uses JQL `currentUser()` (no extra API call); in create/edit it calls `/rest/api/2/myself` to resolve the username.

## Development rules

### Keep the skill in step with the CLI (mandatory)

`internal/skill/SKILL.md` is what agents read instead of `--help`. A command,
flag or behaviour that agents should use but the skill does not mention is, for
them, missing.

- When adding or changing a subcommand, flag, JSON shape or limitation, update
  `SKILL.md` in the same commit/PR.
- Every command the skill shows must run as written: check its flags against
  `jira8 <cmd> --help`.
- Keep it short and generic (no instance URLs, project keys or people): it is
  public and installed on other people's machines.

### Other

- `-o json` on read commands is a contract for scripts (e.g. a mail client parses
  `issue view --with-history -o json --markdown`): don't reshape it silently.
- `--help` text and `Example:` are the in-terminal docs: every command has an
  example.

## Git conventions

- **No `Co-Authored-By: Claude` trailers** in commit messages.
