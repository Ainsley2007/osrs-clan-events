# Durable PB proof storage in Cloudflare R2

Discord attachment URLs expire, breaking Proof links on Accepted PB leaderboard rows. We persist proof images to Cloudflare R2 (bucket `pb-challenge`) at moderation approval time—not at submit—because pending submissions only need the Discord attachment for the review window (~2 days). Every successful approval must produce a durable public Proof URL on the Accepted PB; if the Discord attachment is unavailable or the R2 upload fails, approval is blocked and the Pending Submission stays in the Proof Queue so moderators can retry ✅ or reject for resubmission.

## Considered Options

- **Persist at submit vs at approval.** Submit-time persistence is simpler but stores proofs for rejected runs and runs that never improve the board. Approval-time persistence matches what actually lands on the leaderboard.
- **Upload on every approval vs only when the board changes.** Rejected non-improving submissions at `/submit-pb` instead—only Improving Submissions enter the Proof Queue—so every ✅ updates or creates an Accepted PB and always triggers upload.
- **Backfill legacy Discord URLs.** Skipped; existing broken links remain as Legacy Proof URLs until naturally replaced by a faster submission.
- **Public bucket vs signed URLs.** Public read via an R2.dev subdomain matches today's unlisted-link behavior without URL refresh logic.
- **Keep vs delete superseded objects.** Delete the previous R2 object when a player beats their own Accepted PB in the same PB Category (keyed by the old `proof_submission_id`).

## Consequences

- Object keys are flat `{submission_id}.{ext}`; Proof URLs use the bucket's R2.dev public base URL.
- `/submit-pb` must reject times equal to or slower than the submitter's current Accepted PB in that category.
- Approval flow: download from Discord → upload to R2 → delete superseded object if applicable → commit Accepted PB with R2 Proof URL. Upload or download failure rolls back approval.
- R2 credentials and the public base URL are runtime configuration on the NUC (alongside `.env`), not baked into the image.
