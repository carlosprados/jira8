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

This is a Go CLI tool targeting **Jira Server 8.7.1 REST API v2**, plus the Agile API (`/rest/agile/1.0`) for board ranking only. It is a CLI only, by design: agents drive it from the shell, guided by the embedded skill (`internal/skill/SKILL.md`).

### Key layers

- **`cmd/root.go`** — Cobra root command. `PersistentPreRunE` loads config, creates the HTTP client, and stores both in `cmd/app.State` (a package-level singleton to avoid circular imports between `cmd` and `cmd/issue`).
- **`cmd/app/`** — Shared state holder (`State` struct with Config, Client, Output; all subcommands access it via `app.Get()`) plus shared helpers: text input from flag/file/stdin, `OutputJSON` and `Done` (every Jira command answers `-o json`, writes too), `Confirm` (deletes: no TTY without `--yes` is an error), `ProjectKey`, `CreateFields`/`EditFields` (`--labels` and `--field`, checked against the create/edit screen from createmeta/editmeta), `WorklogStarted`/`JiraDuration` (lenient dates and durations), name matching, Markdown rendering of read results and `TrimHistory`.
- **`cmd/issue/`** — One file per subcommand (list, view, create, edit, transition(s), rank, link, links + link-delete, comment-*, worklog-*, attachment). `format.go` has all terminal rendering helpers using lipgloss.
- **`cmd/epic/`**, **`cmd/project/`**, **`cmd/user/`** — Epic CRUD + children, read-only project metadata (types, statuses, priorities, create-screen fields), and user lookup (`user search|me`).
- **`cmd/board/`** + **`internal/client/board.go`** — `board list|view|flow`. `BoardView` queries each column with `filter = <board filter> AND (<kanban subQuery>) AND status IN (<column statuses>) ORDER BY cf[<rank>] ASC`, plus a sub-task-free count when the board's `constraintType` is `issueCountExclSubs`. Board facts (rank field fallback, several boards per project) are shared with `rank.go`. `--stale` adds `NOT status CHANGED AFTER -Nd AND created < -Nd` on columns with an `indeterminate`-category status. `issue move` (cmd/issue/move.go) = `ResolveColumn` + the one transition into the column's statuses (`performTransition`, shared with `transition`) + rank with `RankRequest.StatusID` set to the target, so a dry run ranks against the new column. `--by-assignee` (`BoardViewRequest.ByAssignee`) fetches every card of the in-progress columns and `cmd/board/wip.go` groups them. `board flow` (`internal/client/flow.go`) searches the board filter (no sub-filter) with `expand=changelog` for issues that entered the last column in the period; `issueFlow` replays the status changes over the columns. History JQL clauses (`CHANGED TO`) need bare status ids: quoted ones are read as names.
- **`cmd/exit.go`** — exit codes (`ExitCode`, documented in README and skill: do not renumber). Usage errors are detected in `Execute` because every command hook sets `SilenceUsage`; `strictGroups` makes unknown subcommands of a group a usage error instead of help + exit 0. Cobra checks required and mutually exclusive flags after the hooks, so `Execute` re-runs those checks to classify them as usage errors. Network failures wrap `client.ErrUnreachable`.
- **`cmd/open.go`** — `jira8 open KEY`; issue URLs come from `client.BrowseURL`, which also fills `url` in issues and create/edit answers.
- **`cmd/config.go`**, **`cmd/ping.go`** — `config show` uses `config.Read` (no validation, own hook) so it works with a broken config; `config.Load` = `Read` + `Validate`. `ping` calls `/serverInfo`.
- **`cmd/markup.go`** — `jira8 markup md2wiki|wiki2md`: the `--markdown` converters, offline; like `skill`, it skips config loading.
- **`--dry-run`** (root persistent flag) — `client.SetDryRun` makes `doAt` and the multipart upload record every non-GET request instead of sending it (a planned create answers key `NEW`). Root captures the command's stdout in a temp file and `Execute` prints the plan instead (`app.PrintPlan`), or replays the output when nothing was planned (reads). `app.Confirm` lets deletes through.
- **`cmd/skill.go`** + **`internal/skill/`** — `jira8 skill install|show|path`. `SKILL.md` is embedded with `go:embed`, so the installed skill always matches the binary; the `skill` subtree skips config loading.
- **`internal/client/jira.go`** — Single HTTP client with Bearer or Basic auth, 15s timeout, 429 retry (up to 3 attempts), and Jira error parsing into `APIError`. All API methods go through the private `do()` helper. `BuildJQL()` builds the `issue list` query.
- **`internal/markup/`** — Markdown ↔ Wiki Markup converters behind `--markdown`. `WikiToMarkdown` is line-based with a fence state machine; inline code, links, mentions and images are parked behind placeholders before the emphasis passes. Writing escapes what Jira's renderer would misread, inside `{{…}}` too. Check changes against Jira's own renderer (`POST /rest/api/1.0/render` with `rendererType: atlassian-wiki-renderer`) and against real issue text: Jira content is far messier than the unit cases.
- **`internal/config/`** — Viper-based config loading: flags > env vars > `~/.jira.yaml`.
- **`internal/models/`** — Jira API request/response structs. `EditIssueRequest.Fields` is `map[string]any` for partial updates.

### Jira Server 8 specifics

- User references use `{"name": "username"}`, **not** `accountId` (that's Jira Cloud).
- Description is plain text / wiki markup, **not** ADF.
- Auth supports both **Bearer token** (`token` field) and **Basic Auth** (`user` + `password` fields). Basic Auth is the default for Jira Server without PAT enabled.
- Transitions require two calls: GET transitions to resolve name→ID, then POST.
- `--assignee me` in list uses JQL `currentUser()` (no extra API call); in create/edit it calls `/rest/api/2/myself` to resolve the username.

## Development rules

### Doctrine: the CLI and the skill are twins

`internal/skill/SKILL.md` is what agents read instead of `--help`. A command,
flag or behaviour the skill does not mention does not exist for them, and a
command it shows that no longer runs misleads them. So:

- **Every change to the CLI updates `SKILL.md` in the same commit**: new or
  renamed commands and flags, JSON shapes, formats, errors, limitations. No
  exceptions, no "docs later".
- `go test ./cmd/` enforces it (`cmd/skill_doctrine_test.go`): every command
  and flag must appear in the skill, and every `jira8 …` line in the skill must
  parse against the real command tree. A deliberate omission goes in
  `notForAgents` with its reason; keep that list short.
- The test cannot judge meaning: when behaviour changes, reread the affected
  skill section and fix it by hand, then run the commands for real against a
  test project.
- Keep it short and generic (no instance URLs, project keys or people): it is
  public and installed on other people's machines.

### Other

- `-o json` on read commands is a contract for scripts (e.g. a mail client parses
  `issue view --with-history -o json --markdown`): don't reshape it silently.
- `--help` text and `Example:` are the in-terminal docs: every command has an
  example.

## Git conventions

- **No `Co-Authored-By: Claude` trailers** in commit messages.
