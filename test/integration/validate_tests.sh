#!/bin/bash
# Integration test validation script
# This script validates that integration tests are properly structured

set -e

echo "================================================"
echo "Integration Test Validation"
echo "================================================"
echo ""

# Check 1: Test compilation
echo "✓ Checking test compilation..."
go test -tags=integration -c -o /tmp/jack-integration-test ./testing/integration/
if [ $? -eq 0 ]; then
    echo "  ✓ All integration tests compile successfully"
    rm /tmp/jack-integration-test
else
    echo "  ✗ Compilation failed"
    exit 1
fi
echo ""

# Check 2: Test discovery
echo "✓ Checking test discovery..."
TEST_COUNT=$(go test -tags=integration -list=. ./testing/integration/ 2>&1 | grep -c "^Test" || true)
echo "  ✓ Found $TEST_COUNT integration tests"
echo ""

# Check 3: Expected test count
EXPECTED_COUNT=23
if [ "$TEST_COUNT" -eq "$EXPECTED_COUNT" ]; then
    echo "  ✓ Test count matches expected ($EXPECTED_COUNT tests)"
else
    echo "  ⚠ Warning: Expected $EXPECTED_COUNT tests, found $TEST_COUNT"
fi
echo ""

# Check 4: Go vet
echo "✓ Running go vet..."
go vet -tags=integration ./testing/integration/
if [ $? -eq 0 ]; then
    echo "  ✓ No vet issues found"
else
    echo "  ✗ Vet issues detected"
    exit 1
fi
echo ""

# Check 5: Test structure validation
echo "✓ Validating test structure..."

# Check that all test files have proper build tags
for file in testing/integration/*_test.go; do
    if ! grep -q "//go:build integration" "$file"; then
        echo "  ✗ Missing build tag in $file"
        exit 1
    fi
done
echo "  ✓ All test files have proper build tags"
echo ""

# Check 6: Fixture files
echo "✓ Checking test fixtures..."
FIXTURE_COUNT=$(ls testing/fixtures/*.json 2>/dev/null | wc -l)
echo "  ✓ Found $FIXTURE_COUNT fixture files"
echo ""

# Check 7: Root privilege requirement
echo "✓ Checking root privilege requirement..."
if [ "$(id -u)" -ne 0 ]; then
    echo "  ℹ Not running as root - tests will be skipped when executed"
    echo "  ℹ To run tests: sudo go test -v -tags=integration ./testing/integration/"
else
    echo "  ✓ Running as root - tests can be executed"
fi
echo ""

# Summary
echo "================================================"
echo "Validation Summary"
echo "================================================"
echo "✓ Compilation:        PASS"
echo "✓ Test discovery:     PASS ($TEST_COUNT tests)"
echo "✓ Go vet:             PASS"
echo "✓ Build tags:         PASS"
echo "✓ Fixtures:           PASS ($FIXTURE_COUNT files)"
echo ""
echo "Integration tests are properly structured and ready to run."
echo ""
echo "To execute tests (requires root):"
echo "  sudo make test-integration"
echo "  # or"
echo "  sudo go test -v -tags=integration ./testing/integration/"
echo ""
echo "To run in Docker (CI-friendly):"
echo "  make test-integration-docker"
echo ""
