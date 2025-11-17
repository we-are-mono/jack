.PHONY: test lint lint-unused coverage coverage-unit coverage-integration coverage-merge check build clean install-tools deadcode deadcode-test clean-check test-integration

# Run all tests (unit tests only)
test:
	@echo "Running unit tests..."
	go test -v -race ./...

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
coverage-integration:
	@echo "==> Building test container..."
	@docker build -t jack-integration-test -f Dockerfile.test . >/dev/null
	@echo "==> Running integration tests with coverage in container..."
	@mkdir -p coverage-data
	@docker run --rm --privileged --cap-add=ALL \
		-v $(PWD)/coverage-data:/coverage-data \
		jack-integration-test \
		sh -c 'go test -v -tags=integration -covermode=atomic -coverprofile=/coverage-data/integration.out -coverpkg=./... ./test/integration/...'
	@cp coverage-data/integration.out coverage-integration.out
	@echo ""
	@echo "Integration test coverage:"
	@go tool cover -func=coverage-integration.out | grep total || echo "No coverage data"

# Merge coverage from unit and integration tests
coverage-merge: coverage-unit coverage-integration
	@echo ""
	@echo "==> Merging coverage profiles..."
	@# Properly merge coverage files (sum coverage counts for same lines)
	@go run ./test/tools/mergecoverage.go coverage-unit.out coverage-integration.out > coverage-combined.out
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

# Run integration tests in Docker
test-integration:
	@echo "Building test container..."
	docker build -t jack-integration-test -f Dockerfile.test .
	@echo "Running integration tests in container..."
	docker run --rm --privileged --cap-add=ALL jack-integration-test \
		go test -v -tags=integration -timeout=10m ./test/integration/...

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
	@echo "  test-integration        - Run integration tests in Docker"
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
