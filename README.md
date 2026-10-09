# jira8

CLI tool for interacting with Jira Server 8 REST API v2. It ships with an agent
skill so coding agents (Claude Code and others) can drive it from the shell.

## Quick Setup

Requires [Go](https://go.dev/) and [Task](https://taskfile.dev/).

```bash
task setup
```

This will:

1. Build the binary
2. Install it to `$PREFIX/bin`: `~/.local/bin` by default (`~/bin` on Windows).
   Override with `PREFIX=/opt/tools task setup`
3. Create `~/.jira.yaml` from the example template

Then edit `~/.jira.yaml` with your credentials and verify:

```bash
task verify
```

### Available tasks

| Task | Description |
|------|-------------|
| `task setup` | Full setup: build + install + create config |
| `task build` | Build the binary |
| `task install` | Build (with version info) and copy to `$PREFIX/bin` |
| `task setup-config` | Copy config template to `~/.jira.yaml` |
| `task verify` | Check binary and config are OK |
| `task lint` | Run gofmt + go vet |
| `task test` | Run all tests |
| `task clean` | Remove built binary |

### Manual installation

```bash
go build -o jira8 .
cp jira8 ~/bin/       # or ~/.local/bin/ on Linux
```

## Configuration

Create `~/.jira.yaml`:

```yaml
# Basic Auth (user + password)
url: https://jira.example.com/jira
user: your.username
password: your.password
project: MYPROJ
```

Or with a Personal Access Token (if enabled on your Jira instance):

```yaml
url: https://jira.example.com/jira
token: <your-personal-access-token>
project: MYPROJ
```

### Configuration priority (highest to lowest)

1. CLI flags (`--url`, `--token`, `--project`; `--config` picks another file)
2. Environment variables
3. `~/.jira.yaml`

### Environment variables

| Variable | Description |
|----------|-------------|
| `JIRA_URL` | Jira server URL |
| `JIRA_USER` | Username for Basic Auth |
| `JIRA_PASSWORD` | Password for Basic Auth |
| `JIRA_TOKEN` | Personal Access Token (Bearer auth) |
| `JIRA_PROJECT` | Default project key |

You can override the default project per-command or per-session:

```bash
jira8 issue list --project OTHER                  # per-command
JIRA_PROJECT=OTHER jira8 issue list               # per-command via env
export JIRA_PROJECT=OTHER && jira8 issue list     # per-session
```

## Usage

### List issues

```bash
jira8 issue list                              # all issues in default project
jira8 issue list --status "In Progress"       # filter by status
jira8 issue list --assignee me                # assigned to current user
jira8 issue list --type Story                 # filter by issue type
jira8 issue list --epic MYPROJ-42             # issues linked to this Epic
jira8 issue list --jql "project = MYPROJ AND priority = High"
jira8 issue list --max 100                    # up to 100 results
```

### View issue

```bash
jira8 issue view MYPROJ-123
jira8 issue view MYPROJ-123 -o json
jira8 issue view MYPROJ-123 --with-history       # + History: who changed which field, when
```

History values longer than 200 characters or spanning several lines (typically
description edits, which store the whole text every time) are trimmed to their
first line plus `… (N chars)`. With `-o json` they come back in full.

### Create issue

```bash
jira8 issue create --summary "Fix login bug" --type Bug
jira8 issue create --summary "New feature" --type Story --description "Details..." --assignee me --priority High
jira8 issue create --summary "Ingest worker" --type Story --epic-link MYPROJ-42   # link to Epic
jira8 issue create --summary "Q2 Refactor" --type Epic --epic-name "Q2 Refactor"  # Epic
jira8 issue create --summary "Write tests" --type Sub-task --parent MYPROJ-123    # Sub-task
```

Long descriptions and bodies can come from a file or stdin, and from Markdown;
see [Markdown and Wiki Markup](#markdown-and-wiki-markup).

### Edit issue

```bash
jira8 issue edit MYPROJ-123 --summary "Updated title"
jira8 issue edit MYPROJ-123 --assignee john.doe --priority Medium
jira8 issue edit MYPROJ-123 --assignee ""              # unassign
jira8 issue edit MYPROJ-123 --epic-link MYPROJ-42      # link to Epic
jira8 issue edit MYPROJ-123 --epic-link ""             # detach from Epic
jira8 issue edit MYPROJ-123 --due-date 2026-09-24 --original-estimate 5d
jira8 issue edit MYPROJ-123 --due-date ""              # clear the due date
```

`--due-date` sets Jira's `duedate` field (`YYYY-MM-DD`). `--original-estimate`
sets `timetracking.originalEstimate`, in Jira's own shorthand (`3d`, `5h`,
`30m`) — this is the up-front estimate, not time already spent; log actual
hours worked with `issue worklog-add` instead.

### Epics

Dedicated ergonomic subcommand for Epic CRUD and child listing. Custom field IDs
(`Epic Name`, `Epic Link`) are resolved dynamically from the Jira instance on
first use — no hardcoded `customfield_XXXXX` required.

```bash
jira8 epic list                                   # Epics in default project
jira8 epic list --status "In Progress"

jira8 epic view MYPROJ-42                         # Epic + its children
jira8 epic view MYPROJ-42 --no-children           # Epic only
jira8 epic children MYPROJ-42                     # just the children table

jira8 epic create --name "Q2 Refactor" --summary "Refactor billing pipeline"
jira8 epic create --name "Q2 Refactor" --summary "..." --description "..." --priority High

jira8 epic edit MYPROJ-42 --name "Q2 Refactor (rev 2)"
jira8 epic edit MYPROJ-42 --summary "New summary" --assignee me
```

### Transitions

```bash
jira8 issue transitions MYPROJ-123               # list available transitions
jira8 issue transition MYPROJ-123 --to "Done"    # perform transition
```

### Ranking (kanban order)

Ranking changes the **vertical** position of an issue inside a board column. It
never changes the status — moving an issue to another column is a transition.

```bash
jira8 issue rank MYPROJ-123 --top                       # first position of its column
jira8 issue rank MYPROJ-123 --bottom                    # last position of its column
jira8 issue rank MYPROJ-123 --before MYPROJ-99          # immediately above another issue
jira8 issue rank MYPROJ-123 --after MYPROJ-99           # immediately below another issue
jira8 issue rank MYPROJ-1 MYPROJ-2 --top --board 110    # a block, keeping relative order
```

`--top` / `--bottom` work out which column the issue currently sits in (from the
board's status mapping) and move it to that column's edge. When the project has
more than one board, pass `--board` with an ID or name; without it the error lists
the candidates:

```
Error: project MYPROJ has 5 boards, specify one: "Team Kanban" (110, kanban), ...
```

Up to 50 issues can be ranked in a single call. If the issues already occupy the
requested edge, the command reports it and sends nothing. Ranking an issue that is
already at the top is a no-change operation, not an error.

Ranking needs Jira Software (the Agile/Greenhopper plugin) — it uses the Agile API
(`/rest/agile/1.0`), unlike every other command, which lives on `/rest/api/2`. The
rank field is discovered per board, falling back to the instance-wide LexoRank
field, so no custom field ID has to be configured.

### Links

```bash
jira8 issue link-types                                   # list available link types
jira8 issue link MYPROJ-1 MYPROJ-2 --type "Relates"      # relate two issues
jira8 issue link MYPROJ-9 MYPROJ-10 --type "Blocks"      # MYPROJ-9 blocks MYPROJ-10
jira8 issue link A B --type "Relates" --comment "note"   # with an optional comment
```

The first key is the subject of the relation ("OUTWARD *phrase* INWARD"); for symmetric
types like `Relates` the order is irrelevant. `--type` defaults to `Relates`.

### Comments

```bash
jira8 issue comment-list MYPROJ-123                          # list comments
jira8 issue comment-add MYPROJ-123 --body "Looks good"       # add a comment
jira8 issue comment-edit MYPROJ-123 --id 84887 --body "Fixed typo"
jira8 issue comment-delete MYPROJ-123 --id 84887             # asks for confirmation; --yes skips it
```

### Worklogs

```bash
jira8 issue worklog-list MYPROJ-123                                  # list worklogs
jira8 issue worklog-add MYPROJ-123 --time 2h --comment "Investig."   # add a worklog
jira8 issue worklog-add MYPROJ-123 --time 30m --date 2026-04-15T09:00:00.000+0200
jira8 issue worklog-delete MYPROJ-123 --id 27705             # asks for confirmation; --yes skips it
```

### Attachments

```bash
jira8 issue attachment add MYPROJ-123 diag.png trace.log       # upload one or more files
jira8 issue attachment list MYPROJ-123                         # list attachments
jira8 issue attachment delete 45821                            # delete by attachment ID

# Or attach at create/edit time:
jira8 issue create --summary "Crash on login" --type Bug \
    --attach screenshot.png --attach trace.log
jira8 issue edit MYPROJ-123 --attach extra-evidence.pdf        # attach without touching other fields
```

Uploads stream from disk so large files do not get buffered in memory. The
upload step is separate from create/edit (Jira's `/issue` endpoint does not
accept files) and is not transactional: if the issue is created but the upload
fails, you get back the new key and the upload error so you can retry with
`issue attachment add`.

### Project metadata

Query valid values for issue types, statuses, and priorities:

```bash
jira8 project types                       # issue types available for creation
jira8 project statuses                    # statuses grouped by issue type
jira8 project priorities                  # global priority levels
jira8 project types --project OTHER         # for a different project
```

### Output format

All commands support `--output json` (or `-o json`) for machine-readable output.

### Markdown and Wiki Markup

Jira Server 8 stores descriptions, comments and worklog comments as Wiki Markup.
jira8 converts in both directions so you can work in Markdown:

```bash
# Write: Markdown in, Wiki Markup sent to Jira
jira8 issue create --summary "Spike" --type Task --markdown --description-file spike.md
jira8 issue comment-add MYPROJ-123 --markdown --body "**Done**, see \`make test\`"
generate-report | jira8 issue comment-add MYPROJ-123 --markdown --body-file -   # from stdin

# Read: Wiki Markup from Jira, Markdown out
jira8 issue view MYPROJ-123 --markdown -o json
jira8 issue comment-list MYPROJ-123 --markdown
```

`--description-file`, `--body-file` and `--comment-file` read the text from a
file, or from stdin with `-`.

Reading produces GitHub-flavoured Markdown and never HTML. What converts:

| Wiki Markup | Markdown |
|-------------|----------|
| `h1.` … `h6.` | `#` … `######` |
| `*bold*`, `_italic_`, `-strike-`, `{{code}}` | `**bold**`, `*italic*`, `~~strike~~`, `` `code` `` |
| `{code:lang}` / `{noformat}` blocks, also when the tag shares a line with text | fenced blocks, content untouched |
| `{quote}`, `bq.` | `>` |
| `*` / `#` lists, nested | `-` / `1.` lists, two spaces per level |
| `\|\|head\|\|` tables, also without a header row | GFM tables (an empty header is added when missing) |
| `[text\|url]`, `[url]`, `[MYPROJ-1]`, `[~user]` | `[text](url)`, `<url>`, `MYPROJ-1`, `@user` |
| `!image.png!`, `!image.png\|thumbnail!` | `![image.png](image.png)` (attachment name, not a URL) |
| `{color}`, `{panel}`, `{anchor}` | removed, their text kept |

Emoticons, `+underline+`, `^sup^`, `~sub~`, `{toc}` and other macros stay
literal. History values (`--with-history`) are never converted.

## Agent skill

The binary embeds a skill document (`SKILL.md`) that teaches a coding agent how
to use jira8: commands, `-o json` output, Markdown conversion, the change
history and the Jira Server 8 pitfalls. Install it where the agent looks for
skills:

```bash
jira8 skill install              # ~/.claude/skills/jira8/SKILL.md
jira8 skill install --project    # ./.claude/skills/jira8/SKILL.md
jira8 skill show                 # print it
```

`task install` installs the skill together with the binary. After upgrading
jira8 by other means, run `jira8 skill install --force` so the skill matches
the binary. No Jira configuration is needed for these commands.

Any agent with a shell can use jira8 this way; the skill is plain Markdown, so
agents without a skills directory can be pointed at `jira8 skill show`.

### Migrating from v1 (MCP server removed)

v2 drops `jira8 mcp serve` and its tools, resources and prompts: every
capability was already a CLI subcommand, agents with a shell use the CLI, and
keeping two surfaces in sync cost more than it gave. Remove the server from
your client (`claude mcp remove jira`, or the `jira8` entry under `mcpServers`)
and install the skill. If you need the MCP server, stay on
[v1.9.0](https://github.com/carlosprados/jira8/releases/tag/v1.9.0).

## Target

- Jira Server 8.7.1
- REST API v2, plus the Agile API (`/rest/agile/1.0`) for board ranking
- Authentication: Basic Auth (user:password) or Personal Access Token (Bearer)
- Epic and ranking commands additionally require Jira Software (Agile/Greenhopper)

## License

[MIT](LICENSE) © Carlos Javier Prados Hijón
