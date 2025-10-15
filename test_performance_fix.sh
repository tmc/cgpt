#!/bin/bash

# Test script to verify the performance fix for readline SplitByLine function

echo "Testing cgpt performance fix..."
echo "Building cgpt with readline fixes..."

# Build cgpt
go build ./cmd/cgpt || exit 1

# Create test data files
echo "Creating test data..."
echo -n $(printf '%*s' 2000 '' | tr ' ' 'A') > /tmp/test_2k.txt
echo -n $(printf '%*s' 4000 '' | tr ' ' 'A') > /tmp/test_4k.txt
echo -n $(printf '%*s' 8000 '' | tr ' ' 'A') > /tmp/test_8k.txt
echo -n $(printf '%*s' 16000 '' | tr ' ' 'A') > /tmp/test_16k.txt

# Test function
test_size() {
    local size=$1
    local file="/tmp/test_${size}.txt"
    echo -n "Testing ${size} characters: "

    # Time the cgpt invocation with dummy backend to avoid API calls
    start_time=$(perl -MTime::HiRes=time -E 'say time')
    cat "$file" | timeout 10s ./cgpt --backend dummy --config /dev/null >/dev/null 2>&1
    end_time=$(perl -MTime::HiRes=time -E 'say time')

    duration=$(echo "$end_time - $start_time" | bc -l)
    printf "%.3f seconds\n" "$duration"

    # Check if it took too long (original bug would cause >2s for 4k chars)
    if (( $(echo "$duration > 2.0" | bc -l) )); then
        echo "  ⚠️  Still slow - may need more optimization"
    else
        echo "  ✅ Performance looks good"
    fi
}

echo ""
echo "Performance test results:"
echo "========================"
test_size "2k"
test_size "4k"
test_size "8k"
test_size "16k"

echo ""
echo "Test completed. If all tests show <0.5s, the fix is working correctly."