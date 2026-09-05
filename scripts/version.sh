#!/usr/bin/env bash
# Prints the version to stamp into the binaries.
#
# Every build path asks this script so that they all get the same answer. An
# unstamped build calls itself "dev", and internal/updates never tells a dev
# build that it is out of date - so a release built without this stamp would
# quietly have no update check at all.
#
#   WHATSAPPGO_VERSION=v1.2.0 scripts/version.sh   -> v1.2.0
#   (a tagged checkout)                            -> v1.2.0
#   (a working copy, or no git at all)             -> a commit id, or dev
#
# A commit id does not parse as a version, which is the intent: somebody
# running their own build is not behind a release.
set -euo pipefail

if [[ -n "${WHATSAPPGO_VERSION:-}" ]]; then
  printf '%s\n' "${WHATSAPPGO_VERSION}"
  exit 0
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
described="$(git -C "${root}" describe --tags --always --dirty 2>/dev/null || true)"
printf '%s\n' "${described:-dev}"
