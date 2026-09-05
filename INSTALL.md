# Installing WhatsAppGo

WhatsAppGo is built from source, and tagged versions are also published as
binaries on the [releases page](https://github.com/shukiv/whatsappgo/releases):
an AppImage for Linux, a ZIP and an installer for Windows, and a DMG for each
macOS architecture.

Those binaries are **unsigned**. macOS refuses an unsigned bundle that came from
another machine until it is allowed in System Settings, and Windows SmartScreen
warns about an unsigned installer. Building from source avoids both.

The desktop application and its Go backend are installed together. Start only
`whatsappgo`; it starts and stops its private `whatsappd` helper automatically.

## Staying up to date

A packaged build looks for a newer release every three hours and asks once when
it finds one. **Settings → Help** shows what this copy is and has the same
button, so nobody has to wait to be asked. On Linux the AppImage replaces itself
and the window reopens; on Windows the installer takes over; on macOS the disk
image opens for you to drag across.

A build made from source is not compared against the releases - it reports
itself as built from source and offers nothing, because a working copy is not
behind anything. `git pull` is the update there.

## Requirements

- Linux, Windows 10 or newer, or macOS 12 or newer
- Go 1.26 or newer
- CMake 3.22 or newer
- a C++20 compiler and `pkg-config`
- Qt 6.5 or newer: Base (including Widgets), Declarative/Quick, Quick Controls,
  Network, and Multimedia

If an older Go installation supports toolchain selection, leave
`GOTOOLCHAIN=auto` enabled so Go can obtain the version declared by `go.mod`.

### Debian, Ubuntu, Linux Mint, and Pop!_OS

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-org-kde-desktop \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

### Fedora and RHEL-family distributions

```bash
sudo dnf install -y gcc-c++ cmake ninja-build pkgconf-pkg-config \
  qt6-qtbase-devel qt6-qtdeclarative-devel qt6-qtmultimedia-devel \
  kf6-qqc2-desktop-style
```

### Arch Linux and Manjaro

```bash
sudo pacman -S --needed base-devel cmake ninja pkgconf \
  qt6-base qt6-declarative qt6-multimedia qqc2-desktop-style
```

The repository can check the native dependencies without changing the system:

```bash
make check-desktop-deps
```

`make check-desktop-deps` is a Linux check; on Windows and macOS the Qt
installer provides everything the build needs.

### Windows

Install Qt 6.5 or newer (Base, Declarative, Quick Controls, Network,
Multimedia), the Go toolchain, CMake and a C++ compiler. Then:

```powershell
powershell -ExecutionPolicy Bypass -File packaging\windows\build.ps1 -QtDir C:\Qt\6.8.2\msvc2022_64
```

That produces a portable ZIP under `build\`, and an installer as well when
[Inno Setup](https://jrsoftware.org/isinfo.php) is installed. Pass
`-SignThumbprint <thumbprint>` to sign every executable; unsigned builds run,
but SmartScreen warns about them.

The daemon and the desktop client talk over a named pipe rather than a
filesystem socket, so nothing appears under the runtime directory on Windows.

### macOS

Install Qt 6.5 or newer, the Go toolchain and the Xcode command line tools,
then:

```bash
CMAKE_PREFIX_PATH=~/Qt/6.8.2/macos packaging/macos/build.sh
```

That produces `build/macos/whatsappgo.app` and a DMG beside it. Signing and
notarisation run only when the environment provides credentials:

```bash
export MACOS_SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
export MACOS_NOTARY_PROFILE=whatsappgo   # from 'xcrun notarytool store-credentials'
packaging/macos/build.sh
```

Without them the bundle is unsigned: it runs on the machine that built it, and
Gatekeeper refuses it anywhere else.

### Notifications away from Linux

On Linux the daemon posts notifications to the desktop's own service, so they
appear whether or not the window is open. Windows and macOS route notifications
through the foreground application, so there the daemon hands the message to the
desktop client and notifications appear only while WhatsAppGo is running.

## Build and run without installing

```bash
git clone https://github.com/shukiv/whatsappgo.git
cd whatsappgo
make desktop
./desktop/build/whatsappgo
```

`make desktop` builds `bin/whatsappd`, `bin/whatsappctl`, the Qt application,
and runnable copies beside the application. No separate daemon setup or systemd
unit is required.

## Install for one user

This installs into `~/.local` and does not require root:

```bash
make check-desktop-deps
make tools
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

While WhatsAppGo is running, automation can use:

```bash
~/.local/bin/whatsappctl status
~/.local/bin/whatsappctl --pretty chats --limit 10
```

See the [command-line and bot API](docs/API.md) for sending, contact resolution,
event streaming, profiles, and the raw protocol.

If the command is not found by name, add `~/.local/bin` to `PATH`. You may need
to sign out and back in before a newly installed application-menu entry appears.

## Install system-wide

```bash
make check-desktop-deps
make tools
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
configuration. On Debian and Ubuntu, install the GNOME integration with:

```bash
sudo apt install gnome-shell-extension-appindicator
```

Enable **AppIndicator and KStatusNotifierItem Support** in the GNOME Extensions
application, then sign out and back in before restarting WhatsAppGo. The icon
registers even if the extension starts after WhatsAppGo. Minimizing hides the
window behind the icon; its menu provides **Open/Hide** and **Quit WhatsAppGo**.
If no tray host exists, minimizing remains a normal window-manager minimize and
closing exits safely. Native notifications continue through the desktop portal
in either case.

## First start

On the phone, open **WhatsApp → Settings → Linked devices → Link a device** and
scan the QR code displayed by WhatsAppGo. The first history sync can take time;
new messages are saved immediately in a per-profile SQLite database.

## Updating

Close WhatsAppGo, update the source tree, and repeat the build and install using
the same prefix:

```bash
git pull --ff-only
make tools
cmake --build desktop/build --parallel
cmake --install desktop/build
```

Use `sudo cmake --install desktop/build` only if the original installation used
a system prefix such as `/usr`.

## Uninstalling

Remove the files from the prefix used during installation:

- `bin/whatsappgo`
- `bin/whatsappd`
- `bin/whatsappctl`
- `share/applications/org.whatsappgo.Desktop.desktop`
- `share/icons/hicolor/scalable/apps/org.whatsappgo.Desktop.svg`

Uninstalling the binaries intentionally leaves account data in
`$XDG_DATA_HOME/whatsappgo` (normally `~/.local/share/whatsappgo`) and cached
media in `$XDG_CACHE_HOME/whatsappgo` (normally `~/.cache/whatsappgo`). Back up
the data directory while the application is closed before removing it.

See [Troubleshooting](docs/TROUBLESHOOTING.md) for graphics, pairing, history,
and media problems.
