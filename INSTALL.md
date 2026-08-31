# Installing WhatsAppGo

WhatsAppGo is currently distributed as source. The repository contains Debian,
RPM, Flatpak, and AppImage packaging definitions, but no signed binary release
is published yet.

The desktop application and its Go backend are installed together. Start only
`whatsappgo`; it starts and stops its private `whatsappd` helper automatically.

## Requirements

- Linux
- Go 1.26 or newer
- CMake 3.22 or newer
- a C++20 compiler and `pkg-config`
- Qt 6.5 or newer: Base (including Widgets), Declarative/Quick, Quick Controls,
  Network, and Multimedia
- KDE Frameworks 6 Kirigami

If an older Go installation supports toolchain selection, leave
`GOTOOLCHAIN=auto` enabled so Go can obtain the version declared by `go.mod`.

### Debian, Ubuntu, Linux Mint, and Pop!_OS

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  libkirigami-dev extra-cmake-modules \
  qml6-module-org-kde-kirigami qml6-module-org-kde-desktop \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

### Fedora and RHEL-family distributions

```bash
sudo dnf install -y gcc-c++ cmake ninja-build pkgconf-pkg-config \
  qt6-qtbase-devel qt6-qtdeclarative-devel qt6-qtmultimedia-devel \
  kf6-kirigami-devel
```

### Arch Linux and Manjaro

```bash
sudo pacman -S --needed base-devel cmake ninja pkgconf \
  qt6-base qt6-declarative qt6-multimedia kirigami
```

The repository can check the native dependencies without changing the system:

```bash
make check-desktop-deps
```

## Build and run without installing

```bash
git clone https://github.com/shukiv/whatsappgo.git
cd whatsappgo
make desktop
./desktop/build/whatsappgo
```

`make desktop` builds `bin/whatsappd`, the Qt application, and a runnable copy
of the helper beside the application. No separate daemon setup or systemd unit
is required.

## Install for one user

This installs into `~/.local` and does not require root:

```bash
make check-desktop-deps
make daemon
cmake -S desktop -B desktop/build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$HOME/.local"
cmake --build desktop/build --parallel
cmake --install desktop/build
```

Launch it from the application menu or run:

```bash
~/.local/bin/whatsappgo
```

If the command is not found by name, add `~/.local/bin` to `PATH`. You may need
to sign out and back in before a newly installed application-menu entry appears.

## Install system-wide

```bash
make check-desktop-deps
make daemon
cmake -S desktop -B desktop/build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX=/usr
cmake --build desktop/build --parallel
sudo cmake --install desktop/build
```

Then run `whatsappgo` or select WhatsAppGo in the application menu.

### GNOME top-bar icon

WhatsAppGo registers a standard Linux system-tray/status icon. GNOME Shell does
not display status icons by itself; enable a StatusNotifier/AppIndicator shell
extension if your GNOME installation does not already provide one. Other
desktops with a standard system tray normally show the icon without extra
configuration. The application continues to use the desktop notification
service or portal when no tray host is available.

## First start

On the phone, open **WhatsApp → Settings → Linked devices → Link a device** and
scan the QR code displayed by WhatsAppGo. The first history sync can take time;
new messages are saved immediately in a per-profile SQLite database.

## Updating

Close WhatsAppGo, update the source tree, and repeat the build and install using
the same prefix:

```bash
git pull --ff-only
make daemon
cmake --build desktop/build --parallel
cmake --install desktop/build
```

Use `sudo cmake --install desktop/build` only if the original installation used
a system prefix such as `/usr`.

## Uninstalling

Remove the files from the prefix used during installation:

- `bin/whatsappgo`
- `bin/whatsappd`
- `share/applications/org.whatsappgo.Desktop.desktop`
- `share/icons/hicolor/scalable/apps/org.whatsappgo.Desktop.svg`

Uninstalling the binaries intentionally leaves account data in
`$XDG_DATA_HOME/whatsappgo` (normally `~/.local/share/whatsappgo`) and cached
media in `$XDG_CACHE_HOME/whatsappgo` (normally `~/.cache/whatsappgo`). Back up
the data directory while the application is closed before removing it.

See [Troubleshooting](docs/TROUBLESHOOTING.md) for graphics, pairing, history,
and media problems.
