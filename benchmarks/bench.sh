#!/bin/bash

# Typo 性能测试脚本

set -e

cd "$(dirname "$0")/.."

echo "=== Typo 性能测试 ==="
echo "环境: $(go env GOOS)/$(go env GOARCH)"
echo "CPU: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'unknown')"
echo ""

go test -bench=. -benchmem ./benchmarks/ -run=^$ -count=1
