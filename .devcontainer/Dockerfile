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

# Pre-install project tools from mise.toml
COPY mise.toml /tmp/mise.toml
RUN cd /tmp && mise trust && mise install
