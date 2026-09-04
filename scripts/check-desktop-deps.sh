#!/usr/bin/env bash

set -eu

missing=""

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        missing="${missing} $1"
    fi
}

require_command cmake
require_command c++
require_command pkg-config

if command -v pkg-config >/dev/null 2>&1; then
    for module in Qt6Core Qt6Gui Qt6Quick Qt6Network Qt6Multimedia; do
        if ! pkg-config --exists "$module"; then
            missing="${missing} $module"
        fi
    done
fi

if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
else
    ID=""
fi

case "${ID:-}" in
    debian|ubuntu|linuxmint|pop)
        install_hint="sudo apt-get update && sudo apt-get install -y build-essential cmake ninja-build pkg-config qt6-base-dev qt6-declarative-dev qt6-multimedia-dev qml6-module-org-kde-desktop qml6-module-qtquick-controls qml6-module-qtmultimedia"
        ;;
    fedora|rhel|centos)
        install_hint="sudo dnf install -y gcc-c++ cmake ninja-build pkgconf-pkg-config qt6-qtbase-devel qt6-qtdeclarative-devel qt6-qtmultimedia-devel qt6-qtquickcontrols2 kf6-qqc2-desktop-style"
        ;;
    arch|manjaro)
        install_hint="sudo pacman -S --needed base-devel cmake ninja pkgconf qt6-base qt6-declarative qt6-multimedia qqc2-desktop-style"
        ;;
    *)
        install_hint="Install CMake, a C++ compiler, pkg-config, and the Qt 6 Base/Declarative/Multimedia development files."
        ;;
esac

if [ -n "$missing" ]; then
    printf '%s\n' "Desktop build prerequisites are missing:${missing}" >&2
    printf '\n%s\n' "Install them with:" >&2
    printf '  %s\n\n' "$install_hint" >&2
    exit 1
fi
