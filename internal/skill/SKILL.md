---
name: jira8
description: Read and change issues in a Jira Server 8 / Data Center instance from the shell with the `jira8` CLI — list and search (JQL), view with comments and change history, create, edit, transition, rank on a kanban board, link, comment, log work, attach files, manage Epics, and query a project's valid types, statuses and priorities. Load whenever the task mentions Jira, an issue key (ABC-123), tickets, bugs, stories, epics, sprints or kanban columns, worklogs or "imputar horas" in Jira, or turning notes into Jira issues. Use INSTEAD of calling the Jira REST API by hand.
---

# jira8 — Jira Server 8 from the shell

`jira8` talks to a Jira **Server/Data Center 8** instance (REST API v2) with the
user's own credentials (`~/.jira.yaml` or `JIRA_*` env vars). Everything is a
CLI subcommand; there is no MCP server.

```sh
jira8 --help · jira8 <group> <cmd> --help     # every command has examples
jira8 skill show                               # this document
```

Pass **`-o json`** whenever you will read the output. Text output is for humans,
is coloured and may change. JSON is the raw Jira payload (`key`, `self`,
`fields.*`), so field names follow the Jira REST API.

## Find out before you write

Never guess names: Jira rejects unknown values and each project differs.

```sh
jira8 project types --project ABC -o json        # valid issue types for create
jira8 project statuses --project ABC -o json     # statuses per issue type
jira8 project priorities -o json
jira8 issue transitions ABC-123 -o json          # transitions valid NOW for this issue
jira8 issue link-types -o json
```

The default project comes from config (`project:`); pass `--project` when the
task names another one.

## Read

```sh
jira8 issue list --status "In Progress" --assignee me -o json
jira8 issue list --type Bug --max 100 -o json
jira8 issue list --jql 'project = ABC AND updated >= -7d ORDER BY updated DESC' -o json
jira8 issue view ABC-123 -o json                 # includes fields.comment.comments[]
jira8 issue view ABC-123 --with-history -o json  # + changelog.histories[]
jira8 issue comment-list ABC-123 -o json
jira8 issue worklog-list ABC-123 -o json
jira8 epic view ABC-42 -o json                   # Epic + children (--no-children to skip)
jira8 epic children ABC-42 -o json
```

- `--jql` overrides every other filter of `issue list`. `--max` defaults to 50.
- `--with-history` returns the full changelog. Description edits store the
  whole text each time: one issue can carry ~1 MB of history. In JSON nothing
  is trimmed, so extract what you need with `jq` instead of reading it whole:
  `jq '[.changelog.histories[] | {created, who: .author.displayName, items: [.items[] | select(.field != "description") | {field, fromString, toString}]}]'`.
  The text output already trims long values to one line.
- `-o json` on `issue view` is large when the description is large. Prefer
  `jq` projections (`{key, summary: .fields.summary, status: .fields.status.name}`).

## Markdown ↔ Wiki Markup

Jira Server 8 stores descriptions, comments and worklog comments as **Wiki
Markup**, not Markdown and not ADF. Add `--markdown`:

- **on write commands** to send Markdown: it is converted to Wiki Markup;
- **on read commands** to get Markdown back (GitHub-flavoured, never HTML).

Without the flag, write commands send the text verbatim, so Markdown would show
up literally in Jira (`**bold**` stays with asterisks).

Reading converts headings, emphasis, inline code, `{code}`/`{noformat}` blocks,
quotes, lists, tables, links, `[~user]` (→ `@user`) and images (→ attachment
name, not a URL); `{color}`, `{panel}`, `{anchor}` are dropped. Emoticons,
`+underline+`, `{toc}` and other macros stay literal. **History values are
never converted.**

For long text use a file or stdin instead of a quoted argument:

```sh
jira8 issue create --project ABC --type Task --summary "Spike: cache" --markdown --description-file spike.md
cat notes.md | jira8 issue comment-add ABC-123 --markdown --body-file -
```

`--description-file`, `--body-file` and `--comment-file` accept `-` for stdin.

## Write

```sh
jira8 issue create --project ABC --type Bug --priority High --summary "…" --markdown --description-file bug.md
jira8 issue create --project ABC --type Sub-task --parent ABC-123 --summary "…"
jira8 issue create --project ABC --type Story --epic-link ABC-42 --summary "…" --attach log.txt
jira8 issue edit ABC-123 --assignee me --priority Major
jira8 issue edit ABC-123 --assignee ""           # unassign ("" clears: also --epic-link, --due-date)
jira8 issue edit ABC-123 --due-date 2026-11-30 --original-estimate 3d
jira8 issue transition ABC-123 --to "In Progress"
jira8 issue comment-add ABC-123 --markdown --body "…"
jira8 issue comment-edit ABC-123 --id 84887 --markdown --body "…"
jira8 issue worklog-add ABC-123 --time 2h --comment "…"        # --date for past work
jira8 issue link ABC-1 ABC-2 --type Blocks       # ABC-1 blocks ABC-2
jira8 issue attachment add ABC-123 a.png b.log
```

- `issue edit` only sends the flags you pass; everything else is untouched.
- `--assignee me` resolves the current user. Users are Jira **usernames**
  (`{"name": …}`), not Cloud `accountId`s.
- `--original-estimate` is the up-front estimate. Time actually spent goes in
  `worklog-add`.
- `transition --to` takes the **transition** name shown by `issue transitions`,
  which is not always the target status name. On a miss, the error lists the
  valid ones.
- `issue create` uploads attachments after creating the issue. If the upload
  fails, the issue still exists: the error gives its key, retry with
  `issue attachment add`.

## Kanban order

```sh
jira8 issue rank ABC-123 --top                   # top of the column it is already in
jira8 issue rank ABC-123 --before ABC-99         # or --after, --bottom
jira8 issue rank ABC-1 ABC-2 --top --board "Team Kanban"
```

Ranking changes the vertical position inside a column, never the status (that
is a transition). It needs Jira Software. When the project has several boards,
`--top`/`--bottom` require `--board` (id or name); the error lists them.

## Epics

`epic list|view|children|create|edit` resolve the Epic Name / Epic Link custom
fields by themselves; never hardcode `customfield_…` ids.

```sh
jira8 epic create --project ABC --name "Billing v2" --summary "Billing v2" --markdown --description-file epic.md
jira8 epic edit ABC-42 --name "Billing v2 (rev)"
```

## Destructive commands

`comment-delete` and `worklog-delete` ask for confirmation on the terminal.
Without one (your case) they read EOF and abort, so pass `--yes`, and only
after the user confirmed that exact deletion. **`attachment delete` does not
ask**: it deletes immediately. Nothing deleted can be recovered.

## Not possible with jira8

- Moving an issue to another project: Jira Server 8 has no REST endpoint for it.
  It must be done in the web UI.
- Deleting or cloning issues, sprints, filters, dashboards, admin settings.

## Errors

Jira errors read `Error: Jira API error (HTTP n): message; field: message` and
exit 1. A `400` naming a field usually means an invalid name: check
`project types|statuses|priorities` or `issue transitions`. `401` is bad
credentials, `403` no permission, and `404` "Issue Does Not Exist" may just mean
the issue is not visible to the user. `429` (rate limit) is retried
automatically. Only a wrong flag or argument prints the command usage.
