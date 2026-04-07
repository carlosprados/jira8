# jira-cli

CLI tool for interacting with Jira Server 8 REST API v2, with MCP server support for AI agent integration.

## Installation

```bash
go build -o jira-cli .
```

## Configuration

Create `~/.jira.yaml`:

```yaml
url: https://jira.amplia.es/jira
token: <your-personal-access-token>
project: ESA
```

Configuration priority (highest to lowest):
1. CLI flags (`--url`, `--token`, `--project`)
2. Environment variables (`JIRA_URL`, `JIRA_EMAIL`, `JIRA_TOKEN`)
3. `~/.jira.yaml`

## Usage

### List issues

```bash
jira issue list                              # all issues in default project
jira issue list --status "In Progress"       # filter by status
jira issue list --assignee me                # assigned to current user
jira issue list --jql "project = ESA AND priority = High"
jira issue list --max 100                    # up to 100 results
```

### View issue

```bash
jira issue view ESA-123
jira issue view ESA-123 -o json
```

### Create issue

```bash
jira issue create --summary "Fix login bug" --type Bug
jira issue create --summary "New feature" --type Story --description "Details..." --assignee me --priority High
```

### Edit issue

```bash
jira issue edit ESA-123 --summary "Updated title"
jira issue edit ESA-123 --assignee john.doe --priority Medium
jira issue edit ESA-123 --assignee ""        # unassign
```

### Transitions

```bash
jira issue transitions ESA-123               # list available transitions
jira issue transition ESA-123 --to "Done"    # perform transition
```

### Output format

All commands support `--output json` (or `-o json`) for machine-readable output.

## MCP Server

Start as an MCP server (stdio transport) for AI agent integration:

```bash
jira mcp serve
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
      "command": "/path/to/jira-cli",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Target

- Jira Server 8.7.1
- REST API v2
- Authentication: Personal Access Token (Bearer)
