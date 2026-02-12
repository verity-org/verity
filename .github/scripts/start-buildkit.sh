#!/bin/bash
set -euo pipefail

BUILDKIT_VERSION="${1:-v0.19.0}"

echo "Starting BuildKit ${BUILDKIT_VERSION}..."
docker run --detach --privileged --name buildkitd \
  --entrypoint buildkitd --rm "moby/buildkit:${BUILDKIT_VERSION}"

echo "Waiting for BuildKit to be ready..."
for i in $(seq 1 30); do
  if docker exec buildkitd buildctl debug workers >/dev/null 2>&1; then
    echo "✅ BuildKit ready"
    exit 0
  fi
  echo "  Waiting... ($i/30)"
  sleep 1
done

echo "❌ BuildKit failed to start"
exit 1
