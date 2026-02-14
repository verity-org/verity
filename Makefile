.PHONY: help build test lint fmt vet sec clean install-tools

# Default target
help:
	@echo "Available targets:"
	@echo "  make build         - Build the verity binary"
	@echo "  make test          - Run tests"
	@echo "  make lint          - Run all linters"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make sec           - Run security scanner (gosec)"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install-tools - Install development tools"

# Build binary
build:
	go build -o verity .

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	gofmt -s -w .
	goimports -w .

# Run go vet
vet:
	go vet ./...

# Run security scanner
sec:
	@which gosec > /dev/null || (echo "gosec not found. Run: make install-tools" && exit 1)
	gosec -quiet ./...

# Run golangci-lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run: make install-tools" && exit 1)
	golangci-lint run --timeout=5m

# Lint workflows
lint-workflows:
	@which actionlint > /dev/null || (echo "actionlint not found. Run: make install-tools" && exit 1)
	actionlint

# Lint YAML files
lint-yaml:
	@which yamllint > /dev/null || (echo "yamllint not found. Run: pip install yamllint" && exit 1)
	yamllint .

# Lint shell scripts
lint-shell:
	@which shellcheck > /dev/null || (echo "shellcheck not found. Install from: https://shellcheck.net" && exit 1)
	shellcheck .github/scripts/*.sh

# Run all quality checks
quality: fmt vet lint lint-workflows lint-yaml lint-shell sec test
	@echo "✓ All quality checks passed!"

# Clean build artifacts
clean:
	rm -f verity
	rm -f coverage.out coverage.html
	rm -rf .verity/
	rm -rf site/dist/

# Install development tools
install-tools:
	@echo "Installing Go tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing actionlint..."
	go install github.com/rhysd/actionlint/cmd/actionlint@latest
	@echo ""
	@echo "✓ Go tools installed!"
	@echo ""
	@echo "Additional tools (install separately):"
	@echo "  - yamllint: pip install yamllint"
	@echo "  - shellcheck: brew install shellcheck (macOS) or apt install shellcheck (Linux)"
