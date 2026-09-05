#!/bin/sh
# Prints how many compilers this machine can run at once without swapping.
#
# `cmake --build --parallel` with no number passes make a bare -j, which means
# unlimited: make starts a compiler for every target whose inputs are ready.
# This project has hundreds, one C++ compiler here peaks around 350 MB, and the
# result on a 16 GB desktop was 40 of them at once, 13 GB resident, the machine
# in swap and the session frozen.
#
# Cores are the usual bound; memory is the one that actually bites here.
#
# Usage: build-jobs.sh [cores] [available KiB]   (arguments are for the test)
set -eu

cores="${1:-}"
if [ -z "${cores}" ]; then
    cores="$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 2)"
fi

available_kib="${2:-}"
if [ -z "${available_kib}" ]; then
    if [ -r /proc/meminfo ]; then
        available_kib="$(awk '/^MemAvailable:/ {print $2; exit}' /proc/meminfo)"
    else
        # macOS has no MemAvailable. Physical memory is the closest honest
        # answer, and the halving below keeps some of it for everything else.
        available_kib="$(( $(sysctl -n hw.memsize 2>/dev/null || echo 4294967296) / 2048 ))"
    fi
fi

# One GiB per compiler. The largest translation units in this build - the
# generated QML type registration and the moc output for rpcclient - are the
# ones that get close to it.
#
# Three quarters of what is available, not all of it: MemAvailable counts page
# cache the kernel believes it can reclaim, and a desktop session with a
# browser open needs room to keep working while the build runs. The machine
# this was written for froze hard, so the headroom is worth the minute.
by_memory="$(( available_kib * 3 / 4 / 1048576 ))"
[ "${by_memory}" -lt 1 ] && by_memory=1

jobs="${cores}"
[ "${by_memory}" -lt "${jobs}" ] && jobs="${by_memory}"
echo "${jobs}"
