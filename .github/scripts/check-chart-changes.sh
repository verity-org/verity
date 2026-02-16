#!/bin/bash
set -euo pipefail

# Checks if charts should be published to OCI.
# Publishes if EITHER:
#   1. Local changes exist (new patches applied), OR
#   2. Chart version doesn't exist in OCI yet (needs initial publish with reports)
# Sets GITHUB_OUTPUT variable 'changes' to 'true' or 'false'.

REGISTRY="${REGISTRY:-ghcr.io/verity-org}"
has_local_changes=false
has_new_versions=false

# Check for local changes in charts/
if ! git diff --quiet charts/; then
  has_local_changes=true
  echo "✓ Local changes detected in charts/"
fi

# Check if any chart versions are missing from OCI
if [ -d "charts" ]; then
  for chart_yaml in charts/*/Chart.yaml; do
    [ -f "$chart_yaml" ] || continue

    chart_dir=$(dirname "$chart_yaml")
    chart_name=$(basename "$chart_dir")
    version=$(yq eval '.version' "$chart_yaml")

    if ! helm show chart "oci://${REGISTRY}/charts/${chart_name}" --version "${version}" &>/dev/null; then
      has_new_versions=true
      echo "✓ New version detected: ${chart_name}:${version} not in OCI"
    fi
  done
fi

if [ "$has_local_changes" = true ] || [ "$has_new_versions" = true ]; then
  echo "changes=true" >> "$GITHUB_OUTPUT"
  if [ "$has_local_changes" = true ]; then
    echo "Publishing: new patches applied"
  fi
  if [ "$has_new_versions" = true ]; then
    echo "Publishing: new chart versions need initial publish with reports"
  fi
else
  echo "changes=false" >> "$GITHUB_OUTPUT"
  echo "Skipping: no changes and all versions already in OCI"
fi
