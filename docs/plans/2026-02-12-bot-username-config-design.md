# Configurable Bot Username for Self-Detection

## Problem

Familiar currently detects its own events using pattern matching against GitLab project access token bot usernames (`project_<id>_bot_<hash>`) and GitHub app bots (`*[bot]`). This works for project tokens but breaks when Familiar runs as a dedicated GitLab user account (e.g. `@Familiar`), because the username doesn't match either bot pattern.

Running as a dedicated account is desirable for UX — comments show a named identity, and users can @-mention it naturally.

## Design

One new config field and a one-line addition to `isBotActor()`. Existing bot pattern detection stays intact.

### 1. Config: `bot_username`

Add `BotUsername string` to the top-level `Config` struct. Default to `"Familiar"` in `DefaultConfig()`.

```yaml
# config.yaml
bot_username: "Familiar"
```

This is a top-level field because bot identity is not provider-specific — the same account name applies across all configured providers.

### 2. Router: case-insensitive username check

Pass the configured bot username into `isBotActor()`. Add a case-insensitive comparison as the first check, before existing patterns:

```go
func isBotActor(actor, botUsername string) bool {
    if actor == "" {
        return false
    }
    if strings.EqualFold(actor, botUsername) {
        return true
    }
    // existing GitHub [bot] and GitLab project token checks unchanged
    ...
}
```

The `Router` already holds `serverCfg`, so it passes `r.serverCfg.BotUsername` at the call site.

### 3. Config example

Add `bot_username` to `config.example.yaml` with a comment noting the default.

## Scope

- No changes to provider code, agent spawning, or token handling
- Existing project token and GitHub bot detection continues to work
- The GitLab account creation and PAT generation are outside Familiar's codebase

## Files Changed

- `internal/config/config.go` — add `BotUsername` field and default
- `internal/event/router.go` — update `isBotActor()` signature and add username check
- `internal/event/router_test.go` — add test cases for configured bot username
- `config.example.yaml` — add `bot_username` with comment
