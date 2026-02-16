# Local Testing Guide

This guide explains how to test the complete verity workflow locally without using GitHub Actions.

## Prerequisites

- Docker and Docker Compose
- Helm CLI
- Go 1.25+
- [Copa](https://github.com/project-copacetic/copacetic) (for patching)
- [Trivy](https://github.com/aquasecurity/trivy) (for vulnerability scanning)

## Quick Start

```bash
# 1. Start local registry and BuildKit
make up

# 2. Test the complete workflow (discover + patch 3 images)
make test-local-workflow
```

## Testing Workflow

### 1. Start Local Infrastructure

```bash
make up
```

This starts:
- **Local OCI Registry** at `localhost:5555`
- **BuildKit** at `tcp://localhost:1234`

### 2. Scan Charts for Images

```bash
# Download chart dependencies and scan for images
make scan

# Or manually:
helm dependency update .
./verity scan --chart . -o values.yaml
```

This updates `values.yaml` with all images found in the charts.

### 3. Discover Images

```bash
make build
./verity discover --images values.yaml --discover-dir .verity
```

This generates:
- `.verity/manifest.json` - Full image list with metadata
- `.verity/matrix.json` - GitHub Actions matrix format

### 4. Test Patching

#### Single Image Test
```bash
make test-local-patch
```

Patches nginx as a quick test.

#### Complete Workflow Test
```bash
make test-local-workflow
```

Discovers all images and patches the first 3 to your local registry.

#### Patch Specific Image
```bash
./verity patch \
  --image "docker.io/grafana/grafana:12.3.3" \
  --registry "localhost:5555/verity" \
  --buildkit-addr "tcp://localhost:1234" \
  --result-dir .verity/results \
  --report-dir .verity/reports
```

### 5. View Results

```bash
# List patched images in local registry
curl http://localhost:5555/v2/_catalog | jq

# Check patch results
ls -la .verity/results/
cat .verity/results/*.json | jq

# Check vulnerability reports
ls -la .verity/reports/
cat .verity/reports/*.json | jq
```

## Testing Matrix Parallelization (Locally)

To simulate the GitHub Actions matrix parallelization locally:

```bash
# Build verity
make build

# Discover images
./verity discover --images values.yaml --discover-dir .verity

# Patch all images in parallel (requires GNU parallel or similar)
cat .verity/matrix.json | jq -r '.include[].image_ref' | \
  parallel -j4 './verity patch \
    --image {} \
    --registry localhost:5555/verity \
    --buildkit-addr tcp://localhost:1234 \
    --result-dir .verity/results \
    --report-dir .verity/reports'
```

Or using a simple bash loop:
```bash
for img in $(jq -r '.include[].image_ref' .verity/matrix.json); do
  echo "Patching $img..."
  ./verity patch \
    --image "$img" \
    --registry "localhost:5555/verity" \
    --buildkit-addr "tcp://localhost:1234" \
    --result-dir .verity/results \
    --report-dir .verity/reports &
done
wait
```

## Cleanup

```bash
# Stop local services
make down

# Clean artifacts
make clean
```

## Troubleshooting

### BuildKit Connection Issues
```bash
# Check if BuildKit is running
docker ps | grep buildkit

# Restart services
make down && make up
```

### Registry Push Failures
```bash
# Check registry is accessible
curl http://localhost:5555/v2/

# View registry logs
docker compose logs registry
```

### Copa Installation
```bash
# Install Copa (macOS)
brew install copa

# Or download from releases
# https://github.com/project-copacetic/copacetic/releases
```

### Trivy Installation
```bash
# Install Trivy (macOS)
brew install aquasecurity/trivy/trivy

# Or see: https://aquasecurity.github.io/trivy/latest/getting-started/installation/
```

## CI vs Local Differences

| Aspect | GitHub Actions | Local Testing |
|--------|---------------|---------------|
| Registry | `ghcr.io/verity-org` | `localhost:5555/verity` |
| BuildKit | docker-container | `tcp://localhost:1234` |
| Parallelization | Matrix strategy | Manual (parallel/loop) |
| Secrets | GitHub secrets | Local credentials |
| Signing | Cosign + GitHub OIDC | Manual cosign |
| Attestations | GitHub Attestations API | Manual |

## Next Steps

Once local testing works:
1. Push to GitHub
2. Renovate updates Chart.yaml
3. `update-images` workflow updates values.yaml
4. `scan-and-patch` workflow patches images to GHCR
5. Images are signed and attested automatically
