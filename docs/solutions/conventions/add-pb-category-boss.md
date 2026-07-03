---
title: Add a PB leaderboard category for a new boss
date: 2026-07-03
category: conventions
module: database
problem_type: convention
component: database
severity: low
applies_when:
  - "A new OSRS boss or activity should appear in the PB submission picker"
  - "The boss belongs to an existing PB category group (e.g. Solo & Duo Bosses A-Z)"
resolution_type: seed_data_update
tags:
  - pb-categories
  - seed-data
  - sqlite
  - boss
---

# Add a PB leaderboard category for a new boss

## Context

PB leaderboard categories are seeded at startup from a static list in `internal/database/sqlite_store.go`. Adding a boss (e.g. Maggot King) requires a seed entry plus test updates so category count, slug order, and group slice indices stay correct.

No separate migration is needed: `seedPBCategories` uses `ON CONFLICT(slug) DO UPDATE`, so existing databases pick up new or changed categories on the next app start.

## Guidance

1. **Add a seed entry** in `seedPBCategories()` inside `internal/database/sqlite_store.go`:
   - `slug`: snake_case identifier (e.g. `maggot_king`)
   - `displayName`: user-facing name (e.g. `Maggot King`)
   - `groupName`: existing group (e.g. `Solo & Duo Bosses (A-Z)`)
   - `groupOrder`: group's sort key (3 for Solo & Duo)
   - `displayOrder`: position within the group (alphabetical for A-Z groups)
   - `imageURL`: OSRS Wiki thumbnail for Discord embeds

2. **Bump `displayOrder`** for any categories that come after the insertion point in the same group.

3. **Update `internal/database/sqlite_pb_test.go`**:
   - Increment expected total category count
   - Insert the new slug in `wantOrder` at the correct global position
   - Shift slice bounds for group assertions (`soloDuoGroup`, `slayerGroup`, etc.) when the new entry falls before later groups

4. **Run tests**: `go test ./internal/database/... -run PBCategor`

## Why This Matters

Missing test updates cause silent ordering regressions: the seed may work but group slice indices and expected counts will fail CI. Skipping the seed entirely means the boss never appears in `/submit-pb` autocomplete.

## When to Apply

- New solo/duo boss, slayer boss, minigame, or raid variant needs PB tracking
- An existing category needs a renamed display label or updated thumbnail (same slug, update seed fields)

## Examples

**Maggot King** added to Solo & Duo Bosses (A-Z), alphabetically after Gauntlet:

```go
{
    slug:         "maggot_king",
    displayName:  "Maggot King",
    groupName:    "Solo & Duo Bosses (A-Z)",
    groupOrder:   3,
    displayOrder: 5,
    imageURL:     "https://oldschool.runescape.wiki/images/Maggot_King.png",
},
```

Test updates: count 30 → 31, `wantOrder` gains `"maggot_king"` after `"gauntlet"`, `soloDuoGroup` slice widens from `[11:17]` to `[11:18]`, and downstream group slices shift by one index.

## Related

- Maggot King is also referenced as a BOTW metric in competition queue docs (`docs/brainstorms/`, `docs/plans/`) — remote Firebase boss config is separate from PB category seeding.
