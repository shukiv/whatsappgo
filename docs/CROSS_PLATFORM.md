# Running on Linux, Windows and macOS

Audited and ported on 2026-09-05. This document records what was wrong, what
changed, and what is still unverified.

## Where it stands

| | Linux | Windows | macOS |
| --- | --- | --- | --- |
| Daemon compiles | yes | yes | yes |
| Daemon and client can talk | Unix socket | named pipe | Unix socket |
| Directories resolve | XDG | `%AppData%` / `%LocalAppData%` | `~/Library` |
| Notifications | freedesktop service | desktop client | desktop client |
| Qt client compiles | yes | expected, CI-only | expected, CI-only |
| Packaging | deb, rpm, flatpak, AppImage | ZIP + Inno Setup | .app + DMG |

Built and tested here on Linux. The Windows and macOS builds are exercised by
CI, not on this machine: the Go side cross-compiles and vets cleanly for both,
and the Qt side has never been compiled on either. Treat the first run on those
platforms as untested.

## What was actually blocking it

### One compile error on Windows

`internal/notify` used `syscall.Stat_t` to check that a notification-daemon
binary is owned by root or the current user before executing it. That was the
only thing in the whole module that did not compile for `GOOS=windows`.

The D-Bus backend is Linux-only in every other respect too, so the file is now
`//go:build linux` and `desktop_other.go` provides a `NewDesktop` that declines.
Nothing was weakened: the ownership check still runs where it means something.

### The runtime directory

`config.ResolveProfile` refused to start without `XDG_RUNTIME_DIR`, which
neither Windows nor macOS sets. It is now per-platform, in `paths_linux.go`,
`paths_darwin.go` and `paths_windows.go`:

- Linux keeps the hard requirement. `XDG_RUNTIME_DIR` is the only directory a
  session guarantees to be per-user, mode 0700 and cleared at logout, and the
  socket's access control rests on all three.
- macOS uses `~/Library/Application Support`, beside the databases. `$TMPDIR`
  (`/var/folders/…`) is the closer analogue — macOS creates it per user with
  mode 0700 — but the system's periodic cleaner deletes files there by age, and
  a bound socket it removed would strand the daemon on a path no client can
  reach. The path also has to match what `desktop/src/rpcclient.cpp` computes,
  and naming the directory outright on both sides is what stops them drifting;
  `QStandardPaths::RuntimeLocation` is deliberately not consulted on macOS.
  macOS caps `sun_path` at 104 bytes, and a test asserts the result fits.
- Windows uses `%LocalAppData%`, and the RPC transport does not use it at all.

Data and cache directories follow the platform as well — `~/Library/Application
Support` and `~/Library/Caches` on macOS, `%AppData%` and `%LocalAppData%` on
Windows. `XDG_DATA_HOME` and `XDG_CACHE_HOME` still win everywhere, because that
is how the tests and sandboxed runs redirect a whole profile.

`desktop/src/rpcclient.cpp` mirrors all of this. The two implementations must
stay in step: a mismatch is a client showing an empty account whose data the
daemon is writing somewhere else.

### The transport, on Windows only

The daemon listened with `net.Listen("unix", …)`; the client uses
`QLocalSocket`. On Unix both are AF_UNIX, so macOS needed nothing. On Windows
`QLocalSocket` is a named pipe ([Qt docs](https://doc.qt.io/qt-6/qlocalsocket.html))
and the two do not interoperate.

`internal/rpc/transport_windows.go` now listens with `go-winio`'s `ListenPipe`.
Two details matter:

- **Access control.** A pipe created with a default security descriptor is
  reachable by every account on the machine, and reaching it means driving the
  WhatsApp session. The listener attaches `D:P(A;;GA;;;<user SID>)`: a protected
  DACL whose only entry is the account running the daemon. That is the Windows
  equivalent of the 0600 socket on Unix.
- **Naming.** The pipe namespace is machine-wide, so the name carries the
  current user (`config.PipeUserSegment`). Two people signed in at once must not
  collide. `desktop/src/rpcclient.cpp` repeats the same transformation, and the
  two have to keep matching.

`ListenPipe` asks for the first pipe instance, so a second daemon fails to start
rather than silently splitting clients between two processes — the same
behaviour `removeStaleSocket` gives on Unix.

### Notifications

`internal/notify` is D-Bus plus `/usr/bin/paplay`, and there is no equivalent
off Linux. Rather than write three Go backends, notification *display* moved to
the client for the platforms that need it:

- `notify.Notifier` gained `Presents() bool`.
- The `notification.received` event carries `handled: "1"` when the daemon
  already showed the message itself.
- `RpcClient` raises `notificationRequested` only for unhandled ones, and
  `main.cpp` shows them through `QSystemTrayIcon::showMessage`. Clicking one
  opens the chat.

Linux is unchanged: the freedesktop service still draws every notification, and
the client stays quiet. The visible difference elsewhere is that notifications
arrive only while WhatsAppGo is running — neither Windows nor macOS lets a
background helper post a notification on behalf of an app that is not there.

If D-Bus is missing on a Linux machine, the client now presents notifications
instead of the user getting none. That is an improvement, not a regression.

### The Qt client

All seven Qt modules ship on all three platforms. The dead Kirigami dependency
was removed first, so nothing KDE-specific remains in the build.

Two changes:

- `daemonExecutable()` looks for `whatsappd.exe` on Windows.
- The Qt Quick Controls style is `org.kde.desktop` on Linux and **Fusion**
  everywhere else. Almost every control here is drawn by the application's own
  QML against `Theme.qml`; a native style would restyle only the few primitives
  underneath and make those the one part of the window that does not match
  WhatsApp. Fusion leaves them neutral, so all three platforms render the same
  design. `QT_QUICK_CONTROLS_STYLE` still overrides it.

`CMakeLists.txt` now builds a `.app` bundle on macOS with the daemon beside the
client in `Contents/MacOS`, a `WIN32_EXECUTABLE` on Windows so no console window
appears behind it, and installs freedesktop metadata only on Linux.

## The daemon leak, found on the way

Thirteen `whatsappd --profile default` processes were alive on the development
machine at once. The cause was in `RpcClient::startBackendForProfile`:

```cpp
// A crashed process can leave its filesystem socket behind. This method is
// called only after connecting failed, so no live listener owns the path.
QLocalServer::removeServer(socketPathForProfile(profile));
```

The comment's premise is wrong. A refused connection does not prove the daemon
is gone — a full listen backlog refuses too. Unlinking a bound Unix socket does
not disturb the process holding it: it keeps accepting on an inode with no name,
holds its SQLite handles open, and is unreachable. Every retry left one more
behind, and several daemons with `SetMaxOpenConns(1)` each on one database is a
contention bug waiting to happen.

Fixed on both sides, because either one alone leaves a hole:

- The client probes first (`RpcClient::backendIsListening`) and only clears a
  path that answers nobody.
- The daemon watches the socket it bound (`watchListener` in
  `internal/rpc/transport_unix.go`) and stops when it is unlinked or replaced,
  so an orphan created any other way heals itself within five seconds.

## Tests

Added. Every one below was verified by removing its fix and watching it fail,
except the macOS-only path test, which cannot run on this machine:

- `internal/rpc`: a daemon whose socket is unlinked stops; one that still owns
  its socket keeps serving; a second daemon cannot take a served socket.
- `internal/config`: XDG variables win on every platform; defaults follow the
  platform; Linux still refuses to run without a runtime directory; on macOS the
  socket lands in Application Support and fits inside `sun_path`.
- `internal/notify`: `Presents()` is false for the no-op notifier and for the
  off-Linux stub, true for the freedesktop backend.
- `desktop-notification-routing`: a handled notification stays out of the
  client, an unhandled one reaches it with its chat, title and body.
- `desktop-backend-probe`: a served socket reads as live, a missing or
  unserved path does not.

`make cross` compiles and vets the module for Linux, Windows and both macOS
architectures. CI runs the same matrix, plus full Qt builds and ctest on
`windows-latest` and `macos-latest`. `go test -race ./...` is clean; the poll
interval the socket watcher uses is a field on the server rather than a package
variable precisely so the tests cannot race it.

## Still open

- The Qt build on Windows and macOS has only ever run in CI, never on a real
  desktop of either. Expect first-run problems that no cross-compile can show.
- Signing is wired but unexercised: no Developer ID certificate and no Windows
  code-signing certificate were available here, so both scripts fall back to
  producing unsigned output with a warning.
- Notification click-to-open is implemented through the tray icon, which needs
  the client running. There is no equivalent of the Linux path that wakes the
  application from a notification when it is closed.
