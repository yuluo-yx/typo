#!/bin/bash

set -e

cd "$(dirname "$0")/.."

echo "=== Typo BenchMark ==="
echo "Env: $(go env GOOS)/$(go env GOARCH)"
echo "CPU: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'unknown')"
echo ""

go test -bench=. -benchmem ./benchmarks/ -run=^$ -count=1
