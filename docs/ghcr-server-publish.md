# Publish `server` Image to GHCR

This guide builds the `server` service from `docker-compose.yml` and pushes it to GitHub Container Registry (GHCR).

## Prereqs

- Docker installed and running.
- A GitHub Personal Access Token (PAT) with `write:packages` (and `read:packages` if you need to pull).

## Choose Image Name and Tag

Set these environment variables so commands stay consistent:

```sh
export GHCR_OWNER="<github-org-or-username>"
export GHCR_REPO="<github-repo>"
export IMAGE_TAG="latest"
```

The target image name will be:

```
ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}
```

## Authenticate Docker to GHCR

```sh
echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GHCR_OWNER" --password-stdin
```

Notes:
- `GITHUB_TOKEN` should be your PAT.
- If you prefer, you can run `docker login ghcr.io` and paste the PAT when prompted.

## Build the `server` Image

From the repo root:

```sh
docker compose build server
```

This builds using `Dockerfile` and the `server` service definition in `docker-compose.yml`.

## Tag the Image for GHCR

Find the built image (it will be named by Compose, usually `peasydeal-product-miner-server:latest`):

```sh
docker image ls | rg "server"
```

Then tag it:

```sh
docker tag <LOCAL_IMAGE_ID_OR_NAME> "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}"
```

If the local name is known, you can use it directly instead of the image ID, for example:

```sh
docker tag peasydeal-product-miner-server:latest "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}"
```

## Push to GHCR

```sh
docker push "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}"
```

## Optional: Use `docker build` Instead of Compose

If you want a single command without Compose:

```sh
docker build -t "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}" .
docker push "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}"
```

## Optional: Verify the Image

```sh
docker pull "ghcr.io/${GHCR_OWNER}/${GHCR_REPO}/server:${IMAGE_TAG}"
```

## Troubleshooting

- 403 errors on push usually mean missing `write:packages` scope or wrong owner.
- If the image is private, ensure your PAT has access to the repo.
- If GHCR forces lowercase paths, make sure `GHCR_OWNER` and `GHCR_REPO` are lowercase.
