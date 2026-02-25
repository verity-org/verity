#!/usr/bin/env bash
set -euo pipefail

# Runs Copa dry-run discovery and generates matrix JSON for GitHub Actions
# Usage: copa-discover.sh <config-file> <reports-dir> <output-file>

CONFIG_FILE="${1:-copa-config.yaml}"
REPORTS_DIR="${2:-reports}"
OUTPUT_FILE="${3:-$GITHUB_OUTPUT}"

echo "Running Copa discovery..."
copa patch \
  --config "$CONFIG_FILE" \
  --report "$REPORTS_DIR" \
  --dry-run \
  --output-json results.json

# Filter to only images that need patching
jq -c '[.[] | select(.status == "WouldPatch") | {name, source, target}]' results.json > matrix.json

count=$(jq 'length' matrix.json)
if [ "$count" -gt 0 ]; then
  echo "has_images=true" >> "$OUTPUT_FILE"
  echo "matrix={\"include\":$(cat matrix.json)}" >> "$OUTPUT_FILE"
  echo "✓ Found $count image(s) that need patching:"
  jq -r '.[] | "  - \(.name): \(.source) -> \(.target)"' matrix.json
else
  echo "has_images=false" >> "$OUTPUT_FILE"
  echo "✓ No images need patching (all up to date)"
fi
