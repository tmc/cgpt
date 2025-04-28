#!/bin/bash
# Example script for testing terminal UX

# Test simple output
echo "Hello from the test script"

# Test multiline output
cat << EOF
This is a multiline
output from the
test script
EOF

# Test error output
echo "This is an error message" >&2

# Test exit code
exit 0
