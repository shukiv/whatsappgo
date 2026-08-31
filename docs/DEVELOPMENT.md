# Development guide

## Toolchain

WhatsAppGo requires Go 1.26+, CMake 3.22+, a C++20 compiler, Qt 6.5+ (Core,
Gui, Quick, Quick Controls 2, Network, Multimedia), and KF6 Kirigami.

On Debian 13:

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  libkirigami-dev extra-cmake-modules \
  qml6-module-org-kde-kirigami qml6-module-org-kde-desktop \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

Run `make check-desktop-deps` for a read-only prerequisite check.

## Build and run

```bash
make desktop
./desktop/build/whatsappgo
```

`make desktop` builds `bin/whatsappd`, builds the Qt application, and copies
the backend beside `desktop/build/whatsappgo`.

The desktop resolves its helper in this order:

1. `WHATSAPPGO_BACKEND`, when set to an executable path;
2. `whatsappd` beside the desktop executable;
3. the development `bin/whatsappd` relative to the build directory;
4. `whatsappd` on `PATH`.

It starts the helper with `--profile <name>` only after the profile socket is
unavailable. The `QProcess` is parented to `RpcClient` and terminated during
desktop shutdown.

## Tests

```bash
go test ./...
go vet ./...
ctest --test-dir desktop/build --output-on-failure
```

The desktop suite covers QML startup, clean stderr, themes, search navigation,
selectable/linkified messages, clipboard images, unread/multiline layout,
media preview, filters, automatic pairing, backend ownership, and bubble-tail
rendering. Tests use offscreen Qt and isolated XDG paths when they persist data.

When fixing a bug, add a focused regression that fails before the production
change. Storage tests use in-memory SQLite. WhatsApp adapter tests exercise
event-to-model transformations without contacting WhatsApp.

## Repository map

| Path | Responsibility |
| --- | --- |
| `cmd/whatsappd` | internal backend entry point and socket listener |
| `internal/config` | validated XDG/profile paths and permissions |
| `internal/whatsapp` | whatsmeow client, pairing, events, media, history |
| `internal/store` | WhatsAppGo SQLite schema, migrations, queries |
| `internal/service` | validation and application operations |
| `internal/rpc` | versioned JSON-lines server |
| `desktop/src` | lifecycle-aware RPC client and application entry point |
| `desktop/qml` | theme, pages, delegates, menus, and media UI |
| `desktop/tests` | Qt integration tests |
| `packaging` | Flatpak, Debian, RPM, AppImage, desktop metadata |

## Storage changes

Migrations in `internal/store` must be additive and preserve existing profiles.
Never recreate `messages.db` merely to change the schema. Use transactions for
cross-table changes and test message counts, foreign keys, deduplication, and
future writes.

whatsmeow owns `device.db`; do not add application tables to it. Read protocol
identity mappings through whatsmeow's store interfaces instead of querying its
private schema from product code.

## RPC changes

Packets are newline-delimited JSON and carry `version: 1`. Add parameter
structs and strict validation in `internal/service`, implement the operation on
the gateway/store, then add the `RpcClient` call and QML interaction. Persist
incoming/outgoing state before publishing an event.

Keep socket methods bounded: chat limits max at 500, message pages max at 200,
and the normal UI page size is 50.

## UI changes

Use `Theme.qml` semantic tokens rather than literal colors. Keep large lists
virtualized, cap media dimensions, preserve RTL/Unicode text, and add accessible
names for icon-only controls. Avoid transformed negative-z primitives in list
delegates; some software/hybrid-GPU scene graphs render them as unbounded
stripes.

A message bubble sizes itself from its content, so nothing inside it may size
itself from the bubble. Width limits inside `MessageDelegate.qml` derive from
`contentMaxWidth`, which depends only on the conversation width. Binding a
child's `Layout.maximumWidth` to `bubble.width` creates a loop that Qt breaks
silently, leaving bubbles collapsed or stretched across the pane;
`ctest -R desktop-message-layout` covers the resulting geometry.

For a screenshot without a live desktop:

```bash
QT_QPA_PLATFORM=offscreen ./desktop/build/whatsappgo \
  --theme light --screenshot /tmp/whatsappgo.png
```

## Debugging

The managed backend is quiet by default. Forward its stdout/stderr through the
desktop terminal with:

```bash
WHATSAPPGO_BACKEND_LOGS=1 ./desktop/build/whatsappgo
```

To test a different helper build:

```bash
WHATSAPPGO_BACKEND=/absolute/path/to/whatsappd \
WHATSAPPGO_BACKEND_LOGS=1 ./desktop/build/whatsappgo
```

Use isolated XDG directories for destructive/manual tests; never point tests at
a real profile database.

## Packaging

All packages install `whatsappgo` and its internal `whatsappd` helper together
in the same binary directory. Packages do not require systemd user units.

- Flatpak: `packaging/flatpak/org.whatsappgo.Desktop.yml`
- Debian: `packaging/debian/`
- RPM: `packaging/rpm/whatsappgo.spec`
- AppImage: `packaging/appimage/build.sh`

Every package builds the backend to `bin/whatsappd` before configuring the
desktop project, because `desktop/CMakeLists.txt` installs `../bin/whatsappd`
next to `whatsappgo`.

whatsmeow requires Go 1.26. Debian 13 (trixie) ships `golang-any` 2:1.24, so
`dpkg-buildpackage` needs a Go toolchain from trixie-backports or unstable, or
a `.deb` built on a host that already has Go 1.26. Debian builds must not
download a toolchain, so `GOTOOLCHAIN=auto` is not a packaging solution.

After changing Go dependencies, regenerate the Flatpak module source list as
described in the root README.
