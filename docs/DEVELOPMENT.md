# Development guide

## Toolchain

WhatsAppGo requires Go 1.26+, CMake 3.22+, a C++20 compiler, Qt 6.5+ (Core,
Gui, Quick, Quick Controls 2, Network, Multimedia, SVG).

On Debian 13:

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev libqt6svg6-dev \
  qml6-module-org-kde-desktop qt6-gtk-platformtheme \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

Run `make check-desktop-deps` for a read-only prerequisite check.

File open/save dialogs use Qt's native platform integration, independently of
the Qt Quick Controls style. GNOME needs `qt6-gtk-platformtheme` (recommended by
the Debian package). In graphical Linux sessions where a service/terminal
launcher omits desktop identification, startup selects `gtk3` before creating
`QApplication`. Existing desktop identification and `QT_QPA_PLATFORMTHEME`
overrides are preserved; offscreen/minimal runs do not select GTK. Qt retains
its built-in picker as a fallback when native integration is unavailable.

### Document cards and jump-to-latest

`DocumentCard.qml` is loaded only for document delegates. It bounds filenames
to two wrapped/elided lines, exposes the full name in a tooltip and keeps the
file type/size separate from the message timestamp. Activation calls
`RpcClient::downloadDocument`: recover an uncached attachment through the
existing RPC, then copy it to `QStandardPaths::DownloadLocation` on a worker
thread. A per-profile/chat/message busy set prevents concurrent duplicate
clicks. Save with exclusive creation, sanitized portable basenames, owner-only
permissions and numbered collisions; never overwrite or auto-launch a file.
Completion and errors return to the UI via `QFutureWatcher`.

The conversation's jump-to-latest button overlays the message viewport, not
the composer. Use the existing `nearTail()`/`returnToTail()` logic: this model
is newest-first with a bottom-to-top `ListView`, so its newest edge is
`positionViewAtBeginning()`. Explicit jumps cancel pending quote/search
navigation and restore following; wheel and scrollbar input release following
until the reader returns to the tail. Do not animate or rebuild message rows
just to show the button.

### Settings and back navigation

For native animated-sticker playback, make libwebp and libwebpdemux available
to pkg-config before configuring CMake (`libwebp-dev` on Debian/Ubuntu). CMake
enables this optional decoder when both are present; no Qt WebP image plugin is
needed. Without them the app builds with its existing static PNG fallback.
Do not create a `MediaPlayer` or `VideoSurface` for every chat delegate: the
inline animation loader must remain inactive until the user selects that
visible message, and must unload offscreen.

Settings uses the existing sidebar tree, with nested message/group notification
pages returning to Notifications and blocked contacts returning to Privacy.
The window keeps a bounded section history with Chats as its fallback. Escape
leaves open Qt popups and media viewers in charge before navigating a panel or
section; don't register two active Cancel shortcuts for the same overlay.
Check whether the active focus item is inside the overlay, not the overlay's
`visible` property: Qt can leave an empty overlay visible. Passive tooltips
must not disable Escape. Verify with actual key presses after focusing the
window, including a popup over a nested settings page.
Wallpaper and emoticon replacement share the pane's existing local composer
preferences. Unsupported settings show explicit availability guidance.

Notification switches use `notifications.get` / `notifications.set` and the
`notifications.updated` event. Each boolean lives in a separate metadata key
in the profile's store, avoiding read/modify/write races between clients. The
WhatsApp event handler applies category and preview preferences before either
native notification delivery or the desktop fallback event. A failed
preference read suppresses that alert rather than exposing a hidden preview.
Linux sound suppression applies to freedesktop hints and fallback sound
playback; Windows/macOS sound controls remain in the OS.

Each Messages/Groups subpage has alert, reaction-alert and sound switches;
Calls has alert and sound switches. Reactions are notified only after a changed
store upsert, for recent incoming reactions to an existing outgoing message.
History, removals and duplicates do not alert. Call offers/notices share a
bounded five-minute dedup set and always direct the user to answer on the phone.
Outgoing sounds run only after successful service sends are stored; the Linux
player is bounded to one process with a five-second timeout. Sound preview RPCs
do not create notifications or send messages.

Local network switches use `preferences.get/set` and `preferences.updated`.
Both live media caching (after acquiring its slot) and the background collector
check per-type download settings. Explicit downloads remain available. The
link-preview switch is enforced in the service composer/send paths and both
historical/refreshed preview resolvers; QML is not the privacy boundary.
`profile.set_name` uses the optional `gateway.ProfileEditor` and WhatsApp's
push-name app-state patch, not the local profile alias. Native settings remain
account-scoped and stale replies are rejected after profile changes.

Security-code changes reuse the bounded alert deduplicator with a separate key
namespace. `profile.get`, `status.audience` and default-timer editing use optional
gateway capabilities. Keep About visibility separate from broadcast audiences;
the legacy privacy `status` name means About. The default timer has no getter
in the pinned library, so the UI asks for a new value with confirmation instead
of presenting a guessed current value. About readbacks must not discard edits
while a save fails or another request completes.

Photo quality is a live composer-setting alias passed into attachment sends,
not a second backend preference. `PhotoQuality::prepare` runs in a dedicated
single-worker pool. Temporary files stay alive through the RPC callback and
account-generation checks cancel a queued upload after a profile switch.
Documents/animations/videos and Original photos bypass transformation.

`ComposerText` shares one syntax highlighter between color emoji and spelling.
Aspell discovery/checks/suggestions are asynchronous, debounced and bounded;
never log their input/output or send draft text to a service. Dispose active
process callbacks before destroying the helper's state. The composer edit menu
must tolerate an empty clipboard: Qt may return null MIME data, especially
under the offscreen platform.

The 2026-09-08 follow-on pass was verified with the existing 44 desktop checks,
Go suite/vet/race checks, cross-platform Go builds, and disposable native/photo/
spelling/RPC probes. No new tests or test infrastructure were added. Native
screenshots covered both themes, profile/audience readback and timer navigation;
Escape dismissed the unaccepted confirmation before returning to Privacy.
Real account-setting writes and security-code-change delivery were not triggered.

### Icons

Standard navigation, menu, message-action and playback icons use the bundled
Lucide SVG subset. Keep using `TintedIcon` / `ThemedToolButton` and semantic
`Theme` colors; their existing glyph sizes and hit targets remain unchanged.
`LucideProvider` renders trusted Qt resources with Qt SVG, tinting with QPainter
so the software renderer works too. Images are cached by name, tint and device
pixel size. No runtime CDN, Node package or GPU effect is needed.

The app logo, status rings and WhatsApp-style receipt marks retain their own
artwork. See [icon provenance, aliases and license](../desktop/qml/icons/README.md)
when adding or updating icons; include new SVGs in `desktop/CMakeLists.txt`.

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

### Large-conversation performance

Keep scroll-anchor stability while optimizing work per bubble: the message
view's three-screen cache buffer is intentional. Prefer lazy controls and
bounded image decoding over shrinking this buffer blindly. Menu popups now
exist only after an interaction; UI inspection must locate them after opening
the menu, rather than holding pointers captured before the click.

Message updates should not scale with the number of loaded rows unless the
operation genuinely affects every row. A date change touches its boundary and
the next dated neighbour; an ordinary edit must not recalculate all dates.
Sidebar event bursts must share refreshes, and cached chat reopening should
not reset a same-identity message page unnecessarily. Cache eviction must
never delete durable history or lower the quality of the full-screen viewer.

For before/after measurements, use the same optimized build, renderer, display
scale and synthetic dataset. Separate component construction, model update and
SQL timings from end-to-end chat-opening/frame timings. A raw-QML/offscreen
probe is useful for relative costs but is not a production frame-rate claim.

### Running the existing checks

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
`desktop-message-scroll` uses an isolated stub daemon and controlled model
pages. It delivers actual Return-key and mouse-wheel events to the offscreen
window, checks the newest delegate's bottom edge (not estimated content height),
and covers sending while browsing, delayed sent rows, same-chat reselection,
cached opens, late row layout, same-count page refreshes, metadata refreshes,
and quoted-message targets. It never sends a real WhatsApp message. Flush
pending model updates before positioning with
[`ListView.forceLayout()`](https://doc.qt.io/qt-6/qml-qtquick-listview.html#forceLayout-method);
do not add unconditional content-height handlers that fight reader scrolling.
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

When fixing a bug, establish a focused regression that fails before the
production change. Follow the active repository/user policy on adding tests:
the recent parity and reliability batches used disposable fixtures outside the
repository rather than adding test infrastructure. Storage tests use in-memory
SQLite. WhatsApp adapter tests exercise event-to-model transformations without
contacting WhatsApp.

### Isolated desktop race checks

Exercise asynchronous failures through the actual QML UI with a synthetic local
RPC server, not a live account. Give each run separate `XDG_RUNTIME_DIR`,
`XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_CACHE_HOME` directories; keep runtime
permissions at 0700. Use `WHATSAPPGO_DISABLE_PROFILE_MONITORS=1` for these probes.
Only fake-RPC probes use `WHATSAPPGO_BACKEND=/bin/false` to prevent launching a
real account backend. **Do not apply that override to the full CTest suite**:
lifecycle tests need to launch their test daemon. Run CTest serially (`-j1`)
when desktop tests compete for resources.

Allow at least 300 ms for popup transitions before asserting settled visibility.
Test both visible dismissal and the absence of an unintended RPC mutation.
Reorder server replies deliberately; cover same-chat reloads, profile changes,
newer clipboard intent, pending edits, refresh-preserved status replies and
hidden-window receipts followed by foreground activation. Mention tests must
assert outgoing recipient metadata, not just the visible name, and exercise Undo.

The 2026-09-11 reliability batch passed 33 focused checks per theme, 44 CTests,
Go tests and vet. These are recorded results, not additional permanent CTest
targets. See the [audit evidence](WHATSAPP_WEB_PWA_GAP_AUDIT.md#desktop-reliability-regressions--2026-09-11-unreleased)
and [desktop state rules](ARCHITECTURE.md#desktop-request-ordering-and-ownership).

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

`desktop-layout-regressions` also checks the account unread badge at the outer
top-right of both 40px and 44px buttons with one-, two-, and three-digit counts.
`desktop-bug-report` checks the toolbar and Help reporting actions using a
stubbed browser opener. Both open the WhatsAppGo GitHub issues URL with
no dialog; browser-launch failure shows the destination for manual navigation.
Use isolated XDG config/data/cache/runtime directories and unset intake
credentials for these tests; never submit test reports to the live intake.
The report-action check reuses the in-process daemon stub so backend-startup
errors cannot overwrite the browser-failure notice under test. The profile-name
check keeps a passive local listener instead of launching helpers. Both use
distinct test profile names to isolate Windows named pipes as well as Unix
socket paths; neither depends on backend startup/shutdown timing.

`desktop-presence-updates` replays presence events through an isolated RPC socket.
It covers missing stop events, explicit paused/offline events, audio recording,
deadline renewal, unrelated updates, chat changes, and connection loss. The test
checks the production 10-second timeout, then shortens the actual Qt timer for
fast regression coverage; no live WhatsApp account is used.

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

Keep post-tag repairs in a `docs/releases/UNRELEASED.md` file until a reviewed
commit and new artifacts contain them, then rename it to the tag being cut. Do
not retroactively describe an existing draft binary as including worktree-only
fixes. The 2026-09-11 reliability repairs are tagged as
[v0.1.9](releases/v0.1.9.md) and are not in the v0.1.8 draft artifacts;
building and publishing artifacts for the new tag remain separate, explicitly
authorized steps.
