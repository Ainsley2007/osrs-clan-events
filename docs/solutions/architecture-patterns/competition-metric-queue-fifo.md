---
module: weekly-competitions
date: 2026-07-01
problem_type: architecture_pattern
component: service_object
severity: medium
applies_when:
  - "Admins need to guarantee a specific BOTW boss or SOTW skill for upcoming week(s)"
  - "Metric selection otherwise uses weighted random from Firebase Remote Config at week start"
tags:
  - botw
  - sotw
  - queue
  - rollover
  - discord
  - metric-selection
related_components:
  - database
  - discord
  - scheduler
---

# Competition Metric Queue (FIFO)

## Context

Weekly competitions pick a **Competition Metric** from the **Metric Pool** (Firebase Remote Config). Weighted random with **Metric Down-weighting** is fair over time but cannot guarantee a specific boss or skill when content drops mid-week (for example a new boss release).

Admins previously had no durable way to reserve metrics except `/stop` + `/start`, which still randomizes selection.

## Guidance

Use a per-guild, per-competition-type FIFO **Competition Metric Queue** stored in SQLite (`metric_queue` table).

**Week start paths** (rollover and `/start`) share one selection rule:

1. Peek queue head for the event type (`botw` / `sotw`).
2. Skip stale entries not present in Remote Config (pop and retry).
3. If a valid head exists, use it; otherwise fall back to existing weighted random.
4. Pop the queue entry only after `CreateEvent` succeeds (failed persist must not consume).

**Admin surface:** `/queue-event` subcommands `add`, `list`, `remove`, `clear` with a required `type` option (BOTW/SOTW). Metric names validate against Remote Config on add; autocomplete uses the same pool.

**Logging:** Rollover and manual start logs annotate new weeks as `— queued` or `— random`.

## Why This Matters

Peek-then-pop-on-success avoids losing queue entries when event creation fails mid-rollover. Separate queues per type keep BOTW and SOTW independent while preserving **Synchronized Competition Period** (both still start together; each type consumes its own queue head).

Stale-entry skipping prevents a removed Remote Config boss from blocking rollover.

## When to Apply

- Guaranteeing one or more future weeks of specific metrics without changing down-weighting rules.
- Scheduling multiple releases ahead (FIFO consumes one entry per week start).

Do not use the queue for mid-week metric swaps without ending the current week — that requires a separate **Metric Replace** flow (out of scope for v1).

## Examples

**Queue Maggot King for next BOTW:**

```
/queue-event add type:BOTW metric:Maggot King
```

**Selection with queue `["Boss A", "Boss B"]`:**

- First rollover/start → Boss A (queue pop after persist)
- Second rollover/start → Boss B
- Third week → weighted random (empty queue)

**Key files:**

- `internal/database/sqlite_metric_queue.go` — persistence
- `internal/discord/services/event_queue.go` — queue helpers + admin service methods
- `internal/discord/services/event.go` — prepare paths + consume on `CreateEvent`
- `internal/discord/cmd_queue_event.go` — Discord command
