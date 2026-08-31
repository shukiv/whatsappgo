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
        if ! dpkg-query -W -f='${Status}' libkirigami-dev 2>/dev/null | grep -q 'install ok installed'; then
            missing="${missing} KF6Kirigami2"
        fi
        install_hint="sudo apt-get update && sudo apt-get install -y build-essential cmake ninja-build pkg-config qt6-base-dev qt6-declarative-dev qt6-multimedia-dev libkirigami-dev extra-cmake-modules qml6-module-org-kde-kirigami qml6-module-qtquick-controls qml6-module-qtmultimedia"
        ;;
    fedora|rhel|centos)
        if ! rpm -q kf6-kirigami-devel >/dev/null 2>&1; then
            missing="${missing} KF6Kirigami2"
        fi
        install_hint="sudo dnf install -y gcc-c++ cmake ninja-build pkgconf-pkg-config qt6-qtbase-devel qt6-qtdeclarative-devel qt6-qtmultimedia-devel kf6-kirigami-devel"
        ;;
    arch|manjaro)
        install_hint="sudo pacman -S --needed base-devel cmake ninja pkgconf qt6-base qt6-declarative qt6-multimedia kirigami"
        ;;
    *)
        install_hint="Install CMake, a C++ compiler, pkg-config, Qt 6 Base/Declarative/Multimedia development files, and KDE Frameworks 6 Kirigami development files."
        ;;
esac

if [ -n "$missing" ]; then
    printf '%s\n' "Desktop build prerequisites are missing:${missing}" >&2
    printf '\n%s\n' "Install them with:" >&2
    printf '  %s\n\n' "$install_hint" >&2
    exit 1
fi
