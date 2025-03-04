#!/usr/bin/env bash
# sb-exec-example.sh - Example file showing common sb-exec usage patterns
#
# This script demonstrates different ways to use sb-exec for:
# - Running tests
# - Benchmarking
# - Environment configuration
# - Data processing
#
# Run with: ./sb-exec-example.sh

set -euo pipefail

echo "Running sb-exec examples..."

# Directory for this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SB_EXEC="${SCRIPT_DIR}/sb-exec.sh"

# Ensure sb-exec is executable
chmod +x "$SB_EXEC"

# Example 1: Basic usage - run a simple command
echo -e "\n\033[1;34mExample 1: Basic usage - run a simple command\033[0m"
"$SB_EXEC" --tag "basic-example" "echo 'Hello, sb-exec!' && ls -la"

# Example 2: Using a specific Docker image
echo -e "\n\033[1;34mExample 2: Using a specific Docker image\033[0m"
"$SB_EXEC" --tag "python-example" --image python:3.9-slim "python -c 'import sys; print(f\"Python version: {sys.version}\"); import platform; print(f\"Platform: {platform.platform()}\")'"

# Example 3: Running a performance test
echo -e "\n\033[1;34mExample 3: Running a performance test\033[0m"
"$SB_EXEC" --tag "performance-test" --image alpine:latest "time for i in \$(seq 1 10); do echo \$i; sleep 0.1; done"

# Example 4: Creating a temporary test file and running tests
echo -e "\n\033[1;34mExample 4: Creating a temporary test file and running tests\033[0m"
TEMP_DIR=$(mktemp -d)
cat > "${TEMP_DIR}/test_script.py" << 'EOF'
def add(a, b):
    return a + b

def test_add():
    assert add(2, 3) == 5
    assert add(-1, 1) == 0
    assert add(0, 0) == 0
    print("All tests passed!")

if __name__ == "__main__":
    test_add()
EOF

"$SB_EXEC" --tag "file-test" --image python:3.9-slim --volume "${TEMP_DIR}:/app" --workdir "/app" "python test_script.py"
rm -rf "$TEMP_DIR"

# Example 5: Running a Node.js application
echo -e "\n\033[1;34mExample 5: Running a Node.js application\033[0m"
TEMP_DIR=$(mktemp -d)
cat > "${TEMP_DIR}/app.js" << 'EOF'
const os = require('os');
console.log('Node.js Version:', process.version);
console.log('Hostname:', os.hostname());
console.log('Platform:', os.platform());
console.log('CPU Architecture:', os.arch());
console.log('CPU Cores:', os.cpus().length);
console.log('Memory (GB):', Math.round(os.totalmem() / 1024 / 1024 / 1024));
EOF

"$SB_EXEC" --tag "node-example" --image node:16-alpine --volume "${TEMP_DIR}:/app" --workdir "/app" "node app.js"
rm -rf "$TEMP_DIR"

# Example 6: Multiple commands with shell script
echo -e "\n\033[1;34mExample 6: Multiple commands with shell script\033[0m"
TEMP_DIR=$(mktemp -d)
cat > "${TEMP_DIR}/multi_step.sh" << 'EOF'
#!/bin/sh
echo "Step 1: Checking environment..."
uname -a
echo "Step 2: Creating test files..."
mkdir -p /tmp/test-data
for i in $(seq 1 5); do
  echo "File $i content" > "/tmp/test-data/file-$i.txt"
done
echo "Step 3: Processing files..."
find /tmp/test-data -type f | sort | xargs cat
echo "Step 4: Cleaning up..."
rm -rf /tmp/test-data
echo "All steps completed successfully!"
EOF
chmod +x "${TEMP_DIR}/multi_step.sh"

"$SB_EXEC" --tag "multi-step" --image ubuntu:latest --volume "${TEMP_DIR}:/scripts" "/scripts/multi_step.sh"
rm -rf "$TEMP_DIR"

# Display summary of executed examples
echo -e "\n\033[1;32mAll examples completed successfully!\033[0m"
echo "View the git notes for details on each execution:"
echo "  git notes --ref=basic-example show"
echo "  git notes --ref=python-example show"
echo "  git notes --ref=performance-test show"
echo "  git notes --ref=file-test show"
echo "  git notes --ref=node-example show"
echo "  git notes --ref=multi-step show"