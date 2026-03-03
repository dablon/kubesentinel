#!/bin/bash
set -e

echo "=== KubeSentinel E2E Test ==="

echo "1. Building binary..."
go build -o kubesentinel ./cmd

echo "2. Running unit tests..."
go test -v -cover ./internal/...

echo "3. Running CLI test..."
./kubesentinel scan -n production

echo "4. Testing JSON output..."
./kubesentinel scan -n production -o json

echo "=== E2E Tests PASSED ==="
