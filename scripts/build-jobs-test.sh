#!/bin/sh
# Checks the job count build-jobs.sh reports for machines of a few shapes.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
failures=0

check() {
    description="$1"
    expected="$2"
    actual="$3"
    if [ "${actual}" != "${expected}" ]; then
        echo "FAILED: ${description}: expected ${expected}, got ${actual}" >&2
        failures=$((failures + 1))
    fi
}

# 16 cores and 13 GiB available: memory allows 13, so cores are the bound.
check "a machine with memory to spare uses its cores" 13 "$("${here}/build-jobs.sh" 16 13631488)"
# The build that started this: 16 cores, and unbounded make took the machine
# into swap. Memory is the bound.
check "memory bounds a machine with many cores" 3 "$("${here}/build-jobs.sh" 16 3670016)"
# Fewer cores than the memory would allow.
check "cores bound a small machine with plenty of memory" 2 "$("${here}/build-jobs.sh" 2 16777216)"
# Never zero, however little is left.
check "one compiler runs even when memory is short" 1 "$("${here}/build-jobs.sh" 8 262144)"

if [ "${failures}" -ne 0 ]; then
    exit 1
fi
echo "build-jobs.sh reports a sensible job count"
