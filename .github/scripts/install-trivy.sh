#!/bin/bash
set -euo pipefail

echo "Installing Trivy..."
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
trivy --version
echo "✅ Trivy installed successfully"
