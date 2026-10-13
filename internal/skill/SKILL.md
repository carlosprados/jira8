---
name: jira8
description: Read and change issues in a Jira Server 8 / Data Center instance from the shell with the `jira8` CLI — list and search (JQL), view with comments and change history, create, edit, transition, rank on a kanban board, link, assign, watch, comment, log work, attach files, manage Epics, and query a project's valid types, statuses and priorities. Load whenever the task mentions Jira, an issue key (ABC-123), tickets, bugs, stories, epics, sprints or kanban columns, worklogs or "imputar horas" in Jira, or turning notes into Jira issues. Use INSTEAD of calling the Jira REST API by hand.
---

# jira8 — Jira Server 8 from the shell

`jira8` talks to a Jira **Server/Data Center 8** instance (REST API v2) with the
user's own credentials.

```sh
jira8 --help · jira8 <group> <cmd> --help     # every command has examples
jira8 skill show                               # this document
```

Configuration lives in `~/.jira.yaml` (or `JIRA_URL`, `JIRA_TOKEN`,
`JIRA_USER`/`JIRA_PASSWORD`, `JIRA_PROJECT`): `url` and credentials are
required, `project` is the default for commands that need one. Never edit it or
ask for credentials yourself: if it is missing or wrong, tell the user.

## Output

Pass **`-o json`** whenever you will read the output. Text output is for humans,
is coloured and may change. Every Jira command honours `-o json`, writes included.

| Command | JSON |
|---|---|
| `issue list`, `epic list`, `epic children` | array of issues (`key`, `url`, `fields.*`) |
| `issue view` | one issue; `fields.comment.comments[]`, `changelog` with `--with-history` |
| `comment-list`, `worklog-list`, `attachment list` | array (`id`, `body` / `timeSpent`, `started`, `comment` / `filename`) |
| `issue create`, `epic create` | `{id, key, self, url}` (+ `attachments[]`) |
| `comment-add`, `comment-edit`, `worklog-add`, `attachment add` | the created object(s), with their `id` |
| `issue edit`, `epic edit` | `{key, url, updated: [field ids]}` (+ `attachments[]`) |
| `transition` | `{key, transition, status}` (+ `updated`, `comment` id, `attachments[]`) |
| `link` | `{outward, type, inward}` |
| `assign` / `watch`, `unwatch` | `{key, assignee}` / `{key, watcher, watching}` |
| `watchers` | `{isWatching, watchCount, watchers: [{name, displayName…}]}` |
| `issue links` | array of `{id, type, direction, phrase, key, summary, status}`, seen from that issue |
| `user search`, `user me` | array / one `{name, key, displayName, emailAddress, active}` |
| `project fields`, `issue fields` | array of `{id, name, required, type, items?, custom?, operations?, allowed?}` |
| `*-delete`, `attachment delete` | `{…, id, deleted: true}` |
| any write with `--dry-run` | `{dry_run: true, requests: [{method, path, body?, files?}]}` |
| `markup md2wiki`, `markup wiki2md` | `{wiki}` / `{markdown}` |
| `open` | `{key, url}` (does not launch a browser) |
| `ping` | `{baseUrl, version, deploymentType, serverTitle, ms}` |
| `config show` | `{file, auth, values: {url, project, user, token, password: {value?, set, source}}, problems: []}` |

Issues are the Jira REST payload, so field names follow the Jira API
(`fields.status.name`, `fields.assignee.name`…), plus `url`: the issue in the
browser, the link to give the user (`self` is the API address).
`jira8 open ABC-123` opens it in the user's browser. Chain steps with `jq`:

```sh
KEY=$(jira8 issue create --project ABC --type Task --summary "…" -o json | jq -r .key)
```

## Find out before you write

Never guess names: Jira rejects unknown values and each project differs.

```sh
jira8 project types --project ABC -o json        # issue types; subtask:true needs --parent
jira8 project statuses --project ABC -o json     # statuses per issue type
jira8 project priorities -o json
jira8 issue transitions ABC-123 -o json          # transitions valid NOW, with the fields their screens ask
jira8 issue link-types -o json
jira8 project fields --project ABC --type Bug -o json   # fields on the create screen + allowed values
jira8 issue fields ABC-123 -o json               # fields you can edit on this issue
jira8 user search "garcia" --project ABC -o json # usernames assignable in ABC (name, email fragment)
jira8 user me -o json                            # who jira8 is logged in as
```

People are given by **username** (`name` in `user search`), never by display
name: resolve "assign it to Ana" with `user search ana` first, and ask the user
when several match. `--max` (default 20) caps the search.

## Read

```sh
jira8 issue list --status "In Progress" --assignee me -o json
jira8 issue list --type Bug --max 100 -o json
jira8 issue list --label backend --label urgent --reporter me --updated-after -7d -o json
jira8 issue list --text "timeout" --component API --priority Major --created-after 2026-10-01 -o json
jira8 issue list --status Done --sort created --asc --all -o json
jira8 issue list --fields labels,components,customfield_10011 -o json
jira8 issue list --jql 'project = ABC AND updated >= -7d ORDER BY updated DESC' -o json
jira8 issue view ABC-123 -o json
jira8 issue view ABC-123 --with-history -o json  # + changelog.histories[]
jira8 issue comment-list ABC-123 -o json
jira8 issue worklog-list ABC-123 -o json
jira8 issue attachment list ABC-123 -o json      # id, filename, size, content URL
jira8 issue links ABC-123 -o json                # "blocks ABC-9", "is blocked by ABC-7"…
jira8 epic view ABC-42 -o json                   # {epic, children} (--no-children to skip)
jira8 epic children ABC-42 -o json
```

- `issue links` reads each link from the issue's side: `phrase` is what this
  issue does to `key` (`direction: outward`) or suffers from it (`inward`).
  `link-delete` answers which link it removed: `{id, outward, type, inward, deleted}`.
- `issue list` filters combine with AND; `--label` repeats (all of them).
  `--text` searches summary, description and comments. Dates take
  `YYYY-MM-DD` or a relative period (`-7d`, `-2w`, `-12h`).
- `--jql` overrides every other filter of `issue list`; `--sort`/`--asc`
  replace its `ORDER BY` only when given. `--sort`: `updated` (default),
  `created`, `priority`, `key`, `rank`; newest/highest first, `--asc` for the
  reverse (board order with `rank`).
- `--max` defaults to 50; `--all` fetches every match. When more issues match
  than are returned, stderr says `Showing N of M issues` (the JSON array stays
  as is): do not assume a list is complete without checking it.
- `issue list` fetches a fixed set of fields (summary, status, assignee,
  priority, type, project, dates). `--fields` adds more by **id** (`labels`,
  `components`, `fixVersions`, `customfield_…`; ids from `project fields`).
- Without `--project` and without a configured project, `issue list` searches
  every project.
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

Writing converts headings, emphasis, code (inline and fenced, with language),
quotes, nested and mixed lists, tables and links, and escapes what Jira would
misread (`{…}`, `[…]`, braces inside inline code). To mention someone write
`[~username]`: it is sent as is. `@username` is not converted.

**Images**: with `--markdown`, a local image `![alt](shot.png)` is uploaded
as an attachment of the issue and becomes `!shot.png|alt=alt!`, so it renders
in Jira.
Relative paths resolve against the Markdown file (`--description-file`,
`--body-file`), or the working directory for inline text and stdin. Every file
is checked before anything is written; two different files with the same name
are an error. Re-sending the same text does not attach an image again (same
name and size). Image URLs (`https://…`) are linked, not uploaded. JSON output
of create/edit lists the uploads in `attachments[]`.

Reading converts the same back, plus `{noformat}`, `[~user]` (→ `@user`) and
images (→ attachment name, not a URL); `{color}`, `{panel}`, `{anchor}` are
dropped. Emoticons, `+underline+`, `{toc}` and other macros stay literal.
**History values are never converted.**

For long text use a file or stdin instead of a quoted argument:

```sh
jira8 issue create --project ABC --type Task --summary "Spike: cache" --markdown --description-file spike.md
cat notes.md | jira8 issue comment-add ABC-123 --markdown --body-file -
```

`--description-file`, `--body-file` and `--comment-file` accept `-` for stdin.

To see a conversion without touching Jira (no config needed):

```sh
jira8 markup md2wiki spec.md                     # what --markdown would send
jira8 issue view ABC-123 -o json | jq -r .fields.description | jira8 markup wiki2md
```

`md2wiki` does not upload images: it only shows the `!name|alt=…!` they become.

## Write

```sh
jira8 issue create --project ABC --type Bug --priority Major --summary "…" --markdown --description-file bug.md
jira8 issue create --project ABC --type Sub-task --parent ABC-123 --summary "…"
jira8 issue create --project ABC --type Story --epic-link ABC-42 --summary "…" --attach log.txt
jira8 issue edit ABC-123 --assignee me --priority Critical
jira8 issue edit ABC-123 --assignee ""           # unassign ("" clears: also --epic-link, --due-date)
jira8 issue assign ABC-123 jdoe                  # also me, or "" to unassign
jira8 issue watch ABC-123                        # you, or a USER as 2nd arg
jira8 issue unwatch ABC-123 jdoe
jira8 issue watchers ABC-123 -o json
jira8 issue edit ABC-123 --due-date 2026-11-30 --original-estimate 3d
jira8 issue transition ABC-123 --to "In Progress"
jira8 issue transition ABC-123 --to Done --field "Resolution=Fixed" --markdown --comment "…"
jira8 issue comment-add ABC-123 --markdown --body "…"
jira8 issue comment-edit ABC-123 --id 84887 --markdown --body "…"
jira8 issue worklog-add ABC-123 --time 1h30m --date 2026-10-09 --comment "…"
jira8 issue link ABC-1 ABC-2 --type Blocks       # ABC-1 blocks ABC-2
jira8 issue attachment add ABC-123 a.png b.log
```

- `issue edit` only sends the flags you pass, and is all or nothing: if Jira
  rejects one field, none changes.
- `issue assign` needs only the Assign permission: use it when `issue edit
  --assignee` is refused (403). Some projects require an assignee, so `""` can
  fail with "Issues must be assigned".
- `--assignee me` resolves the current user. Users are Jira **usernames**
  (`{"name": …}`), not Cloud `accountId`s.
- Durations (`--time`, `--original-estimate`) take `w d h m`: `2h`, `1h30m`,
  `3d`. `--original-estimate` is the up-front estimate; time actually spent
  goes in `worklog-add`.
- `worklog-add --date`: `YYYY-MM-DD` (09:00 local time), `YYYY-MM-DDTHH:MM`
  (local) or RFC 3339. Default: now.
- `transition --to` takes the **transition** name shown by `issue transitions`,
  which is not always the target status name. On a miss, the error lists the
  valid ones. Some transitions have a screen (`fields` in `issue
  transitions`), typically a required Resolution on Done: pass it with
  `--field "Resolution=…"`; a missing required field fails before anything
  is sent and lists the allowed values. `--field` on a transition without a
  screen is an error: use `issue edit`. `--comment`/`--comment-file` (with
  `--markdown`) add a comment right after the transition.
- `issue create` uploads attachments after creating the issue. If the upload
  fails, the issue still exists: the error gives its key, retry with
  `issue attachment add`.
- **`--dry-run`** on any Jira command prints the requests it would send (method,
  path, JSON body, files to upload) and sends none; exit 0. Reads still go to
  Jira, so what jira8 checks itself (`--field`, `--labels`, transition names
  and screens, users, link types, boards) fails as usual. What only Jira checks
  when it receives the write (issue type, priority, required fields of create,
  ids that do not exist) does **not**: a clean dry run does not promise the
  write will be accepted. A dry-run create calls the new issue `NEW`; deletes
  need no `--yes`. On a read command it changes nothing.

```sh
jira8 issue create --project ABC --type Task --summary "…" --markdown --description-file spec.md --dry-run
```

## Labels, components, versions and custom fields

```sh
jira8 issue create --project ABC --type Story --summary "…" --labels backend,api --field "Story Points=5"
jira8 issue edit ABC-123 --add-label urgent --remove-label triage
jira8 issue edit ABC-123 --labels ""             # remove every label
jira8 issue edit ABC-123 --field "Component/s=core,ui" --field "Fix Version/s=2.4"
jira8 issue edit ABC-123 --field customfield_10300='{"value":"Gold"}'
```

- `--labels a,b` replaces the labels; `--add-label`/`--remove-label` keep the
  rest (edit only, not combined with `--labels`). Labels have no spaces.
- `--field NAME=VALUE` (repeatable) takes the field name or id as listed by
  `project fields` (create) or `issue fields` (edit). The value is converted
  by field type: numbers, option and user names (`me` works), versions and
  components by name; arrays take commas or repeated flags; `""` clears; a
  value starting with `{` or `[` is sent as raw JSON.
- jira8 checks `--field` against the screen before writing: a field not on
  it or a value outside `allowed` fails with the valid choices. Labels are not
  pre-checked: if the project hides them, Jira answers 400 (see Errors).
- All four (`issue create|edit`, `epic create|edit`) take `--labels` and
  `--field`; both edits take `--add-label`/`--remove-label`.
- Projects can have hundreds of versions: `jq` the `allowed` list rather than
  reading it whole.

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
fields by themselves; never hardcode `customfield_…` ids. Prefer
`epic create` over `issue create --type Epic`, which also needs `--epic-name`.

```sh
jira8 epic create --project ABC --name "Billing v2" --summary "Billing v2" --markdown --description-file epic.md
jira8 epic edit ABC-42 --name "Billing v2 (rev)"
```

## Destructive commands

```sh
jira8 issue comment-delete ABC-123 --id 84887 --yes
jira8 issue worklog-delete ABC-123 --id 27705 --yes
jira8 issue attachment delete 45821 --yes        # ids from the *-list commands
jira8 issue link-delete 55539 --yes              # id from issue links
```

They ask on a terminal. Without one (your case) they fail with exit 1 unless
you pass `--yes`. Pass it only after the user confirmed that exact deletion:
nothing deleted can be recovered.

## Not possible with jira8

Say so instead of improvising with the REST API:

- sprints and votes;
- deleting, cloning or moving issues to another project (Jira Server 8 has no
  REST endpoint to move; it is done in the web UI);
- filters, dashboards, boards configuration, admin settings.

## Errors

Errors go to stderr as one line. Only a wrong flag or argument also prints the
command usage. The exit code tells what failed:

| Exit | Meaning | What to do |
|---|---|---|
| 0 | done (also a `--dry-run`) | |
| 1 | jira8 refused before sending: unknown name, missing required field, no project, config, file | read the message, fix the input |
| 2 | usage: unknown command or flag, wrong arguments | check `--help` |
| 3 | Jira said 401/403: credentials or permission | tell the user |
| 4 | Jira said 404 | check the key or id; it may be invisible to the user |
| 5 | Jira rejected the request (400, 409, other 4xx) | read the message below |
| 6 | Jira failed (5xx) or kept rate-limiting | retry later |
| 7 | Jira was not reached (network, VPN, timeout) | tell the user; do not loop |

- `Jira API error (HTTP n): message; field: message`:
  - `400` naming a value (`Priority name 'High' is not valid`): check
    `project types|statuses|priorities` or `issue transitions`;
  - `400` "Field 'x' cannot be set. It is not on the appropriate screen": the
    project does not expose that field (`project fields` / `issue fields`
    list what it does); nothing jira8 can do, tell the user;
  - `400` "Issue type is a sub-task but parent issue key…": add `--parent`;
  - `401` bad credentials, `403` no permission, `404` "Issue Does Not Exist"
    may just mean the issue is not visible to the user;
  - `429` (rate limit) is retried automatically.
- `cannot reach Jira (check the network or VPN)`: the server is not reachable
  from this machine. Do not retry in a loop or touch the config; tell the user.
- To diagnose, without changing anything:

  ```sh
  jira8 config show -o json    # file read, each value's source (flag/env/file), problems
  jira8 ping                   # does Jira answer? version, base URL, latency
  jira8 user me                # do the credentials work?
  ```

  `config show` never prints the token or password, only whether they are set.
- `no project: pass --project KEY…`: pass `--project`.
