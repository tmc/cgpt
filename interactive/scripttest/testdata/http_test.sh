#!/bin/bash
# Test script for HTTP recording/replaying

echo "Making HTTP request to example.com"
curl -s https://example.com
echo "Request completed"

# Test another request
echo "Making HTTP request to httpbin.org"
curl -s https://httpbin.org/get
echo "Second request completed"

exit 0
