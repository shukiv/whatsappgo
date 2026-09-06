# Development guide

## Toolchain

WhatsAppGo requires Go 1.26+, CMake 3.22+, a C++20 compiler, Qt 6.5+ (Core,
Gui, Quick, Quick Controls 2, Network, Multimedia).

On Debian 13:

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-org-kde-desktop \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

Run `make check-desktop-deps` for a read-only prerequisite check.

## Build and run

```bash
make desktop
./desktop/build/whatsappgo
```

`make desktop` builds `bin/whatsappd` and `bin/whatsappctl`, builds the Qt
application, and copies both Go executables beside `desktop/build/whatsappgo`.

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

The desktop suite covers QML startup, clean
stderr, themes, search and quoted-message navigation, selectable/linkified
messages, clipboard images, message and chat-list wheel scrolling during model
updates, presence, status-notification suppression, media preview/playback,
played receipts, filters, compact geometry, menu edge clamping, automatic
pairing, backend ownership, and bubble-tail rendering. Tests use offscreen Qt
and isolated XDG paths when they persist data.

`desktop-media-preview` delivers mouse-wheel events through the offscreen Qt
window, rather than calling the viewer's zoom helper directly. It checks
pointer-anchored zoom in both directions without modifiers, pixel-only deltas,
horizontal/empty events, and the 100%–500% limits. QML `WheelEvent` coordinates
are `x` and `y`, unlike the `position` point used by `TapHandler` events.

`desktop-composer-drafts` also checks same-chat selection preservation, voice
recording cancellation on navigation, and media-library message targets.
`desktop-conversation-updates` uses a local stub daemon to verify document and
voice-send parameters and out-of-order starred-message responses. Neither test
records microphone input or sends messages to WhatsApp. Go tests cover document
upload/wire types, scoped stars (including aliases and limits), and refusal to
forward undownloaded attachments as caption-only text.

`desktop-privacy-settings` verifies initial loading, account isolation,
reconnects, and privacy events racing with older reads using isolated stub
daemons. The settings, status-story, and composer tests also cover separate
online visibility controls, paused story progress, and media selection keyed
by both chat and message. Go regressions cover edited link previews, replayed
history, and preview requests that finish after an edit. None of these tests
uses a live WhatsApp account.

Alias-merge regressions preserve deleted/edited copies and rich metadata. The
media recovery test deletes the cache boundary by starting with archive-only
bytes under a pre-merge identity. Mark-all-read tests span 1,202 active/archived
chats and inject an individual failure. Composer tests exercise failed sends,
late acknowledgements, new typing, and account/chat isolation without sending
real messages. Intake tests use `httptest` and must never file test tickets in
the production tracker. See [bug reporting](BUG_REPORTING.md) for runtime key
configuration and the corrected findings.

When fixing a bug, add a focused regression that fails before the production
change. Storage tests use in-memory SQLite. WhatsApp adapter tests exercise
event-to-model transformations without contacting WhatsApp.

## Repository map

| Path | Responsibility |
| --- | --- |
| `cmd/whatsappd` | internal backend entry point and socket listener |
| `cmd/whatsappctl` | JSON CLI, raw API client, and event stream |
| `internal/config` | validated XDG/profile paths and permissions |
| `internal/whatsapp` | whatsmeow client, pairing, events, media, history |
| `internal/store` | WhatsAppGo SQLite schema, migrations, queries |
| `internal/mediastore` | chunked attachment database |
| `internal/service` | validation and application operations |
| `internal/rpc` | versioned JSON-lines server |
| `desktop/src` | lifecycle-aware RPC client, conversation model, entry point |
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
the gateway/store, add it to `rpc.discover`, then add the desktop or CLI
interaction. Persist incoming/outgoing state before publishing an event.

Keep socket methods bounded: chat limits max at 500, message pages max at 200,
and the normal UI page size is 50.

## UI changes

Use `Theme.qml` semantic tokens rather than literal colors. Keep large lists
virtualized, cap media dimensions, preserve RTL/Unicode text, and add accessible
names for icon-only controls. Avoid transformed negative-z primitives in list
delegates; some software/hybrid-GPU scene graphs render them as unbounded
stripes.

The native desktop geometry is specified in
[`design-system/whatsappgo/pages/desktop.md`](../design-system/whatsappgo/pages/desktop.md).
Its measured baselines include a 64 px rail and headers, 40 px rail actions,
32 px filter chips, 36 px menu rows, and component-specific popup widths. Do not
reintroduce a single oversized default for all menus. Responsive filter ordering
and overflow behavior are part of the specification, not screenshot-only polish.

`Popup` content may not reach its final height until the first polish after
`open()`. Shared menus therefore clamp both before and after opening and whenever
their final implicit height changes. Tests for a bottom-edge message menu must
assert both the action menu and paired reaction tray remain within the window.

Let `ListView` handle ordinary mouse-wheel physics. Code that restores an anchor
after pagination or a real reorder must key it by stable message/chat identity
and pixel offset. Avatar, preview, unread, receipt, and presence refreshes must
not reset `contentY`, `currentIndex`, or force the list back to an endpoint.

Declare QtMultimedia objects inside a `Loader` that is inactive until they are
needed. A `MediaPlayer` or `VideoOutput` created at startup initialises the
FFmpeg backend and prints hardware-decoder probing warnings, which
`ctest -R desktop-clean-startup` rejects.

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

`--screenshot-chat <jid>` opens a conversation first, which is how a rendering
is compared with WhatsApp Web:

```bash
QT_QPA_PLATFORM=offscreen ./desktop/build/whatsappgo --profile <name> \
  --screenshot-chat '1234567890@lid' --screenshot /tmp/conversation.png
```

The run attaches to whichever daemon already serves that profile, so it does not
disturb a window that is open. Measure the result rather than eyeballing it: at
a 1.25 device scale a 24-pixel item is 30 pixels on screen, so compare in device
pixels and check a shared element - an avatar, a tick - to confirm both images
are at the same scale.

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

All packages install `whatsappgo`, its internal `whatsappd` helper, and
`whatsappctl` together in the same binary directory. Packages do not require
systemd user units.

- Flatpak: `packaging/flatpak/org.whatsappgo.Desktop.yml`
- Debian: `packaging/debian/`
- RPM: `packaging/rpm/whatsappgo.spec`
- AppImage: `packaging/appimage/build.sh`

Every package builds the backend and CLI into `bin/` before configuring the
desktop project, because `desktop/CMakeLists.txt` installs both next to
`whatsappgo`.

whatsmeow requires Go 1.26. Debian 13 (trixie) ships `golang-any` 2:1.24, so
`dpkg-buildpackage` needs a Go toolchain from trixie-backports or unstable, or
a `.deb` built on a host that already has Go 1.26. Debian builds must not
download a toolchain, so `GOTOOLCHAIN=auto` is not a packaging solution.

After changing Go dependencies, regenerate the Flatpak module source list as
described in the root README.

## Cutting a release

1. Choose the next `vMAJOR.MINOR.PATCH` tag and update package metadata in
   `desktop/CMakeLists.txt`, the Windows installer, Flatpak build commands,
   Debian changelog, RPM spec, and AppStream release history.
2. Record release notes in `docs/releases/<tag>.md`. Keep credential requirements,
   unsigned-build warnings and known limitations explicit.
3. Commit the reviewed changes, push the branch, and wait for that exact commit
   to pass the complete CI workflow, including desktop tests on all platforms.
4. Create an annotated tag at the tested commit and push it. The Release workflow
   builds Linux, Windows and Apple Silicon artifacts and generates `SHA256SUMS`.
5. Apply the prepared notes to the draft release and verify all platform assets
   and checksums. Leave the release as a draft until a human reviews and publishes
   it; creating a tag does not itself authorize automatic publication.

Release builds receive `WHATSAPPGO_VERSION` from the tag. Do not cut a tag from
a dirty worktree or silently omit uncommitted fixes from its source snapshot.
