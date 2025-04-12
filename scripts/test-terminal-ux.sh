#!/bin/bash
# Script to test terminal UX features including bracketed paste mode and interrupt handling

set -e  # Exit on any error

# Ensure we're in the right directory
cd "$(dirname "$0")/.." || exit 1

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

print_header() {
  echo -e "\n${YELLOW}==== $1 ====${NC}\n"
}

print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
  echo -e "${RED}✗ $1${NC}"
}

# Build the binary if needed
if [ ! -f ./cgpt ] || [ "$(find ./cmd/cgpt -name "*.go" -newer ./cgpt 2>/dev/null)" ]; then
  echo "Building cgpt..."
  go build -o cgpt ./cmd/cgpt
fi

# Create a temporary directory for test files
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

print_header "Testing Bracketed Paste Mode"

# Create a test file with large content
LARGE_FILE="$TEMP_DIR/large_input.txt"
for i in {1..100}; do
  echo "Line $i: This is a test line with some content to simulate a large paste operation." >> "$LARGE_FILE"
done

# Test 1: Simulate bracketed paste with large content
print_header "Test 1: Large Paste Simulation"
{
  # Send bracketed paste start marker
  printf "\033[200~"
  # Send content
  cat "$LARGE_FILE"
  # Send bracketed paste end marker
  printf "\033[201~\n"
} | ./cgpt --backend=dummy --model=dummy-model 2>&1 | tee "$TEMP_DIR/paste_output.txt"

if grep -q "Pasted" "$TEMP_DIR/paste_output.txt"; then
  print_success "Paste size indication detected"
else
  print_error "No paste size indication found"
fi

print_header "Testing Slow Responses"

# Test 2: Slow responses
print_header "Test 2: Slow Response Handling"
echo "Testing slow responses..." | ./cgpt --backend=dummy --model=dummy-model --slow-responses 2>&1 | tee "$TEMP_DIR/slow_output.txt"

if grep -q "dummy backend response" "$TEMP_DIR/slow_output.txt"; then
  print_success "Slow response completed successfully"
else
  print_error "Slow response test failed"
fi

print_header "Testing Interrupt Handling"

# Test 3: Interrupt handling (this is tricky to automate)
print_header "Test 3: Manual Interrupt Test"
echo "To test interrupt handling:"
echo "1. Run: ./cgpt --backend=dummy --model=dummy-model --slow-responses"
echo "2. Enter some text and press Enter"
echo "3. While response is streaming, press Ctrl+C"
echo "4. Verify [Interrupted] message appears"
echo "5. Press Ctrl+C at empty prompt to exit"

print_header "All automated tests completed"
echo "For complete testing, please run the manual interrupt test described above."
