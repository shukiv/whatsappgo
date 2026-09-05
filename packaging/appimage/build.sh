#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
appdir="${project_root}/build/AppDir"
tools_dir="${project_root}/build/appimage-tools"

cd "${project_root}"

mkdir -p "${appdir}/usr/bin" "${tools_dir}" "${project_root}/bin"
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "${project_root}/bin/whatsappd" "${project_root}/cmd/whatsappd"
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "${project_root}/bin/whatsappctl" "${project_root}/cmd/whatsappctl"
# Its own build directory, the way packaging/macos/build.sh has one: this
# configures with an install prefix of /usr and without the tests, neither of
# which belongs in the tree a developer builds in.
build_dir="${project_root}/desktop/build-appimage"
cmake -S "${project_root}/desktop" -B "${build_dir}" -G Ninja -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_TESTING=OFF
cmake --build "${build_dir}" --parallel
DESTDIR="${appdir}" cmake --install "${build_dir}"
install -Dm644 "${project_root}/packaging/metainfo/org.whatsappgo.Desktop.metainfo.xml" "${appdir}/usr/share/metainfo/org.whatsappgo.Desktop.metainfo.xml"

arch="$(uname -m)"
case "${arch}" in
  x86_64) tool_arch="x86_64" ;;
  aarch64) tool_arch="aarch64" ;;
  *) echo "Unsupported AppImage architecture: ${arch}" >&2; exit 1 ;;
esac

linuxdeploy="${tools_dir}/linuxdeploy-${tool_arch}.AppImage"
qt_plugin="${tools_dir}/linuxdeploy-plugin-qt-${tool_arch}.AppImage"
if [[ ! -x "${linuxdeploy}" ]]; then
  curl -fL "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${tool_arch}.AppImage" -o "${linuxdeploy}"
  chmod +x "${linuxdeploy}"
fi
if [[ ! -x "${qt_plugin}" ]]; then
  curl -fL "https://github.com/linuxdeploy/linuxdeploy-plugin-qt/releases/download/continuous/linuxdeploy-plugin-qt-${tool_arch}.AppImage" -o "${qt_plugin}"
  chmod +x "${qt_plugin}"
fi

export QML_SOURCES_PATHS="${project_root}/desktop/qml"
export EXTRA_PLATFORM_PLUGINS="libqwayland-egl.so;libqwayland-generic.so"
export LDAI_OUTPUT="${project_root}/build/WhatsAppGo-${tool_arch}.AppImage"
"${linuxdeploy}" --appdir "${appdir}" --desktop-file "${project_root}/packaging/applications/org.whatsappgo.Desktop.desktop" --plugin qt --output appimage
