FROM mcr.microsoft.com/devcontainers/base:debian

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    git \
    jq \
    unzip \
    && rm -rf /var/lib/apt/lists/*

USER vscode

# Install mise (runtime version manager)
RUN curl https://mise.run | sh

# Add mise shims and bin to PATH for non-interactive shells
ENV PATH="/home/vscode/.local/share/mise/shims:/home/vscode/.local/bin:${PATH}"

# Activate mise for interactive shells
RUN echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc

# Pre-install project tools (keep in sync with mise.toml)
RUN mise use -g go@1.26 node@22 golangci-lint@2
