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
```

### Edit issue

```bash
jira8 issue edit MYPROJ-123 --summary "Updated title"
jira8 issue edit MYPROJ-123 --assignee john.doe --priority Medium
jira8 issue edit MYPROJ-123 --assignee ""        # unassign
```

### Transitions

```bash
jira8 issue transitions MYPROJ-123               # list available transitions
jira8 issue transition MYPROJ-123 --to "Done"    # perform transition
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
| `jira_list_issues` | List issues with filters or JQL |
| `jira_get_issue` | Get issue details |
| `jira_create_issue` | Create a new issue |
| `jira_edit_issue` | Edit an existing issue |
| `jira_transition_issue` | Transition an issue |
| `jira_list_transitions` | List available transitions |

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
