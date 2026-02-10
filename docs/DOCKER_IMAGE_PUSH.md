# Docker Image Push Instructions

This document describes how to push the Modeltunnel Docker image to GitHub Container Registry (GHCR).

## Prerequisites

1. GitHub PAT (Personal Access Token) with `write:packages` scope
2. Docker installed on your machine

## Step 1: Create GitHub PAT

1. Go to GitHub Settings → Developer Settings → Personal access tokens → Tokens (classic)
2. Generate new token with:
   - `write:packages` scope
   - (Optional) `read:packages` scope

## Step 2: Login to GitHub Container Registry

```bash
# Using your GitHub username and PAT
echo YOUR_PAT | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

## Step 3: Tag the Image

```bash
# Tag for version v1.0.0
docker tag modeltunnel:latest ghcr.io/steliosot/modeltunnel:v1.0.0

# Also tag as latest
docker tag modeltunnel:latest ghcr.io/steliosot/modeltunnel:latest
```

## Step 4: Push to GHCR

```bash
# Push version tag
docker push ghcr.io/steliosot/modeltunnel:v1.0.0

# Push latest tag
docker push ghcr.io/steliosot/modeltunnel:latest
```

## Step 5: Verify

```bash
# List images in GHCR
ghcr.io/steliosot/modeltunnel

# Or via GitHub CLI
gh repo view steliosot/modeltunnel --json packages
```

## Using the Image

Once pushed, users can pull and use the image:

```bash
docker pull ghcr.io/steliosot/modeltunnel:latest

docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up --ollama --model mistral
```

## Troubleshooting

### Permission Denied

If you get `denied` error when pushing:

```
error from registry: denied
```

**Solution:** Check that your PAT has the `write:packages` scope.

### Authentication Failed

If you get authentication errors:

```
no basic auth credentials
```

**Solution:** Login to GHCR again with your PAT:
```bash
echo YOUR_PAT | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

### Image Too Large

The image should be ~45MB. If it's significantly larger:

1. Check you're using the multi-stage Dockerfile
2. Verify `CGO_ENABLED=1` is set during build
3. Ensure Alpine Linux is used as base image

## Automating with GitHub Actions

To automatically build and push on releases, create `.github/workflows/docker.yml`:

```yaml
name: Docker

on:
  push:
    tags:
      - 'v*'

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to the Container registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest,${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.ref_name }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

## Release Process

When creating a new release (e.g., v1.1.0):

1. Tag the release: `git tag v1.1.0`
2. Push the tag: `git push origin v1.1.0`
3. GitHub Actions will automatically build and push the image
4. Update CHANGELOG.md
5. Create GitHub release with notes

## Verification Checklist

- [ ] Image builds successfully (`docker build`)
- [ ] Image size is ~45MB
- [ ] Can pull from GHCR (`docker pull ghcr.io/steliosot/modeltunnel:latest`)
- [ ] Container starts and runs properly
- [ ] Health endpoint responds
- [ ] Dashboard accessible
- [ ] API endpoints work with authentication
- [ ] SQLite database support working