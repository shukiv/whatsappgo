#!/usr/bin/env bash
# Builds WhatsAppGo.app, and optionally signs and notarises it into a DMG.
#
# Run on macOS with Qt 6.5 or newer and the Go toolchain installed. Point
# CMAKE_PREFIX_PATH at the Qt installation if cmake cannot find it, for example
#   CMAKE_PREFIX_PATH=~/Qt/6.8.2/macos packaging/macos/build.sh
#
# Signing and notarising are skipped unless the environment provides the
# credentials. Without them the bundle still runs locally, but Gatekeeper will
# refuse it on any other machine:
#   MACOS_SIGN_IDENTITY   "Developer ID Application: Name (TEAMID)"
#   MACOS_NOTARY_PROFILE  a profile stored with 'xcrun notarytool store-credentials'
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
build_dir="${project_root}/desktop/build-macos"
stage_dir="${project_root}/build/macos"
app="${stage_dir}/whatsappgo.app"

cd "${project_root}"

arch="$(uname -m)"
case "${arch}" in
  arm64) goarch="arm64" ;;
  x86_64) goarch="amd64" ;;
  *) echo "Unsupported macOS architecture: ${arch}" >&2; exit 1 ;;
esac

mkdir -p "${project_root}/bin" "${stage_dir}"
# Stamped, or the built bundle calls itself "dev" and never offers an update.
version="$("${project_root}/scripts/version.sh")"
# The daemon is pure Go, so it cross-builds without a C toolchain.
CGO_ENABLED=0 GOOS=darwin GOARCH="${goarch}" go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${project_root}/bin/whatsappd" ./cmd/whatsappd
CGO_ENABLED=0 GOOS=darwin GOARCH="${goarch}" go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${project_root}/bin/whatsappctl" ./cmd/whatsappctl

cmake -S "${project_root}/desktop" -B "${build_dir}" -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX="${stage_dir}" -DBUILD_TESTING=OFF
# Bounded by memory as well as cores; see scripts/build-jobs.sh.
cmake --build "${build_dir}" --parallel "$("${project_root}/scripts/build-jobs.sh")"
rm -rf "${app}"
cmake --install "${build_dir}"

# macdeployqt copies the Qt frameworks and the QML modules the application
# actually imports into the bundle. Without -qmldir it ships no QML plugins and
# the window comes up empty.
macdeployqt "${app}" -qmldir="${project_root}/desktop/qml"

if [[ -n "${MACOS_SIGN_IDENTITY:-}" ]]; then
  # The daemon and the CLI are separate Mach-O files inside the bundle and each
  # needs its own signature before the bundle as a whole can be signed.
  for helper in whatsappd whatsappctl; do
    codesign --force --options runtime --timestamp \
             --sign "${MACOS_SIGN_IDENTITY}" "${app}/Contents/MacOS/${helper}"
  done
  codesign --force --deep --options runtime --timestamp \
           --sign "${MACOS_SIGN_IDENTITY}" "${app}"
  codesign --verify --deep --strict --verbose=2 "${app}"
else
  echo "MACOS_SIGN_IDENTITY is not set; the bundle is unsigned and only runs on this machine." >&2
fi

dmg="${stage_dir}/WhatsAppGo-${goarch}.dmg"
rm -f "${dmg}"
hdiutil create -volname WhatsAppGo -srcfolder "${app}" -ov -format UDZO "${dmg}"

if [[ -n "${MACOS_NOTARY_PROFILE:-}" ]]; then
  xcrun notarytool submit "${dmg}" --keychain-profile "${MACOS_NOTARY_PROFILE}" --wait
  xcrun stapler staple "${dmg}"
else
  echo "MACOS_NOTARY_PROFILE is not set; the disk image is not notarised." >&2
fi

echo "Built ${app}"
echo "Built ${dmg}"
