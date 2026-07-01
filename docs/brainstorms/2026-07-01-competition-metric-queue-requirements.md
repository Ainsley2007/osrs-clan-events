---
date: 2026-07-01
topic: competition-metric-queue
---

# Competition Metric Queue

## Summary

Guild administrators get an admin-only Discord command to queue **Competition Metrics** (bosses for BOTW, skills for SOTW) in FIFO order per competition type. When a new week starts — via scheduled **Rollover** or manual `/start` — the bot uses the front of the queue if any entries exist; when the queue is empty, selection returns to the existing weighted-random **Metric Pool** behavior with **Metric Down-weighting**.

## Problem Frame

New bosses and skills land on unpredictable schedules. The bot otherwise picks the next **Competition Metric** at **Rollover** using weighted random from **Recent Metric History**, which is fair over time but offers no way to guarantee a specific metric for the upcoming week (for example when a new boss releases mid-week and admins want it as the next BOTW without waiting for random luck).

Today admins can `/stop` and `/start`, but `/start` still randomizes both BOTW and SOTW. There is no durable, per-guild way to reserve the next metric.

## Key Decisions

- **FIFO per competition type** — Separate queues for BOTW and SOTW. Multiple entries schedule multiple future weeks; one entry is consumed per new week start.
- **Consume on any week start** — Both scheduled **Rollover** and manual `/start` dequeue the front item when present. Queued metrics do not wait exclusively for rollover.
- **Random only when queue is empty** — After the queue is drained, selection reverts to existing weighted-random logic. No persistent override beyond queued entries.
- **Validate on add** — A metric must exist in the remote **Metric Pool** (Firebase Remote Config) when queued. Unknown names are rejected at command time.
- **Skip stale queue entries at selection** — If the queue head references a metric no longer in remote config, drop it and try the next entry; do not block week start.
- **Admin visibility** — When a queued metric is used or queue entries are changed, reflect that in the guild log channel (same pattern as other admin competition actions). **Rollover Log** should distinguish queued vs random selection for the new week.

## Requirements

**Command**

- R1. Provide a single admin-only Discord slash command `/queue-event` with subcommands: `add`, `list`, `remove`, and `clear`.
- R2. Every subcommand requires a `type` option: BOTW or SOTW.
- R3. `add` accepts a metric name (boss or skill). The name is validated against the remote **Metric Pool** for that type. On success, append to the end of that guild's FIFO for the chosen type.
- R4. `list` returns the ordered queue for the chosen type (position + metric name). Empty queue returns an explicit empty state.
- R5. `remove` accepts a 1-based position and removes that entry. Invalid position returns a clear error.
- R6. `clear` removes all entries for the chosen type and reports how many were removed.
- R7. Metric name input uses autocomplete sourced from the remote **Metric Pool** (same validation universe as `add`).

**Selection at week start**

- R8. When preparing a new BOTW or SOTW event, if that type's queue is non-empty, use the front entry (after skipping any stale entries per Key Decisions).
- R9. After the new event is successfully persisted, remove the consumed queue entry. Failed week start must not consume the queue.
- R10. When the queue is empty, use existing weighted-random selection with **Metric Down-weighting** unchanged.
- R11. A consumed queued metric still enters **Recent Metric History** like any other week so future random picks down-weight it normally.

**Permissions and scope**

- R12. Only guild administrators may use the command (consistent with `/start`, `/stop`, `/addpoints`).
- R13. Queue state is per guild and per competition type, persisted across bot restarts.

## Key Flows

- F1. **Queue a new boss for next week**
  - **Trigger:** Admin runs `add` with type BOTW and a boss from the pool.
  - **Actors:** Guild administrator, bot.
  - **Steps:** Validate boss exists in remote config → append to guild BOTW queue → confirm position in queue → log to guild log channel.
  - **Covered by:** R3, R7, R12, R13.

- F2. **Scheduled rollover with queued skill**
  - **Trigger:** Active SOTW week ends; **Rollover** starts the next week.
  - **Actors:** Scheduler, bot.
  - **Steps:** Complete current week → prepare next SOTW → dequeue queue head if present (skip stale) → create event → consume queue entry → **Rollover Log** shows queued metric for new SOTW and random/queued for BOTW independently.
  - **Covered by:** R8, R9, R10, R11.

- F3. **Manual start while queue has entries**
  - **Trigger:** Admin runs `/start` while BOTW and/or SOTW queues have entries.
  - **Actors:** Guild administrator, bot.
  - **Steps:** Start new weeks for both types → each type consumes its own queue head if present, else random.
  - **Covered by:** R8, R9, R10.

## Acceptance Examples

- AE1. **Covers R3, R8, R9**
  - **Given:** BOTW queue is `["Maggot King"]` and remote config includes Maggot King.
  - **When:** **Rollover** starts the next BOTW week.
  - **Then:** New BOTW metric is Maggot King, queue is empty, **Rollover Log** indicates the metric was queued.

- AE2. **Covers R8, R10**
  - **Given:** BOTW queue is empty.
  - **When:** **Rollover** starts the next BOTW week.
  - **Then:** Metric is chosen by weighted random; behavior matches pre-queue bot.

- AE3. **Covers R8, R9, FIFO**
  - **Given:** BOTW queue is `["Boss A", "Boss B"]`.
  - **When:** Two consecutive week starts occur (rollover or `/start`).
  - **Then:** First week is Boss A, second is Boss B, third week uses random selection.

- AE4. **Covers stale skip**
  - **Given:** BOTW queue is `["Removed Boss", "Vorkath"]` and Removed Boss is not in remote config.
  - **When:** Next BOTW week starts.
  - **Then:** Removed Boss is dropped without blocking; Vorkath is selected; queue is empty afterward.

- AE5. **Covers R3 rejection**
  - **Given:** Admin queues `"Not A Real Boss"`.
  - **When:** Name is not in remote config.
  - **Then:** Command fails with a clear error; queue unchanged.

## Scope Boundaries

**Deferred for later**

- Mid-week **Metric Replace** (swap the active boss or skill without ending the synchronized week).
- Queue size limits, duplicate-entry warnings, or deduplication rules (duplicates allowed unless planning finds a reason not to).
- Player-visible queue announcements outside the admin log channel and **Rollover Log**.

**Outside this product's identity**

- Changing **Metric Down-weighting** rules or **Metric Pool** contents from Discord (remote config remains the source of pool membership).
- Cross-guild queue sharing.

## Dependencies / Assumptions

- Remote config continues to define the authoritative **Metric Pool** for bosses and skills.
- **Rollover** and `/start` remain the only paths that begin a new competition week for a guild.
- Partial in-repo work already wires BOTW queue consumption into event preparation; this feature completes BOTW, adds SOTW parity, persistence operations, Discord command, logging, and tests.

## Outstanding Questions

**Deferred to planning**

- Exact log-channel message wording and **Rollover Log** field layout.
- Whether `remove` without position should support removing by metric name (v1 is position-only per R5).
