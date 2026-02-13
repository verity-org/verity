#!/bin/bash
set -euo pipefail

CHARTS_DIR="${1:-charts}"

echo "Checking for changes in ${CHARTS_DIR}/..."

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

if git diff --quiet "${CHARTS_DIR}/"; then
  echo "No changes to commit"
  exit 0
fi

echo "Changes detected, committing..."
git add "${CHARTS_DIR}/"
# Also stage standalone image reports if they exist
if [ -d "${CHARTS_DIR}/_standalone/reports" ]; then
  git add "${CHARTS_DIR}/_standalone/reports/"
fi
git commit -m "chore: patch images for updated chart versions

Automatically patched container images using Copa after chart version update.

Co-Authored-By: github-actions[bot] <github-actions[bot]@users.noreply.github.com>"

echo "Pushing changes..."
git push

echo "✅ Changes committed and pushed successfully"
