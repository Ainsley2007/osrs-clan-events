---
title: Go package structure and handler layering cleanup
date: 2026-07-06
category: architecture-patterns
module: discord
problem_type: convention
component: discord
severity: medium
applies_when:
  - "Bot struct exposes too many exported fields consumed by main or scheduler"
  - "Command handlers reach past services into database.Store"
  - "Scheduler imports Discord presentation code"
  - "Background jobs silently ignore leaderboard or interaction errors"
resolution_type: refactor
tags:
  - package-layout
  - exports
  - layering
  - scheduler
  - discord
  - services
---

# Go package structure and handler layering cleanup

## Context

The bot had grown a wide public `Bot` API, handlers calling `database.Store` directly, and `scheduler` importing `discord` for log embeds. Configuration also drifted (`DATABASE_URL` loaded but unused while `main` read `DATABASE_PATH` directly).

## Guidance

### Package boundaries

| Layer | Responsibility | Should not |
|-------|----------------|------------|
| `cmd/` | Wire dependencies, start/stop | Reach into bot internals |
| `internal/discord` | Handlers, session, notifier impl | Query DB directly from handlers |
| `internal/discord/services` | Business logic + narrow store interfaces | Import `discord` handlers |
| `internal/scheduler` | Background jobs | Import `internal/discord` (use `Notifier` interface) |

### Bot wiring

- Unexport service fields on `Bot`.
- Expose `bot.Scheduler(store)` as the only scheduler wiring point.
- Implement `scheduler.Notifier` in `discord.SessionNotifier` for rollover DMs and log embeds.

### Handlers

- Route persistence through services (`GuildService`, `EventService`, etc.).
- Use `requireAdmin` / `interactionActor` for guild command safety.
- Use `goSafe` for deferred/async handler work so panics are logged.
- Prefer `RefreshLeaderboards` over raw `Update*` calls when errors should be logged.

### Config

- Load `DATABASE_PATH` into `config.Config.DatabasePath` with default `osrs_events.db`.
- Return `database.Store` from `NewSQLiteStore`, not `*SQLiteStore`.

### Partial /start failure

If BOTW starts but SOTW fails, call `EventService.AbortStartedEvent` on the BOTW row and `AbortActiveEventIfPresent` for `sotw` in case SOTW was persisted before snapshot seeding failed.

Pre-check `/start` with `GetActiveEvents` (any `is_active` row), not `IsEventRunning` (end time in the future), so expired-but-active competitions still block until `/stop` or rollover.

## Key files

- `internal/discord/bot.go` — narrow API, `Scheduler()`
- `internal/scheduler/notifier.go` — `Notifier` + `RolloverEvent`
- `internal/discord/notifier.go` — Discord implementation
- `internal/discord/permissions.go` — `requireAdmin`, `requireGuildActor`
- `internal/discord/async.go` — `goSafe`
- `internal/discord/services/store_interfaces.go` — per-domain store slices

## Verification

```bash
make test
go build ./...
```
