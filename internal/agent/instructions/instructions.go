package instructions

import _ "embed"

//go:embed familiar.md
var familiarMD string

// Content returns the Familiar agent instructions markdown.
// This is written to /home/agent/.claude/CLAUDE.md at container startup
// so Claude Code auto-loads it as global instructions.
func Content() string { return familiarMD }
