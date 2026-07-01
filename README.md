# osrs-events

Discord bot for OSRS clan competitions: weekly Boss of the Week (BOTW) and Skill of the Week (SOTW), personal best (PB) leaderboards with proof moderation, and donation tracking.

## Features

### Weekly competitions (BOTW & SOTW)

- **BOTW** tracks boss kill count for a randomly selected boss each week.
- **SOTW** tracks XP gained in a randomly selected skill each week.
- Competitions run for **7 days**. BOTW and SOTW for a guild share the same start and end time.
- **Metric selection** picks from Firebase Remote Config. Recently used bosses/skills are down-weighted so the pool rotates naturally.
- **Leaderboards** are posted as embeds in dedicated channels (weekly and overall, per competition type).
- **Points** are awarded at week end based on progress and configured thresholds.

### Personal bests (PB)

- Players submit proofs with `/submit-pb` (screenshot + declared time).
- Only **improving** submissions are accepted (faster than the player's current record, or no record yet).
- Submissions land in the **proof queue** (`pb-proofs` channel) for moderator review.
- Admins approve with ✅ or reject with ❌ on the proof post.
- Approved proofs are stored in **Cloudflare R2** with a durable public URL on the leaderboard.
- Optional **teammates** (up to 4) can be tagged on group PBs.
- Leaderboards are grouped by activity (e.g. minigames, raids) with quick-link navigation at the bottom of the channel.

### Donations

- Track clan donations and spending from a shared fund.
- Donation leaderboard is maintained in a configured channel.

### Automation

On a schedule the bot:

- **Rollover** — completes expired weeks, awards points, and starts the next BOTW/SOTW together (Sunday 22:00 UTC by default per guild).
- **Hourly snapshots** — refreshes hiscore data for active competitions and updates leaderboards.
- **Pending starts** — creates initial snapshots when a competition's start time arrives.
- **Missing accounts** — DMs players whose RSN is not on the hiscores and logs unresolved cases at rollover.

When the bot joins a server it **auto-creates** categories, channels, and leaderboard messages. When removed from a server it **purges** that guild's data.

## Discord channels

The bot creates and maintains these channels (names are fixed; categories show the current metric when a week is active):

| Channel | Purpose |
|---------|---------|
| `botw-weekly` | Current week BOTW standings |
| `botw-overall` | All-time BOTW points |
| `sotw-weekly` | Current week SOTW standings |
| `sotw-overall` | All-time SOTW points |
| `pb-leaderboard` | PB category boards and quick links |
| `pb-proofs` | Moderation queue for pending submissions |

Categories: `╔═══BOTW═══╗`, `╔═══SOTW═══╗`, and a PB category (renamed to include the metric when active).

## Commands

### Everyone

| Command | Description |
|---------|-------------|
| `/add` `rsn` | Link an OSRS account to yourself for competitions. |
| `/remove` `rsn` | Stop tracking an account. |
| `/rename` `current-rsn` `new-rsn` | Update a tracked RSN (e.g. after a name change). |
| `/exit` | Leave all competitions and remove your tracked accounts. |
| `/tracked` | List tracked accounts (your own, or another user if admin). |
| `/stats` | View your progress across past and current BOTW/SOTW weeks. |
| `/submit-pb` `category` `attachment` `time` `teammates?` | Submit a PB proof. Time format: `MM:SS.xx` or `H:MM:SS.xx`. |

### Administrators

| Command | Description |
|---------|-------------|
| `/start` | Start BOTW and SOTW for the current week. |
| `/stop` | End active competitions early and award points. |
| `/queue-event` | Queue bosses (BOTW) or skills (SOTW) for upcoming weeks (`add`, `list`, `remove`, `clear`). |
| `/addpoints` `user` `type` `amount` | Adjust BOTW or SOTW points (negative to subtract). |
| `/setup-logging-channel` `channel` | Channel for competition and rollover logs. |
| `/setup-donation-channel` `channel` | Channel for the donation leaderboard. |
| `/add-donation` `user` `amount` | Record a donation (amount in millions, e.g. `1.5` = 1.5m). |
| `/use-funds` `amount` `description` | Record spending from donation funds. |

Admin-only commands also accept an optional `user` argument on `/add`, `/remove`, `/rename`, `/exit`, and `/tracked` to act on behalf of another member.

PB moderation uses **message reactions** (not slash commands): ✅ to approve, ❌ to reject, only on posts in `pb-proofs` and only by server administrators.

## Configuration

Environment variables (typically via `.env` on the host — see [DEPLOY.md](DEPLOY.md)):

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes | Discord bot token |
| `GOOGLE_APPLICATION_CREDENTIALS` | Yes | Path to Firebase service account JSON (Remote Config for boss/skill/PB definitions) |
| `R2_ACCOUNT_ID` | Yes | Cloudflare account ID for PB proof storage |
| `R2_ACCESS_KEY_ID` | Yes | R2 access key |
| `R2_SECRET_ACCESS_KEY` | Yes | R2 secret key |
| `R2_PUBLIC_BASE_URL` | Yes | Public base URL for proof images |
| `R2_BUCKET` | No | Bucket name (default: `pb-challenge`) |
| `DATABASE_PATH` | No | SQLite path (default: `osrs_events.db`) |
| `LOG_FILE` | No | Rotating log file path (default: `logs/app.log`) |

Verify R2 connectivity: `go run cmd/r2-check/main.go`

## Development

```bash
# Run locally
make run

# Build binary
make build

# Run tests
go test ./...

# Optional: Firebase Remote Config integration test (needs valid credentials)
go test ./internal/firebase/...
```

CI runs `go test ./...` on push to `main` and on pull requests to `main` / `develop`. The Docker image is built and pushed only after tests pass on `main`. The Firebase integration test skips automatically when credentials are not configured.

## Deployment

Production deployment uses Docker on a Linux host with Watchtower for auto-updates. See [DEPLOY.md](DEPLOY.md).

## Further reading

- [CONTEXT.md](CONTEXT.md) — domain terminology (PB categories, rollover, proof queue, etc.)
- [docs/adr/0001-durable-pb-proof-storage.md](docs/adr/0001-durable-pb-proof-storage.md) — why PB proofs use R2
