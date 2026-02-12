#!/bin/bash
set -euo pipefail

echo "Installing Copa..."
COPA_VERSION=$(curl -s https://api.github.com/repos/project-copacetic/copacetic/releases/latest | jq -r .tag_name | sed 's/^v//')
echo "Latest Copa version: ${COPA_VERSION}"

curl -fsSL -o copa.tar.gz "https://github.com/project-copacetic/copacetic/releases/download/v${COPA_VERSION}/copa_${COPA_VERSION}_linux_amd64.tar.gz"
tar -xzf copa.tar.gz copa
sudo mv copa /usr/local/bin/
rm copa.tar.gz

copa --version
echo "✅ Copa installed successfully"
