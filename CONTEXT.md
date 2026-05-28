# OSRS Events Bot Context

This context defines the domain language for guild-managed competition features in the bot. It exists to keep feature naming consistent as BOTW/SOTW and PB systems evolve.

## Language

### Personal Best Domain

**PB Category**:
A named leaderboard lane for one activity (for example `inferno`) with its own review queue and ranking.
_Avoid_: Event type, boss category

**PB Submission**:
A user-submitted proof payload for a PB Category, containing proof media and optional entered time until moderation resolves it.
_Avoid_: Run record, final PB

**Pending Submission**:
A PB Submission awaiting moderator decision and still eligible for accept/reject reactions.
_Avoid_: Approved submission, archived submission

**Accepted PB**:
The fastest approved time for one user in one guild and one PB Category.
_Avoid_: Latest PB, any approved run

**Proof Queue**:
The moderation stream of Pending Submissions shown in the `pb-proofs` channel.
_Avoid_: Leaderboard feed, audit log

**Reviewed Submission**:
A PB Submission that already has a final moderation decision and can no longer be decided again.
_Avoid_: Pending submission, duplicate decision

**PB Leaderboard Message**:
The persistent embed message for a guild/category that displays the current top rankings.
_Avoid_: Proof post, submission message

## Example Dialogue

Dev: "A user posted a run in `pb-proofs`; is that already their PB?"  
Domain expert: "No, that is a **PB Submission** in the **Proof Queue**. It becomes an **Accepted PB** only after moderator approval, and only if it is faster than their existing accepted time."  
Dev: "So the public board updates from the accepted value, not from every submission?"  
Domain expert: "Exactly. The **PB Leaderboard Message** reflects accepted fastest times per **PB Category**."
