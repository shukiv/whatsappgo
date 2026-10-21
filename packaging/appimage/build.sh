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
# portal theme reaches the desktop's own chooser over D-Bus. Both are copied
# in rather than hoping the Qt plugin notices them: shipping an image without
# a platform theme is the bug this guards against.
#
# The image must work on whatever the reader is running, so it carries two:
#
#   libqxdgdesktopportal.so  asks xdg-desktop-portal, which hands the request
#                            to the desktop's own chooser. Right answer where
#                            a portal is running, and the only one that works
#                            under a sandbox.
#   libqgtk3.so              the GTK chooser, used as the base theme when no
#                            portal answers. A portal is not universal: it can
#                            be absent, or installed and failing to start, and
#                            then the portal theme alone leaves Qt drawing its
#                            own dialog.
#
# GTK itself is NOT bundled. libqgtk3.so links against the host's GTK, which
# the AppImage excludelist leaves out of the image, so there is no second GTK
# to clash with the one the desktop is already using. A host without GTK 3
# simply fails to load the plugin and falls through to the portal.
find_platform_theme() {
  local plugin="$1" candidate
  for candidate in \
    "$(qmake6 -query QT_INSTALL_PLUGINS 2>/dev/null)/platformthemes/${plugin}" \
    "/usr/lib/${arch}-linux-gnu/qt6/plugins/platformthemes/${plugin}" \
    "/usr/lib/qt6/plugins/platformthemes/${plugin}"; do
    if [[ -f "${candidate}" ]]; then printf '%s' "${candidate}"; return 0; fi
  done
  return 1
}
portal_theme="$(find_platform_theme libqxdgdesktopportal.so)" || {
  echo "libqxdgdesktopportal.so not found: install qt6-xdgdesktopportal-platformtheme" >&2
  echo "(without it the app draws its own file dialog instead of the desktop's)" >&2
  exit 1
}
gtk_theme="$(find_platform_theme libqgtk3.so)" || {
  echo "libqgtk3.so not found: install qt6-gtk-platformtheme" >&2
  echo "(without it a desktop with no running portal gets the app's own dialog)" >&2
  exit 1
}
install -Dm644 "${portal_theme}" "${appdir}/usr/plugins/platformthemes/libqxdgdesktopportal.so"
install -Dm644 "${gtk_theme}" "${appdir}/usr/plugins/platformthemes/libqgtk3.so"

export QML_SOURCES_PATHS="${project_root}/desktop/qml"
export EXTRA_PLATFORM_PLUGINS="libqwayland-egl.so;libqwayland-generic.so"
export LDAI_OUTPUT="${project_root}/build/WhatsAppGo-${tool_arch}.AppImage"
"${linuxdeploy}" --appdir "${appdir}" --desktop-file "${project_root}/packaging/applications/org.whatsappgo.Desktop.desktop" --plugin qt --output appimage
