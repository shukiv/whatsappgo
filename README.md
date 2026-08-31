# WhatsAppGo

WhatsAppGo is an unofficial, native WhatsApp client for Linux. Its interface is
built with Qt 6, QML, and Kirigami. A bundled Go backend uses
[whatsmeow](https://github.com/tulir/whatsmeow) to connect through WhatsApp's
multi-device protocol. There is no Chromium, WebKit, Electron, or WhatsApp Web
page in the application.

The desktop application starts and owns its backend automatically. Build it,
launch `whatsappgo`, and use it as one application—there is no service or
systemd unit to start manually.

> **Important:** WhatsAppGo is not affiliated with, authorized by, or endorsed
> by WhatsApp or Meta. Protocol changes can interrupt it, and use of an
> unofficial client may carry account risk. Voice and video calls cannot be
> placed because the underlying protocol library does not implement them.

## Quick start

On Debian 13:

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  libkirigami-dev extra-cmake-modules \
  qml6-module-org-kde-kirigami qml6-module-org-kde-desktop \
  qml6-module-qtquick-controls qml6-module-qtmultimedia

git clone https://github.com/shukiv/whatsappgo.git
cd whatsappgo
make desktop
./desktop/build/whatsappgo
```

`make desktop` builds the Go backend, builds the Qt application, and places the
backend beside the desktop executable. Starting the desktop executable starts
the backend automatically. On desktops with a system tray, closing the window
keeps WhatsAppGo connected in the tray; choose **Quit WhatsAppGo** from its tray
menu to stop the application and every backend process that it owns.

To link an account, open WhatsApp on the phone and choose **Settings → Linked
devices → Link a device**, then scan the QR code. Phone-code pairing accepts an
international number without `+`, spaces, or a leading zero.

For Fedora, Arch, per-user installation, system-wide installation, upgrades,
and uninstalling, see [Installing WhatsAppGo](INSTALL.md).

## Features

- QR and phone-code account linking
- multiple isolated account profiles with top-level account tabs
- text and media messages, voice notes, documents, replies, reactions, edits,
  delete-for-everyone, receipts, and typing indicators
- SQLite-backed history with 50-message pagination and message search
- phone-number/LID identity consolidation so one contact has one history
- inline photo, video, and sticker previews before the full file is fetched
- voice notes with the sender's waveform that play in place and run on to the next
- built-in audio and video playback; no browser or external player
- link previews with title, description, and picture; a one-time YouTube
  metadata backfill repairs historical cards that arrived without thumbnails
- shared contacts and places as cards, with a phone number to message and a map
- attachments fetched automatically for the conversation on screen
- attachments stored in a database, so clearing the media cache loses nothing
- image copy/paste, native media preview, downloads, and cached avatars
- chat filters, pinned and favorite conversations, groups, statuses, channels,
  communities, synchronized call records, and profile/settings screens
- native system notifications and a tray/status icon, light/dark/system
  appearance, RTL text, selectable message text, clickable links, and
  accessible controls
- `whatsappctl` command-line automation with JSON output, live event streams,
  media sending, contact resolution, and a discoverable local API for bots
- Flatpak, Debian, RPM, and AppImage packaging definitions

Call records may be displayed when WhatsApp supplies them, but starting voice
or video calls is not supported.

## Documentation

- [Installation guide](INSTALL.md): dependencies, source build, per-user and
  system-wide installation, updates, and uninstalling
- [User guide](docs/USER_GUIDE.md): pairing, accounts, messaging, history,
  appearance, data, and limitations
- [Troubleshooting](docs/TROUBLESHOOTING.md): build, startup, graphics, history,
  pairing, and media problems
- [Development guide](docs/DEVELOPMENT.md): repository setup, testing, debugging,
  packaging, and contribution workflow
- [Architecture](docs/ARCHITECTURE.md): process lifecycle, storage, event flow,
  identity aliases, memory behavior, and RPC
- [Command-line and bot API](docs/API.md): `whatsappctl`, events, raw methods,
  security, and automation examples
- [Security and privacy](docs/SECURITY.md): local plaintext data, credentials,
  reporting, and account risk

## Requirements

- Go 1.26 or newer; an older Go installation with `GOTOOLCHAIN=auto` can fetch
  the required toolchain
- CMake 3.22 or newer and a C++20 compiler
- Qt 6.5 or newer: Core, Gui, Widgets, Quick, Quick Controls 2, Network, and
  Multimedia
- KDE Frameworks 6 Kirigami

Check native dependencies without changing the system:

```bash
make check-desktop-deps
```

## Build, test, and install

```bash
make desktop
make test
go vet ./...
ctest --test-dir desktop/build --output-on-failure
```

Install under the configured CMake prefix:

```bash
cmake -S desktop -B desktop/build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr
make desktop
sudo cmake --install desktop/build
```

`whatsappgo`, its internal `whatsappd` helper, and the optional `whatsappctl`
automation client are installed together. Launch `whatsappgo` first; the CLI
controls the backend that the desktop owns.

The package definitions under `packaging/` are maintained for release work;
until signed artifacts are attached to a GitHub release, install from source as
described in [INSTALL.md](INSTALL.md).

Qt Quick's software renderer is the default because it is reliable and light
on hybrid-GPU Linux systems. A working GPU stack can opt in with:

```bash
QT_QUICK_BACKEND=rhi ./desktop/build/whatsappgo
```

## Local data

Each account has isolated state:

| Data | Default account | Additional account |
| --- | --- | --- |
| Device keys/session | `$XDG_DATA_HOME/whatsappgo/device.db` | `$XDG_DATA_HOME/whatsappgo/profiles/<name>/device.db` |
| Chats/messages | `$XDG_DATA_HOME/whatsappgo/messages.db` | `$XDG_DATA_HOME/whatsappgo/profiles/<name>/messages.db` |
| Attachments | `$XDG_DATA_HOME/whatsappgo/media.db` | `$XDG_DATA_HOME/whatsappgo/profiles/<name>/media.db` |
| Media cache | `$XDG_CACHE_HOME/whatsappgo/media/` | `$XDG_CACHE_HOME/whatsappgo/profiles/<name>/media/` |
| Runtime socket | `$XDG_RUNTIME_DIR/whatsappgo/whatsappd.sock` | `$XDG_RUNTIME_DIR/whatsappgo/whatsappd-<name>.sock` |

If `XDG_DATA_HOME` or `XDG_CACHE_HOME` is unset, the usual
`~/.local/share` and `~/.cache` locations are used. `messages.db` stores every
message delivered to this linked device. The UI reads the newest 50 messages
and requests older pages while scrolling upward.

The databases contain decrypted message text and metadata. They have per-user
Unix permissions but no additional application-level encryption.

## Project layout

```text
cmd/whatsappd/       internal Go backend executable
cmd/whatsappctl/     JSON command-line and bot client
internal/whatsapp/  whatsmeow adapter and event handling
internal/store/     SQLite message index and migrations
internal/mediastore/ SQLite attachment storage
internal/rpc/       local JSON-lines RPC transport
internal/service/   application operations exposed to the desktop
desktop/            Qt/Kirigami application and UI tests
packaging/          Flatpak, Debian, RPM, AppImage, desktop metadata
docs/               user, developer, architecture, security, troubleshooting
```

## License

GNU General Public License v3.0 or later. See [LICENSE](LICENSE). Product names
and trademarks belong to their respective owners.
