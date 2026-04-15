# jira8

CLI tool for interacting with Jira Server 8 REST API v2, with MCP server support for AI agent integration.

## Quick Setup

Requires [Go](https://go.dev/) and [Task](https://taskfile.dev/).

```bash
task setup
```

This will:

1. Build the binary
2. Install it to `~/bin` (Windows) or `~/.local/bin` (Linux)
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
| `task install` | Build and copy to user PATH |
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

1. CLI flags (`--url`, `--token`, `--project`)
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
./jira8 issue list --project OTHER                # per-command
JIRA_PROJECT=OTHER ./jira8 issue list             # per-command via env
export JIRA_PROJECT=OTHER && ./jira8 issue list   # per-session
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
```

### Create issue

```bash
jira8 issue create --summary "Fix login bug" --type Bug
jira8 issue create --summary "New feature" --type Story --description "Details..." --assignee me --priority High
jira8 issue create --summary "Ingest worker" --type Story --epic-link MYPROJ-42   # link to Epic
jira8 issue create --summary "Q2 Refactor" --type Epic --epic-name "Q2 Refactor"  # Epic
```

### Edit issue

```bash
jira8 issue edit MYPROJ-123 --summary "Updated title"
jira8 issue edit MYPROJ-123 --assignee john.doe --priority Medium
jira8 issue edit MYPROJ-123 --assignee ""              # unassign
jira8 issue edit MYPROJ-123 --epic-link MYPROJ-42      # link to Epic
jira8 issue edit MYPROJ-123 --epic-link ""             # detach from Epic
```

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

### Comments

```bash
jira8 issue comment-list MYPROJ-123                          # list comments
jira8 issue comment-add MYPROJ-123 --body "Looks good"       # add a comment
```

### Worklogs

```bash
jira8 issue worklog-list MYPROJ-123                                  # list worklogs
jira8 issue worklog-add MYPROJ-123 --time 2h --comment "Investig."   # add a worklog
jira8 issue worklog-add MYPROJ-123 --time 30m --date 2026-04-15T09:00:00.000+0200
```

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

## MCP Server

Start as an MCP server (stdio transport) for AI agent integration:

```bash
jira8 mcp serve
```

### Available MCP tools

| Tool | Description |
|------|-------------|
| `jira_list_issues` | List issues (supports `issue_type`, `epic`, JQL, etc.) |
| `jira_get_issue` | Get issue details |
| `jira_create_issue` | Create a new issue (supports `epic_name`, `epic_link`) |
| `jira_edit_issue` | Edit an existing issue (supports `epic_name`, `epic_link`) |
| `jira_transition_issue` | Transition an issue |
| `jira_list_transitions` | List available transitions |
| `jira_add_comment` | Add a comment |
| `jira_list_comments` | List comments |
| `jira_add_worklog` | Add a worklog entry |
| `jira_list_worklogs` | List worklog entries |
| `jira_list_issue_types` | List issue types for a project |
| `jira_list_statuses` | List statuses grouped by issue type |
| `jira_list_priorities` | List available priorities |
| `jira_list_epics` | List Epics in a project |
| `jira_list_epic_children` | List issues linked to an Epic |
| `jira_create_epic` | Create an Epic (shortcut for `jira_create_issue` with `issue_type=Epic`) |
| `jira_edit_epic` | Edit an Epic (exposes friendly `name` for Epic Name) |
| `jira_view_epic` | Get an Epic and (optionally) its linked children in one call |

### Claude Code integration

Add to your Claude Code MCP settings:

```json
{
  "mcpServers": {
    "jira": {
      "command": "/path/to/jira8",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Target

- Jira Server 8.7.1
- REST API v2
- Authentication: Basic Auth (user:password) or Personal Access Token (Bearer)
