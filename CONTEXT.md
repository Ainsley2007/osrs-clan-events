# OSRS Events Bot Context

This context defines the domain language for guild-managed competition features in the bot. It exists to keep feature naming consistent as BOTW/SOTW and PB systems evolve.

## Language

### Personal Best Domain

**PB Category**:
A named leaderboard lane for one activity (for example `inferno`) with its own review queue and ranking.
_Avoid_: Event type, boss category

**Leaderboard Group**:
A named bundle of PB Categories rendered together in one Discord message (for example `Minigames`).
_Avoid_: Category, channel section

**Group Bundle Message**:
The single Discord message for one Leaderboard Group, containing group header text and all category embeds in that group.
_Avoid_: Per-category message, proof post

**Global Rebuild**:
A recovery action that removes all PB group bundle messages for a guild and recreates them from persisted state.
_Avoid_: Partial fix, single-message patch

**PB Submission**:
A user-submitted proof payload for a PB Category, containing proof media and optional entered time until moderation resolves it.
_Avoid_: Run record, final PB

**Pending Submission**:
A PB Submission awaiting moderator decision and still eligible for accept/reject reactions.
_Avoid_: Approved submission, archived submission

**Accepted PB**:
The fastest approved time for one user in one guild and one PB Category.
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

## Example Dialogue

Dev: "A user posted a run in `pb-proofs`; is that already their PB?"  
Domain expert: "No, that is a **PB Submission** in the **Proof Queue**. It becomes an **Accepted PB** only after moderator approval, and only if it is faster than their existing accepted time."  
Dev: "So the public board updates from the accepted value, not from every submission?"  
Domain expert: "Exactly. The **Group Bundle Message** reflects accepted fastest times per **PB Category** within its **Leaderboard Group**."
