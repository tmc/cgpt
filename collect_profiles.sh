#!/bin/bash

# Script to simultaneously collect multiple pprof profiles from cgpt

DURATION=${1:-10}
OUTPUT_DIR=${2:-/tmp}
PPROF_HOST=${3:-localhost:6060}

echo "Collecting pprof profiles for ${DURATION} seconds from ${PPROF_HOST}..."
echo "Output directory: ${OUTPUT_DIR}"

# Create timestamped filename prefix
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
PREFIX="${OUTPUT_DIR}/cgpt_${TIMESTAMP}"

echo "Starting simultaneous profile collection..."
echo "Note: Mutex and block profiling should be enabled in cgpt when pprof server starts"

# Start all profiles in parallel
curl -s "http://${PPROF_HOST}/debug/pprof/profile?seconds=${DURATION}" > "${PREFIX}_cpu.pprof" &
CPU_PID=$!

curl -s "http://${PPROF_HOST}/debug/pprof/mutex?seconds=${DURATION}" > "${PREFIX}_mutex.pprof" &
MUTEX_PID=$!

curl -s "http://${PPROF_HOST}/debug/pprof/block?seconds=${DURATION}" > "${PREFIX}_block.pprof" &
BLOCK_PID=$!

curl -s "http://${PPROF_HOST}/debug/pprof/goroutine" > "${PREFIX}_goroutine.pprof" &
GOROUTINE_PID=$!

curl -s "http://${PPROF_HOST}/debug/pprof/heap" > "${PREFIX}_heap.pprof" &
HEAP_PID=$!

# Start execution trace
curl -s "http://${PPROF_HOST}/debug/pprof/trace?seconds=${DURATION}" > "${PREFIX}_trace.out" &
TRACE_PID=$!

echo "Collection started. PIDs: CPU=${CPU_PID}, MUTEX=${MUTEX_PID}, BLOCK=${BLOCK_PID}, GOROUTINE=${GOROUTINE_PID}, HEAP=${HEAP_PID}, TRACE=${TRACE_PID}"
echo "Waiting ${DURATION} seconds for collection to complete..."

# Wait for all background processes to complete
wait $CPU_PID
wait $MUTEX_PID
wait $BLOCK_PID
wait $GOROUTINE_PID
wait $HEAP_PID
wait $TRACE_PID

echo "Profile collection complete!"
echo ""
echo "Generated files:"
ls -la "${PREFIX}"*

echo ""
echo "Analysis commands:"
echo "  CPU:       pprof -top ${PREFIX}_cpu.pprof"
echo "  Mutex:     pprof -top ${PREFIX}_mutex.pprof"
echo "  Blocking:  pprof -top ${PREFIX}_block.pprof"
echo "  Goroutine: pprof -top ${PREFIX}_goroutine.pprof"
echo "  Heap:      pprof -top ${PREFIX}_heap.pprof"
echo "  Trace:     go tool trace ${PREFIX}_trace.out"
echo ""
echo "Web UI:    pprof -http=:8080 ${PREFIX}_cpu.pprof"