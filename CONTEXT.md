# OSRS Events Bot Context

This context defines the domain language for guild-managed competition features in the bot. It exists to keep feature naming consistent as BOTW/SOTW and PB systems evolve.

## Language

### Personal Best Domain

**PB Category**:
A named leaderboard lane for one activity (for example `inferno`) with its own review queue and ranking.
_Avoid_: Event type, boss category

**Leaderboard Group**:
A named bundle rendered together in one Discord message (for example `Minigames`), made up of PB Category boards and optionally other fixed content.
_Avoid_: Category, channel section

**Submission Rules**:
The informational Leaderboard Group at the top of the PB leaderboard channel: one Rules Embed with how to submit and clan expectations, not a ranked PB Category.
_Avoid_: PB Category, proof post, Accepted PB

**Rules Embed**:
The single static embed in the Submission Rules Leaderboard Group; not tied to rankings or the Proof Queue.
_Avoid_: Group Bundle Message for bosses, Pending Submission

**Group Bundle Message**:
The single Discord message for one Leaderboard Group, containing group header text and its embeds (category boards and/or a Rules Embed).
_Avoid_: Per-category message, proof post

**Global Rebuild**:
A recovery action that removes all PB group bundle messages for a guild and recreates them from persisted state.
_Avoid_: Partial fix, single-message patch

**PB Submission**:
A user-submitted proof payload for a PB Category, containing proof media and a declared time until moderation resolves it.
_Avoid_: Run record, final PB

**Valid Declared Time**:
A declared time that matches the standard in-game time format and can be ranked on the leaderboard.
_Avoid_: Free-text guess, moderator-edited time

**Pending Submission**:
A PB Submission with a Valid Declared Time that is in the Proof Queue and awaiting moderator decision.
_Avoid_: Approved submission, archived submission

**PB Submitter**:
The Discord member who runs `/submit-pb` and owns the submission for ranking purposes.
_Avoid_: Any teammate, moderator

**PB Teammate**:
An optional additional clan member tagged on a submission as having participated in a group PB; must be a member of the same Discord server as the submission.
_Avoid_: PB Submitter, moderator, users outside the server

**Leaderboard Display Name**:
The name line shown on a PB Category board: PB Submitter first, then PB Teammates when present (for example `Alice, Bob, Charlie`).
_Avoid_: RSN, Discord handle only

**Accepted PB**:
The fastest approved time for one PB Submitter in one guild and one PB Category, shown on the board with its Leaderboard Display Name.
_Avoid_: Latest PB, any approved run

**PB Leaderboard Place**:
One of the top three rank positions shown on a PB Category board (1st, 2nd, or 3rd place).
_Avoid_: Top three records, row limit

**PB Place Tie**:
When multiple Accepted PBs share the same canonical time at one PB Leaderboard Place, all tied players are shown for that place and the visible row count can exceed three.
_Avoid_: Hidden tie, single winner per place

**PB Category Variant**:
A separate PB Category for an alternate ruleset of the same activity (for example `duke_sucellus_awakened` alongside `duke_sucellus`), with its own ranking and Accepted PB.
_Avoid_: Shared leaderboard lane, mode flag on one category

**Proof Queue**:
The moderation stream of Pending Submissions shown in the `pb-proofs` channel.
_Avoid_: Leaderboard feed, audit log

**Reviewed Submission**:
A PB Submission that already has a final moderation decision and can no longer be decided again.
_Avoid_: Pending submission, duplicate decision

**PB Leaderboard Message State**:
Persisted mapping from guild and Leaderboard Group to the Group Bundle Message ID used for updates.
_Avoid_: Per-category state, proof message

### Weekly Competition Domain

**Weekly Competition**:
A time-bounded guild contest tracked by the bot, currently either Boss of the Week (BOTW) or Skill of the Week (SOTW).
_Avoid_: Event, season, PB category

**BOTW**:
Boss of the Week; a Weekly Competition where progress is measured in boss kill count against a selected boss metric.
_Avoid_: PB Category, boss PB

**SOTW**:
Skill of the Week; a Weekly Competition where progress is measured in experience gained for a selected skill metric.
_Avoid_: PB Category, skill training PB

**Competition Metric**:
The boss or skill chosen for one Weekly Competition week (for example Fletching or Vorkath).
_Avoid_: PB Category, activity slug

**Metric Pool**:
The configured set of bosses or skills eligible to be chosen as the Competition Metric for a new week.
_Avoid_: Leaderboard group, command choices

**Recent Metric History**:
The record of Competition Metrics used in prior weeks for one guild and one competition type, used when picking the next metric.
_Avoid_: PB records, proof queue

**Metric Down-weighting**:
Selection bias that lowers the chance a metric is picked again when it appears often in Recent Metric History, without removing it from the pool.
_Avoid_: Ban list, guaranteed rotation, PB tie logic

**Rollover**:
The end-of-week transition that completes the current Weekly Competition, awards points, and starts the next week with a newly selected Competition Metric.
_Avoid_: Global Rebuild, guild initialization

## Example Dialogue

Dev: "A user posted a run in `pb-proofs`; is that already their PB?"  
Domain expert: "No, that is a **PB Submission** in the **Proof Queue**. It becomes an **Accepted PB** only after moderator approval, and only if it is faster than their existing accepted time."  
Dev: "So the public board updates from the accepted value, not from every submission?"  
Domain expert: "Exactly. The **Group Bundle Message** reflects accepted fastest times per **PB Category** within its **Leaderboard Group**."

Dev: "User typed `12:34` on `/submit-pb` — does that land in the Proof Queue?"  
Domain expert: "No. Without a **Valid Declared Time**, there is no **Pending Submission**. They get a command error; only format-valid times enter the **Proof Queue**."  
Dev: "What if the time parses but doesn't match the screenshot?"  
Domain expert: "That is moderation. **Valid Declared Time** means format, not proof accuracy — moderators reject factual mismatches."

Dev: "Fletching was SOTW twice already — why did it get picked again?"  
Domain expert: "**Metric Down-weighting** reduces the chance, it does not block repeats. Fletching still had a share of the **Metric Pool** because only **Recent Metric History** for that guild matters, and untouched skills stay more likely than heavily used ones."  
Dev: "So new bosses and skills in the pool get a better shot?"  
Domain expert: "Yes. Metrics that rarely or never appear in **Recent Metric History** are favored when **Rollover** selects the next **Competition Metric**."
