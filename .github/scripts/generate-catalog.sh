#!/usr/bin/env bash
set -euo pipefail

# Collects patch results and generates vulnerability catalog
# Usage: generate-catalog.sh <patch-results-dir> <reports-dir> <registry> <output-file>

PATCH_RESULTS_DIR="${1:-patch-results}"
REPORTS_DIR="${2:-reports}"
REGISTRY="${3:-}"
OUTPUT_FILE="${4:-site/src/data/catalog.json}"

if [ -z "$REGISTRY" ]; then
  echo "Error: REGISTRY is required"
  exit 1
fi

echo "Collecting patch results from $PATCH_RESULTS_DIR..."

# Collect all patched image names
images_json='[]'
for result in "$PATCH_RESULTS_DIR"/*.json; do
  [ -f "$result" ] || continue
  name=$(jq -r .name "$result")
  source=$(jq -r .source "$result")
  target=$(jq -r .target "$result")
  images_json=$(echo "$images_json" | jq --arg n "$name" --arg s "$source" --arg t "$target" \
    '. += [{"name": $n, "source": $s, "target": $t}]')
done

mkdir -p .verity
echo "$images_json" > .verity/images.json

IMAGE_COUNT=$(jq 'length' .verity/images.json)
echo "✓ Collected $IMAGE_COUNT patched image(s)"

echo "Generating catalog..."
./verity catalog \
  --images-json .verity/images.json \
  --reports-dir "$REPORTS_DIR" \
  --registry "$REGISTRY" \
  --output "$OUTPUT_FILE"

echo "✓ Catalog generated at $OUTPUT_FILE"
