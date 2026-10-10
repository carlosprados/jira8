// Package skill embeds the agent skill document shipped with the binary,
// so the installed skill always matches the installed CLI.
package skill

import _ "embed"

// Name is the skill directory name under .claude/skills.
const Name = "jira8"

//go:embed SKILL.md
var Content []byte
