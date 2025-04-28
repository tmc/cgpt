#!/bin/bash
# Script to run interactive terminal UX tests

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
if [ ! -f ./cgpt ] || [ "$(find cmd/cgpt -type f -name "*.go" -newer cgpt 2>/dev/null)" ]; then
  echo "Building cgpt..."
  go build -o cgpt ./cmd/cgpt
fi

# Record HTTP requests if flag is set
RECORD_FLAG=""
if [ "$1" == "--record" ]; then
  RECORD_FLAG="--httprecord=.*"
  shift
fi

# Default paths
HTTP_RECORD_FILE="./cmd/cgpt/testdata/terminal_ux_test.httprr"
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Display help
if [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
  echo "Usage: $0 [--record] [test-type] [options]"
  echo ""
  echo "Options:"
  echo "  --record        Record HTTP interactions to replay file"
  echo "  --help, -h      Show this help message"
  echo ""
  echo "Test types:"
  echo "  paste           Test bracketed paste mode handling"
  echo "  slow            Test slow responses"
  echo "  interrupt       Test interrupt handling (manual)"
  echo "  all             Run all automated tests (default)"
  echo ""
  echo "Examples:"
  echo "  - Run all tests:           ./scripts/terminal-ux-test.sh"
  echo "  - Record mode:             ./scripts/terminal-ux-test.sh --record"
  echo "  - Test paste handling:     ./scripts/terminal-ux-test.sh paste"
  echo "  - Test slow responses:     ./scripts/terminal-ux-test.sh slow"
  echo ""
  exit 0
fi

# Determine which test to run
TEST_TYPE="all"
if [ -n "$1" ] && [[ "$1" != -* ]]; then
  TEST_TYPE="$1"
  shift
fi

# Run the appropriate test(s)
case "$TEST_TYPE" in
  "paste")
    print_header "Testing Bracketed Paste Mode"
    
    # Create a test file with large content
    LARGE_FILE="$TEMP_DIR/large_input.txt"
    for i in {1..100}; do
      echo "Line $i: This is a test line with some content to simulate a large paste operation." >> "$LARGE_FILE"
    done
    
    # Simulate bracketed paste
    {
      # Send bracketed paste start marker
      printf "\033[200~"
      # Send content
      cat "$LARGE_FILE"
      # Send bracketed paste end marker
      printf "\033[201~\n"
    } | ./cgpt --backend=dummy --model=dummy-model $RECORD_FLAG "$@" 2>&1 | tee "$TEMP_DIR/paste_output.txt"
    
    if grep -q "Pasted" "$TEMP_DIR/paste_output.txt" || grep -q "dummy backend response" "$TEMP_DIR/paste_output.txt"; then
      print_success "Paste test completed successfully"
    else
      print_error "Paste test failed"
    fi
    ;;
    
  "slow")
    print_header "Testing Slow Responses"
    echo "Testing slow responses..." | ./cgpt --backend=dummy --model=dummy-model --slow-responses $RECORD_FLAG "$@" 2>&1 | tee "$TEMP_DIR/slow_output.txt"
    
    if grep -q "dummy backend response" "$TEMP_DIR/slow_output.txt"; then
      print_success "Slow response test completed successfully"
    else
      print_error "Slow response test failed"
    fi
    ;;
    
  "interrupt")
    print_header "Testing Interrupt Handling (Manual)"
    echo "To test interrupt handling:"
    echo "1. Run: ./cgpt --backend=dummy --model=dummy-model --slow-responses $RECORD_FLAG"
    echo "2. Enter some text and press Enter"
    echo "3. While response is streaming, press Ctrl+C"
    echo "4. Verify [Interrupted] message appears"
    echo "5. Press Ctrl+C at empty prompt to exit"
    
    read -p "Press Enter to start the manual test..."
    ./cgpt --backend=dummy --model=dummy-model --slow-responses $RECORD_FLAG "$@"
    ;;
    
  "all"|*)
    print_header "Running All Terminal UX Tests"
    
    # Test 1: Bracketed paste mode
    print_header "Test 1: Bracketed Paste Mode"
    
    # Create a test file with large content
    LARGE_FILE="$TEMP_DIR/large_input.txt"
    for i in {1..50}; do
      echo "Line $i: This is a test line with some content to simulate a large paste operation." >> "$LARGE_FILE"
    done
    
    # Simulate bracketed paste
    {
      # Send bracketed paste start marker
      printf "\033[200~"
      # Send content
      cat "$LARGE_FILE"
      # Send bracketed paste end marker
      printf "\033[201~\n"
    } | ./cgpt --backend=dummy --model=dummy-model $RECORD_FLAG "$@" 2>&1 | tee "$TEMP_DIR/paste_output.txt"
    
    if grep -q "Pasted" "$TEMP_DIR/paste_output.txt" || grep -q "dummy backend response" "$TEMP_DIR/paste_output.txt"; then
      print_success "Paste test completed successfully"
    else
      print_error "Paste test failed"
    fi
    
    # Test 2: Slow responses
    print_header "Test 2: Slow Responses"
    echo "Testing slow responses..." | ./cgpt --backend=dummy --model=dummy-model --slow-responses $RECORD_FLAG "$@" 2>&1 | tee "$TEMP_DIR/slow_output.txt"
    
    if grep -q "dummy backend response" "$TEMP_DIR/slow_output.txt"; then
      print_success "Slow response test completed successfully"
    else
      print_error "Slow response test failed"
    fi
    
    # Test 3: HTTP record/replay
    print_header "Test 3: HTTP Record/Replay"
    echo "Using HTTP record file: $HTTP_RECORD_FILE"
    
    if [ -n "$RECORD_FLAG" ]; then
      echo "Recording HTTP interactions..."
      echo "Test HTTP recording" | ./cgpt --backend=dummy --http-record="$HTTP_RECORD_FILE" "$@" 2>&1 | tee "$TEMP_DIR/http_output.txt"
    else
      echo "Replaying HTTP interactions..."
      echo "Test HTTP replay" | ./cgpt --backend=dummy --http-record="$HTTP_RECORD_FILE" "$@" 2>&1 | tee "$TEMP_DIR/http_output.txt"
    fi
    
    if grep -q "dummy backend response" "$TEMP_DIR/http_output.txt"; then
      print_success "HTTP record/replay test completed successfully"
    else
      print_error "HTTP record/replay test failed"
    fi
    
    print_header "Manual Interrupt Test"
    echo "To test interrupt handling:"
    echo "1. Run: ./cgpt --backend=dummy --model=dummy-model --slow-responses $RECORD_FLAG"
    echo "2. Enter some text and press Enter"
    echo "3. While response is streaming, press Ctrl+C"
    echo "4. Verify [Interrupted] message appears"
    echo "5. Press Ctrl+C at empty prompt to exit"
    ;;
esac

print_header "Tests completed"
