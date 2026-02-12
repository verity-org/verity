# Workflow Architecture

Verity uses a self-maintaining workflow system to automatically scan, patch, and publish security-hardened Helm charts.

## Overview

```
┌─────────────────┐
│   Chart.yaml    │  ← Renovate updates versions
└────────┬────────┘
         │
    ┌────▼─────────────────────────────────────┐
    │                                           │
    │  Scheduled Scan (Daily)    PR on Update  │
    │  Patch on New Vulns        Patch Charts  │
    │       │                          │        │
    │       ▼                          ▼        │
    │   Creates PR              Commits to PR   │
    │       │                          │        │
    └───────┼──────────────────────────┼────────┘
            │                          │
            └──────────┬───────────────┘
                       │
                  Merge to main
                       │
                       ▼
            ┌──────────────────────┐
            │  Publish to GHCR     │
            │  - Charts (OCI)      │
            │  - Images (Docker)   │
            └──────────────────────┘
```

## Workflows

### 1. CI Workflow (`ci.yaml`)

**Triggers:** Pull requests (except for `charts/**` and `**.md` changes)

**Purpose:** Validate code changes and ensure patching functionality works

**Jobs:**
- **Unit Tests** - Run Go tests
- **Scan** - Discover images in Chart.yaml dependencies
- **Validate Patching** - Test full patch pipeline and validate wrapper charts

**Does NOT:**
- Commit changes
- Publish anything
- Modify the PR

---

### 2. Scheduled Scan (`scheduled-scan.yaml`)

**Triggers:**
- Cron: Daily at 2 AM UTC
- Manual: `workflow_dispatch`

**Purpose:** Continuously monitor for new vulnerabilities

**Flow:**
1. Checkout main branch
2. Run verity with patching enabled
3. Check if any charts changed
4. If changes detected:
   - Create new branch `security/scheduled-patch-<run-number>`
   - Commit updated wrapper charts
   - Open PR with security label

**Use Case:** New CVE discovered for existing chart version → Auto-PR created

---

### 3. Patch on PR (`patch-on-pr.yaml`)

**Triggers:** Pull requests that modify `Chart.yaml`

**Purpose:** Automatically patch images when Renovate (or other bots) update chart versions

**Flow:**
1. Checkout PR branch
2. Run verity with patching
3. Commit patched charts to **the same PR branch**
4. Add comment to PR confirming patching complete

**Use Case:** Renovate bumps prometheus from 25.8.0 → 25.9.0 → Patching runs → PR is complete with wrapper charts

**Important:** Only runs when PR comes from the same repository (security measure)

---

### 4. Publish (`publish.yaml`)

**Triggers:** Push to `main` branch

**Purpose:** Publish wrapper charts and patched images to GHCR

**Flow:**
1. Find all wrapper charts (`charts/*-verity/`)
2. For each chart:
   - Build Helm dependencies
   - Package chart
   - Push to `oci://ghcr.io/<org>/charts/<chart-name>`
3. Verify patched images exist in GHCR
4. Generate and upload chart index

**Publishes:**
- Helm charts to `ghcr.io/<org>/charts/`
- Patched images already pushed during PR workflows

---

## Self-Maintenance Flow

### Scenario 1: Renovate Updates Chart Version

```
1. Renovate opens PR: Chart.yaml updated (prometheus 25.8.0 → 25.9.0)
   ↓
2. patch-on-pr.yaml triggers
   ↓
3. Verity patches all images for prometheus 25.9.0
   ↓
4. Patched charts committed to Renovate's PR branch
   ↓
5. Renovate's PR now includes wrapper chart with patched images
   ↓
6. Review & merge PR
   ↓
7. publish.yaml publishes to GHCR
```

### Scenario 2: New CVE Discovered

```
1. Scheduled scan runs nightly
   ↓
2. Trivy finds new fixable vulnerability
   ↓
3. Copa patches affected images
   ↓
4. Wrapper charts updated with new patches
   ↓
5. PR auto-created with security label
   ↓
6. Review & merge PR
   ↓
7. publish.yaml publishes updated charts to GHCR
```

### Scenario 3: Manual Change to Chart.yaml

```
1. Developer updates Chart.yaml manually
   ↓
2. Opens PR
   ↓
3. ci.yaml validates changes
   ↓
4. patch-on-pr.yaml runs and adds patched charts
   ↓
5. Review & merge
   ↓
6. publish.yaml publishes to GHCR
```

## Permissions Required

### GitHub Token Permissions

All workflows use `GITHUB_TOKEN` with these permissions:

- `contents: write` - For scheduled-scan and patch-on-pr to commit
- `pull-requests: write` - For scheduled-scan to create PRs
- `packages: write` - For GHCR image and chart publishing

### GHCR Access

Workflows authenticate to GHCR using:
```bash
echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io --username "${{ github.actor }}" --password-stdin
echo "${{ secrets.GITHUB_TOKEN }}" | helm registry login ghcr.io --username "${{ github.actor }}" --password-stdin
```

## Configuration

### Registry Settings

Patched images are pushed to:
```
ghcr.io/${{ github.repository_owner }}/<image-name>:<tag>-patched
```

Charts are published to:
```
oci://ghcr.io/${{ github.repository_owner }}/charts/<chart-name>
```

### Scheduled Scan Frequency

Edit `.github/workflows/scheduled-scan.yaml`:
```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM UTC
```

Common schedules:
- `'0 */6 * * *'` - Every 6 hours
- `'0 0 * * 0'` - Weekly on Sunday
- `'0 2 * * 1-5'` - Weekdays at 2 AM

### Artifact Retention

Vulnerability reports are retained for 30 days:
```yaml
retention-days: 30
```

Chart index retained for 90 days.

## Consuming Published Charts

### Install from GHCR

```bash
# Install patched prometheus
helm install my-prometheus \
  oci://ghcr.io/descope/charts/prometheus-verity \
  --version 1.0.0

# With custom values
helm install my-prometheus \
  oci://ghcr.io/descope/charts/prometheus-verity \
  --version 1.0.0 \
  -f my-values.yaml
```

### List Available Charts

Download the chart index artifact from the latest publish workflow run.

## Workflow Dependencies

### Required Tools

Installed automatically in workflows:
- Go (from `go.mod`)
- Trivy (latest)
- Copa (latest)
- Helm 3
- BuildKit (v0.19.0)
- Docker
- yq
- jq

### GitHub Actions

Third-party actions used:
- `actions/checkout@v4`
- `actions/setup-go@v5`
- `actions/upload-artifact@v4`
- `peter-evans/create-pull-request@v6` (scheduled-scan only)
- `actions/github-script@v7` (patch-on-pr only)

## Security Considerations

### PR from Forks

`patch-on-pr.yaml` includes safety check:
```yaml
if: github.event.pull_request.head.repo.full_name == github.repository
```

This prevents malicious PRs from forks from triggering patching with write access.

### Image Trust

Patched images are:
1. Built from original images pulled from upstream
2. Scanned with Trivy
3. Patched with Copa (official Microsoft tool)
4. Pushed to your GHCR with `-patched` suffix
5. Never modify original upstream images

### Secrets

No additional secrets required beyond the default `GITHUB_TOKEN`. All authentication uses GitHub's built-in token with appropriate scopes.

## Troubleshooting

### Scheduled scan creates PR but no changes

This can happen if:
- Vulnerabilities exist but are not fixable
- Copa skips images with no OS-level patches
- All images already patched

Check the workflow logs and Trivy reports in artifacts.

### Patch workflow fails on PR

Common causes:
- BuildKit failed to start
- GHCR authentication failed
- Out of disk space (patch workflow is resource-intensive)
- Chart dependency download failed

Check specific job logs for errors.

### Published charts not visible in GHCR

Ensure:
1. Workflow completed successfully
2. Package visibility is set to public in GitHub settings
3. You're using the correct OCI URL format

### Renovate PR doesn't trigger patching

Verify:
1. Renovate modified `Chart.yaml` file
2. PR is from same repository (not a fork)
3. Workflow file `patch-on-pr.yaml` exists on the target branch

## Monitoring

### Check Workflow Status

```bash
# List recent workflow runs
gh workflow list

# View specific workflow runs
gh run list --workflow=scheduled-scan.yaml

# Check latest scheduled scan
gh run view --workflow=scheduled-scan.yaml
```

### Review Vulnerability Reports

Trivy JSON reports are uploaded as artifacts on every patch run:
1. Go to Actions tab
2. Select workflow run
3. Download trivy-reports artifact
4. Review JSON files for vulnerability details

## Extending

### Add More Charts

Simply add to `Chart.yaml`:
```yaml
dependencies:
  - name: prometheus
    version: "25.8.0"
    repository: oci://ghcr.io/prometheus-community/charts
  - name: grafana
    version: "7.0.0"
    repository: https://grafana.github.io/helm-charts
```

All workflows automatically handle multiple charts.

### Custom Patching Logic

Modify `main.go` to add custom logic before/after patching. The workflow calls `./verity` so any changes to the binary are automatically used.

### Additional Publish Targets

Edit `publish.yaml` to add more registries:
```yaml
- name: Push to additional registry
  run: |
    helm push chart.tgz oci://another-registry.com/charts
```
