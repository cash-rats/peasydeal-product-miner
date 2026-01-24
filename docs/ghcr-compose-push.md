# Build and Push `server` Image to GHCR (Docker Compose)

This guide explains how to build the `server` image defined in `docker-compose.yml` and push it to GitHub Container Registry (GHCR).

## Prerequisites

- Docker Desktop or Docker Engine installed
- GitHub account with a Personal Access Token (PAT) that has:
  - `read:packages`
  - `write:packages`

## 1) Authenticate to GHCR

```bash
export GHCR_TOKEN="YOUR_GHCR_PAT"
echo "$GHCR_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

## 2) Choose Your Image Name

Use this format:

```
ghcr.io/<github-username>/<repo-name>:<tag>
```

Example:

```
ghcr.io/peasydeal/peasydeal-product-miner:latest
```

## 3) Build the Image

### Option A — Build via Docker Compose

```bash
docker compose build server
```

If you didn’t set an explicit image name in `docker-compose.yml`, tag the built image:

```bash
docker images | head -n 5
docker tag <local_image>:<local_tag> ghcr.io/<github-username>/<repo-name>:<tag>
```

### Option B — Build Directly from Dockerfile

```bash
docker build -t ghcr.io/<github-username>/<repo-name>:<tag> -f Dockerfile .
```

## 4) Push to GHCR

```bash
docker push ghcr.io/<github-username>/<repo-name>:<tag>
```

## Optional: Let Compose Tag the Image

If you want `docker compose build` and `docker compose push` to use the correct GHCR name automatically, add an `image` field under `services.server` in `docker-compose.yml`:

```yaml
services:
  server:
    image: ghcr.io/<github-username>/<repo-name>:<tag>
```

Then run:

```bash
docker compose build server
docker compose push server
```

