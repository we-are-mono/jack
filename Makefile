.PHONY: test lint lint-unused coverage coverage-unit coverage-integration coverage-merge check build clean install-tools deadcode deadcode-test clean-check test-integration docker-base-image test-docker-combined serve-coverage

# Run all tests (unit tests only)
test:
	@echo "Running unit tests..."
	go test -v -race ./...

# Build cached Docker base image (only when it doesn't exist)
docker-base-image:
	@if ! docker image inspect jack-integration-base >/dev/null 2>&1; then \
		echo "Building cached base image (this happens once)..."; \
		docker build -t jack-integration-base -f Dockerfile.test . && \
		echo "✓ Base image built and cached"; \
	else \
		echo "✓ Base image already exists (cached)"; \
	fi

# Minimum coverage threshold (percentage)
# Unit tests: 35.2% | Integration tests: ~15-20% | Combined: ~50-55%
MIN_COVERAGE=50

# Run unit tests with coverage
coverage-unit:
	@echo "==> Running unit tests with coverage..."
	@go test -v -race -coverprofile=coverage-unit.out ./...
	@echo ""
	@echo "Unit test coverage:"
	@go tool cover -func=coverage-unit.out | grep total || echo "No coverage data"

# Run integration tests with coverage (requires Docker)
coverage-integration: docker-base-image
	@echo "==> Building binaries locally..."
	@mkdir -p bin
	@go build -o jack
	@cd plugins/core/nftables && go build -o ../../../bin/jack-plugin-nftables .
	@cd plugins/core/wireguard && go build -o ../../../bin/jack-plugin-wireguard .
	@cd plugins/core/dnsmasq && go build -o ../../../bin/jack-plugin-dnsmasq .
	@cd plugins/core/monitoring && go build -o ../../../bin/jack-plugin-monitoring .
	@cd plugins/core/leds && go build -o ../../../bin/jack-plugin-leds .
	@cd plugins/core/sqlite3 && CGO_ENABLED=1 go build -o ../../../bin/jack-plugin-sqlite3 .
	@echo "==> Running integration tests with coverage in container..."
	@mkdir -p coverage-data
	@docker run --rm --privileged --cap-add=ALL \
		-v $(PWD):/opt/jack \
		-v $(PWD)/bin:/usr/lib/jack/plugins \
		-v $(PWD)/coverage-data:/coverage-data \
		-w /opt/jack \
		jack-integration-base \
		sh -c 'go mod download && go test -v -p=1 -tags=integration -covermode=atomic -coverprofile=/coverage-data/integration.out -coverpkg=./... ./test/integration/...'
	@cp coverage-data/integration.out coverage-integration.out
	@echo ""
	@echo "Integration test coverage:"
	@go tool cover -func=coverage-integration.out | grep total || echo "No coverage data"

# Merge coverage from unit and integration tests
coverage-merge: coverage-unit coverage-integration
	@echo ""
	@echo "==> Merging coverage profiles..."
	@if [ ! -f $(HOME)/go/bin/gocovmerge ]; then \
		echo "Error: gocovmerge not found. Install with: go install github.com/wadey/gocovmerge@latest"; \
		exit 1; \
	fi
	@$(HOME)/go/bin/gocovmerge coverage-unit.out coverage-integration.out > coverage-combined.out
	@echo "==> Generating combined coverage report..."
	@go tool cover -html=coverage-combined.out -o coverage.html
	@echo ""
	@echo "Combined coverage summary:"
	@go tool cover -func=coverage-combined.out | grep total

# Run all coverage (unit + integration + merge + threshold check)
coverage: coverage-merge
	@echo ""
	@echo "==> Checking coverage threshold..."
	@COVERAGE=$$(go tool cover -func=coverage-combined.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	COVERAGE_INT=$$(echo "$$COVERAGE" | awk -F. '{print $$1}'); \
	echo "Coverage: $$COVERAGE% (minimum: $(MIN_COVERAGE)%)"; \
	if [ $$COVERAGE_INT -lt $(MIN_COVERAGE) ]; then \
		echo "❌ FAILED: Coverage $$COVERAGE% is below minimum $(MIN_COVERAGE)%"; \
		exit 1; \
	else \
		echo "✅ PASSED: Coverage $$COVERAGE% meets minimum $(MIN_COVERAGE)%"; \
	fi
	@echo ""
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	$(HOME)/go/bin/golangci-lint run ./...

# Run linter focusing only on unused code detection
lint-unused:
	@echo "Running unused code detection..."
	$(HOME)/go/bin/golangci-lint run --enable-only=unused,unparam,ineffassign,wastedassign ./...

# Run deadcode analysis for unreachable functions
deadcode:
	@echo "Running deadcode analysis (unreachable functions)..."
	@if [ ! -f $${HOME}/go/bin/deadcode ]; then \
		echo "Error: deadcode not installed. Run 'make install-tools' first."; \
		exit 1; \
	fi
	$${HOME}/go/bin/deadcode ./...

# Run deadcode analysis including tests
deadcode-test:
	@echo "Running deadcode analysis (including tests)..."
	@if [ ! -f $${HOME}/go/bin/deadcode ]; then \
		echo "Error: deadcode not installed. Run 'make install-tools' first."; \
		exit 1; \
	fi
	$${HOME}/go/bin/deadcode -test ./...

# Run quick check (lint + test)
check: lint test

# Full check with coverage (run before deployment)
check-full: lint coverage

# Comprehensive check: lint + deadcode + coverage
clean-check: lint deadcode coverage
	@echo ""
	@echo "======================================"
	@echo "✓ All checks passed!"
	@echo "======================================"

# Run integration tests in Docker (without coverage)
test-integration: docker-base-image
	@echo "==> Building binaries locally..."
	@mkdir -p bin
	@go build -o jack
	@cd plugins/core/nftables && go build -o ../../../bin/jack-plugin-nftables .
	@cd plugins/core/wireguard && go build -o ../../../bin/jack-plugin-wireguard .
	@cd plugins/core/dnsmasq && go build -o ../../../bin/jack-plugin-dnsmasq .
	@cd plugins/core/monitoring && go build -o ../../../bin/jack-plugin-monitoring .
	@cd plugins/core/leds && go build -o ../../../bin/jack-plugin-leds .
	@cd plugins/core/sqlite3 && CGO_ENABLED=1 go build -o ../../../bin/jack-plugin-sqlite3 .
	@echo "==> Running integration tests in container..."
	@docker run --rm --privileged --cap-add=ALL \
		-v $(PWD):/opt/jack \
		-v $(PWD)/bin:/usr/lib/jack/plugins \
		-w /opt/jack \
		jack-integration-base \
		sh -c 'go mod download && go test -v -p=1 -tags=integration -timeout=10m ./test/integration/...'

# Run both unit and integration tests in Docker with combined coverage
test-docker-combined: docker-base-image
	@echo "==> Building binaries locally..."
	@mkdir -p bin
	@go build -o jack
	@cd plugins/core/nftables && go build -o ../../../bin/jack-plugin-nftables .
	@cd plugins/core/wireguard && go build -o ../../../bin/jack-plugin-wireguard .
	@cd plugins/core/dnsmasq && go build -o ../../../bin/jack-plugin-dnsmasq .
	@cd plugins/core/monitoring && go build -o ../../../bin/jack-plugin-monitoring .
	@cd plugins/core/leds && go build -o ../../../bin/jack-plugin-leds .
	@cd plugins/core/sqlite3 && CGO_ENABLED=1 go build -o ../../../bin/jack-plugin-sqlite3 .
	@echo "==> Running unit and integration tests in Docker with coverage..."
	@mkdir -p coverage-data
	@docker run --rm --privileged --cap-add=ALL \
		-v $(PWD):/opt/jack \
		-v $(PWD)/bin:/usr/lib/jack/plugins \
		-v $(PWD)/coverage-data:/coverage-data \
		-w /opt/jack \
		jack-integration-base \
		sh -c ' \
			echo "==> Running unit tests with coverage..."; \
			go test -v -race -covermode=atomic -coverprofile=/coverage-data/unit.out -coverpkg=./... ./...; \
			echo ""; \
			echo "==> Running integration tests with coverage..."; \
			go test -v -p=1 -tags=integration -covermode=atomic -coverprofile=/coverage-data/integration.out -coverpkg=./... ./test/integration/...; \
			echo ""; \
			echo "==> Merging coverage profiles with gocovmerge..."; \
			/go/bin/gocovmerge /coverage-data/unit.out /coverage-data/integration.out > /coverage-data/combined.out \
		'
	@cp coverage-data/combined.out coverage-combined.out
	@echo ""
	@echo "==> Generating HTML coverage reports..."
	@go tool cover -html=coverage-data/unit.out -o coverage-data/unit.html
	@go tool cover -html=coverage-data/integration.out -o coverage-data/integration.html
	@go tool cover -html=coverage-data/combined.out -o coverage-data/combined.html
	@echo "==> Generating coverage index page..."
	@UNIT_COV=$$(go tool cover -func=coverage-data/unit.out | grep total | awk '{print $$3}'); \
	INTEGRATION_COV=$$(go tool cover -func=coverage-data/integration.out | grep total | awk '{print $$3}'); \
	COMBINED_COV=$$(go tool cover -func=coverage-combined.out | grep total | awk '{print $$3}'); \
	cat > coverage-data/index.html <<EOF \
<!DOCTYPE html> \
<html> \
<head> \
    <title>Jack Coverage Reports</title> \
    <meta charset="utf-8"> \
    <style> \
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 40px auto; max-width: 800px; line-height: 1.6; color: #333; } \
        h1 { color: #2c3e50; border-bottom: 3px solid #3498db; padding-bottom: 10px; } \
        .report { background: #f8f9fa; border-left: 4px solid #3498db; padding: 20px; margin: 20px 0; border-radius: 4px; } \
        .report h2 { margin-top: 0; color: #2c3e50; } \
        .coverage { font-size: 2em; font-weight: bold; color: #27ae60; } \
        .coverage.low { color: #e74c3c; } \
        .coverage.medium { color: #f39c12; } \
        a { color: #3498db; text-decoration: none; font-weight: 500; } \
        a:hover { text-decoration: underline; } \
        .stats { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 15px; margin: 30px 0; } \
        .stat { background: white; border: 1px solid #ddd; padding: 15px; text-align: center; border-radius: 4px; } \
        .stat-value { font-size: 2.5em; font-weight: bold; margin: 10px 0; } \
        .stat-label { color: #7f8c8d; font-size: 0.9em; text-transform: uppercase; } \
        footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; color: #7f8c8d; font-size: 0.9em; } \
    </style> \
</head> \
<body> \
    <h1>Jack Test Coverage Reports</h1> \
    <div class="stats"> \
        <div class="stat"> \
            <div class="stat-label">Unit Tests</div> \
            <div class="stat-value">$$UNIT_COV</div> \
        </div> \
        <div class="stat"> \
            <div class="stat-label">Integration Tests</div> \
            <div class="stat-value">$$INTEGRATION_COV</div> \
        </div> \
        <div class="stat"> \
            <div class="stat-label">Combined</div> \
            <div class="stat-value">$$COMBINED_COV</div> \
        </div> \
    </div> \
    <div class="report"> \
        <h2>📊 Combined Coverage</h2> \
        <p>Merged coverage from both unit and integration tests.</p> \
        <p><a href="combined.html">View Combined Coverage Report →</a></p> \
    </div> \
    <div class="report"> \
        <h2>🧪 Unit Tests</h2> \
        <p>Code coverage from unit tests only (no integration tests).</p> \
        <p><a href="unit.html">View Unit Test Coverage →</a></p> \
    </div> \
    <div class="report"> \
        <h2>🐳 Integration Tests</h2> \
        <p>Code coverage from integration tests running in Docker.</p> \
        <p><a href="integration.html">View Integration Test Coverage →</a></p> \
    </div> \
    <footer> \
        <p>Generated by <code>make test-docker-combined</code></p> \
    </footer> \
</body> \
</html> \
EOF
	@echo ""
	@echo "Combined coverage summary:"
	@go tool cover -func=coverage-combined.out | grep total
	@echo ""
	@echo "Coverage files:"
	@echo "  - Unit: coverage-data/unit.out → coverage-data/unit.html"
	@echo "  - Integration: coverage-data/integration.out → coverage-data/integration.html"
	@echo "  - Combined: coverage-combined.out → coverage-data/combined.html"
	@echo ""
	@echo "Run 'make serve-coverage' to view reports in browser"

# Serve coverage reports via HTTP
serve-coverage:
	@if [ ! -d coverage-data ]; then \
		echo "Error: coverage-data directory not found. Run 'make test-docker-combined' first."; \
		exit 1; \
	fi
	@if [ ! -f coverage-data/combined.html ]; then \
		echo "Error: Coverage HTML reports not found. Run 'make test-docker-combined' first."; \
		exit 1; \
	fi
	@if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "Error: Port 8080 is already in use."; \
		echo ""; \
		echo "Process using port 8080:"; \
		lsof -Pi :8080 -sTCP:LISTEN; \
		echo ""; \
		echo "To kill the process, run: kill \$$(lsof -t -i:8080)"; \
		exit 1; \
	fi
	@echo "Starting HTTP server on http://localhost:8080"
	@echo ""
	@echo "📊 Coverage reports available at:"
	@echo "  ➜ http://localhost:8080/ (Coverage overview)"
	@echo "  ➜ http://localhost:8080/combined.html (Combined coverage)"
	@echo "  ➜ http://localhost:8080/unit.html (Unit tests only)"
	@echo "  ➜ http://localhost:8080/integration.html (Integration tests only)"
	@echo ""
	@echo "Press Ctrl+C to stop server"
	@echo ""
	@cd coverage-data && python3 -m http.server 8080

# Build binaries
build:
	@echo "Building..."
	./build.sh

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/deadcode@latest
	@echo "Tools installed!"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f coverage*.out coverage.html
	rm -rf coverage-data/
	rm -f jack
	rm -rf bin/

# Help
help:
	@echo "Available targets:"
	@echo "  test                    - Run unit tests"
	@echo "  test-integration        - Run integration tests in Docker (fast, cached)"
	@echo "  test-docker-combined    - Run unit + integration tests in Docker with merged coverage"
	@echo "  serve-coverage          - Serve coverage HTML reports via HTTP (port 8080)"
	@echo "  docker-base-image       - Build cached Docker base image (auto-built when needed)"
	@echo "  coverage                - Run combined unit + integration coverage (enforces $(MIN_COVERAGE)% minimum)"
	@echo "  coverage-unit           - Run unit tests with coverage only"
	@echo "  coverage-integration    - Run integration tests with coverage only (requires Docker)"
	@echo "  coverage-merge          - Merge unit and integration coverage profiles"
	@echo "  lint                    - Run linter (all checks)"
	@echo "  lint-unused             - Run unused code detection only"
	@echo "  deadcode                - Find unreachable functions"
	@echo "  deadcode-test           - Find unreachable functions (including tests)"
	@echo "  check                   - Quick check (lint + test)"
	@echo "  check-full              - Full check with coverage"
	@echo "  clean-check             - Comprehensive check (lint + deadcode + coverage)"
	@echo "  build                   - Build binaries"
	@echo "  install-tools           - Install development tools (golangci-lint, deadcode)"
	@echo "  clean                   - Clean build artifacts"
