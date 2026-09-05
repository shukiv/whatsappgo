#!/usr/bin/env bash
# Checks scripts/version.sh. Run by CI.
#
# The stamp decides whether the update check works at all, and it is easy to
# break without noticing: every build still succeeds, it just calls itself
# "dev" and never offers an update.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failures=0

check() {
  local what="$1" want="$2" got="$3"
  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${what}: wanted '${want}', got '${got}'" >&2
    failures=$((failures + 1))
  fi
}

scratch="$(mktemp -d)"
cleanup() {
  [[ -d "${scratch}" ]] || return 0
  command -v trash >/dev/null 2>&1 && trash "${scratch}" >/dev/null 2>&1 && return 0
  rm -rf "${scratch}"
}
trap cleanup EXIT

check "the environment wins" "v9.9.9" "$(WHATSAPPGO_VERSION=v9.9.9 "${here}/version.sh")"

# A tagged checkout stamps the tag. This builds a throwaway repository rather
# than depending on whatever tags this one happens to have.
repo="${scratch}/repo"
mkdir -p "${repo}/scripts"
cp "${here}/version.sh" "${repo}/scripts/version.sh"
git -C "${repo}" init --quiet
tree="$(git -C "${repo}" write-tree)"
head="$(GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
  git -C "${repo}" commit-tree "${tree}" -m first)"
git -C "${repo}" update-ref HEAD "${head}"
git -C "${repo}" tag v4.5.6 "${head}"
check "a tagged checkout" "v4.5.6" "$(cd "${repo}" && ./scripts/version.sh)"

# Outside git there is nothing to describe, and the build still has to be
# named something.
outside="${scratch}/outside"
mkdir -p "${outside}/scripts"
cp "${here}/version.sh" "${outside}/scripts/version.sh"
check "no git" "dev" "$(cd "${outside}" && GIT_CEILING_DIRECTORIES="${scratch}" ./scripts/version.sh)"

if (( failures > 0 )); then
  echo "${failures} version.sh check(s) failed" >&2
  exit 1
fi
echo "version.sh checks passed"
