#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
appdir="${project_root}/build/AppDir"
tools_dir="${project_root}/build/appimage-tools"

cd "${project_root}"

mkdir -p "${appdir}/usr/bin" "${tools_dir}" "${project_root}/bin"
# Stamped, or the built AppImage calls itself "dev" and never offers an update.
version="$("${project_root}/scripts/version.sh")"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${project_root}/bin/whatsappd" "${project_root}/cmd/whatsappd"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${project_root}/bin/whatsappctl" "${project_root}/cmd/whatsappctl"
# Its own build directory, the way packaging/macos/build.sh has one: this
# configures with an install prefix of /usr and without the tests, neither of
# which belongs in the tree a developer builds in.
build_dir="${project_root}/desktop/build-appimage"
# Animated stickers are decoded by libwebp, and a build without it tells the
# reader they are unavailable. A published AppImage must not be that build.
cmake -S "${project_root}/desktop" -B "${build_dir}" -G Ninja -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_TESTING=OFF -DWHATSAPPGO_REQUIRE_WEBP=ON
# Bounded by memory as well as cores: an unbounded -j starts one compiler
# per ready target and takes a 16 GB machine into swap.
cmake --build "${build_dir}" --parallel "$("${project_root}/scripts/build-jobs.sh")"
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

# Without a platform theme Qt has no native file chooser and falls back to
# drawing its own, which is not the dialog the rest of the desktop uses. The
# portal theme reaches the desktop's own chooser over D-Bus; the GTK theme
# would mean bundling GTK itself. Copy it in rather than hoping the Qt plugin
# notices it: shipping an image without one is the bug this guards against.
portal_theme=""
for candidate in \
  "$(qmake6 -query QT_INSTALL_PLUGINS 2>/dev/null)/platformthemes/libqxdgdesktopportal.so" \
  "/usr/lib/${arch}-linux-gnu/qt6/plugins/platformthemes/libqxdgdesktopportal.so" \
  "/usr/lib/qt6/plugins/platformthemes/libqxdgdesktopportal.so"; do
  if [[ -f "${candidate}" ]]; then portal_theme="${candidate}"; break; fi
done
if [[ -z "${portal_theme}" ]]; then
  echo "libqxdgdesktopportal.so not found: install qt6-xdgdesktopportal-platformtheme" >&2
  echo "(without it the app draws its own file dialog instead of the desktop's)" >&2
  exit 1
fi
install -Dm644 "${portal_theme}" "${appdir}/usr/plugins/platformthemes/libqxdgdesktopportal.so"

export QML_SOURCES_PATHS="${project_root}/desktop/qml"
export EXTRA_PLATFORM_PLUGINS="libqwayland-egl.so;libqwayland-generic.so"
export LDAI_OUTPUT="${project_root}/build/WhatsAppGo-${tool_arch}.AppImage"
"${linuxdeploy}" --appdir "${appdir}" --desktop-file "${project_root}/packaging/applications/org.whatsappgo.Desktop.desktop" --plugin qt --output appimage
