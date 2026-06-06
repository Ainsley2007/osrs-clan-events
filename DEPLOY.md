# Deploying osrs-events (NUC + Watchtower)

This describes how to run the bot on your Linux NUC and have it update automatically when you merge to `main`.

## Flow

1. **Develop** on branch `develop`, open a PR to `main`.
2. **Merge** the PR into `main`.
3. **GitHub Actions** builds the Docker image and pushes it to GitHub Container Registry (GHCR) as `ghcr.io/<your-org>/osrs-events:latest` (and a short SHA tag).
4. **On your NUC** the app runs in a container. **Watchtower** periodically checks the registry; when it sees a new `latest` image, it pulls it and restarts the container.

## How secrets and data are handled

Nothing sensitive or runtime-specific goes into the image. Everything lives on the NUC and is passed in at run time:

| What | Where on NUC | How the container sees it |
|------|----------------|---------------------------|
| **`.env`** | `~/osrs-events/.env` | `env_file: - .env` — Docker injects those variables into the container. The file is never copied into the image. |
| **`firebase-credentials.json`** | `~/osrs-events/firebase-credentials.json` | Mounted read‑only into the container at `/app/firebase-credentials.json`. Set `GOOGLE_APPLICATION_CREDENTIALS=/app/firebase-credentials.json` in `.env`. |
| **R2 proof storage** | `~/osrs-events/.env` | Cloudflare R2 credentials and public bucket URL for durable PB proof images (see below). |
| **DB file** | `~/osrs-events/data/osrs_events.db` | The directory `./data` is mounted at `/app/data`. The app uses `DATABASE_PATH=/app/data/osrs_events.db`. The file is created on first run and persists across container restarts and image updates. |

Result: you keep one copy of `.env` and `firebase-credentials.json` on the NUC; the DB grows in `./data`. No secrets in the image, no manual copy‑paste into the container.

---

## One-time setup on the NUC

### 1. Log in to GHCR (if the image is private)

```bash
echo $GITHUB_PAT | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

Use a [GitHub Personal Access Token](https://github.com/settings/tokens) with `read:packages`. If the GHCR package is **public** (repo → Package → Package settings → Change visibility), you can pull without logging in; otherwise use the PAT on the NUC.

### 2. Create app directory and put config/secrets there

```bash
mkdir -p ~/osrs-events/data
cd ~/osrs-events
```

**Create `.env`** (same directory as `docker-compose.prod.yml`), with at least:

```env
DISCORD_TOKEN=your_discord_bot_token
GOOGLE_APPLICATION_CREDENTIALS=/app/firebase-credentials.json
R2_ACCOUNT_ID=your_cloudflare_account_id
R2_ACCESS_KEY_ID=your_r2_access_key_id
R2_SECRET_ACCESS_KEY=your_r2_secret_access_key
R2_BUCKET=pb-challenge
R2_PUBLIC_BASE_URL=https://pub-xxxxx.r2.dev
```

**R2 setup (one-time in Cloudflare):**

1. Create an R2 API token with Object Read & Write on the `pb-challenge` bucket.
2. Enable public access on the bucket and copy the `r2.dev` base URL for `R2_PUBLIC_BASE_URL`.
3. Find your account ID in the Cloudflare dashboard (R2 overview) for `R2_ACCOUNT_ID`.

PB approvals persist proof images to R2 at moderation time. If R2 is not configured, the bot starts but PB approvals fail until these variables are set.

**Copy your Firebase key file** into that directory as `firebase-credentials.json`:

```bash
cp /path/to/your/firebase-service-account.json ~/osrs-events/firebase-credentials.json
```

The **DB file** will appear automatically at `./data/osrs_events.db` the first time the app runs; you don’t create it by hand.

**Recommended layout on the NUC:**

```
~/osrs-events/
├── docker-compose.prod.yml   # you copy or git-clone this repo’s file
├── .env                      # you create: DISCORD_TOKEN, GOOGLE_APPLICATION_CREDENTIALS, etc.
├── firebase-credentials.json # you copy your Firebase service-account JSON here
└── data/                     # created by mkdir; DB appears here on first run
    └── osrs_events.db        # created automatically
```

### 3. Use the production compose file

Edit `docker-compose.prod.yml` and replace `OWNER/REPO` with your GitHub org/repo in **lowercase**, e.g.:

`ghcr.io/myuser/osrs-events:latest` → `image: ghcr.io/myuser/osrs-events:latest`

Then:

```bash
docker compose -f docker-compose.prod.yml up -d
```

This runs the **published** image (no local build). Watchtower will only update this container if you use that same image name and tag (e.g. `latest`).

### 4. Watchtower

Ensure Watchtower is running and can reach the registry. If you use labels, add to the osrs-events service:

```yaml
labels:
  - "com.centurylinklabs.watchtower.enable=true"
```

Otherwise Watchtower usually monitors all running containers. It will pull `ghcr.io/<owner>/<repo>:latest` and restart the osrs-events container when the digest changes.

## Summary

| Step | Where |
|------|--------|
| PR from `develop` → `main` | GitHub |
| Build & push image on push to `main` | `.github/workflows/build-and-push.yml` |
| Run image on NUC | `docker-compose.prod.yml` |
| Auto-update on new image | Watchtower |

After your first merge to `main`, the image will be at  
`ghcr.io/<owner>/<repo>:latest`. Use that in `docker-compose.prod.yml` and you’re set.
