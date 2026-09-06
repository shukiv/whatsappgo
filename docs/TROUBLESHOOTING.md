# Troubleshooting

## `make: cmake: No such file or directory`

CMake and the native development packages are missing. On Debian 13, install
the command shown in the root README, then run:

```bash
make check-desktop-deps
make desktop
```

## `module "org.kde.desktop" is not installed`

Install the distribution's KDE desktop QML style module. On Debian 13 it is
`qml6-module-org-kde-desktop`. It is a Qt Quick Controls style and does not
require the Plasma desktop session. The style is chosen only on Linux, and only
when nothing has set `QT_QUICK_CONTROLS_STYLE`; Windows and macOS use Fusion.

For a temporary diagnostic run, use:

```bash
QT_QUICK_CONTROLS_STYLE=Fusion ./desktop/build/whatsappgo
```

The normal package should include the desktop style dependency.

## Backend is not connected

Do not run `whatsappd` manually. Rebuild both components and start the desktop:

```bash
make desktop
./desktop/build/whatsappgo
```

The build copies `whatsappd` beside the UI. The application removes a stale
profile socket after a refused connection and starts its owned backend.

To see backend diagnostics:

```bash
WHATSAPPGO_BACKEND_LOGS=1 ./desktop/build/whatsappgo
```

If using a nonstandard build, set `WHATSAPPGO_BACKEND` to its absolute helper
path.

## A `whatsappd` process with no window

There should never be one. The desktop stops the helpers it started when it
quits, and each helper also ends on its own once the client that started it is
gone, however it went - see "Overview" in [ARCHITECTURE.md](ARCHITECTURE.md).
A helper still running with no client is worth reporting through the bug button
in the sidebar; `ps -o pid,ppid,etimes,args -C whatsappd` names its age and
parent, which is what the report needs.

## `failed to create drawable`, VDPAU, NVIDIA, or FFmpeg warnings

The application defaults to Qt Quick's software renderer, which avoids a
persistent OpenGL context. Make sure no shell startup file forces
`QT_QUICK_BACKEND=rhi` or an incompatible `QSG_RHI_BACKEND`.

The missing `libvdpau_nvidia.so` message is an optional FFmpeg hardware-decoder
probe on systems without the NVIDIA VDPAU driver. It should not prevent normal
software media decoding. Rebuild and run the current binary if old warnings
persist.

## Diagonal green or white lines across a chat

Older builds used rotated negative-z rectangles for message tails. Some Qt
software/hybrid-GPU paths stretched those nodes into diagonal bands. Rebuild
the current desktop; tails are now untransformed Canvas triangles:

```bash
make desktop
```

## Chat history appears incomplete

First scroll upward: only the newest 50 messages are loaded initially. History
is stored in `messages.db`, not held entirely in the UI.

Current builds also consolidate WhatsApp's phone-number and LID identities.
Restarting the rebuilt application runs this migration automatically and
preserves message counts. Do not delete either database to solve a display
problem.

The application cannot retrieve messages that WhatsApp did not provide to the
linked device. Keep the phone and desktop online during initial sync and try
scrolling to the oldest local boundary again.

## A conversation or the chat list jumps while scrolling

Current builds let the Qt list handle ordinary mouse-wheel input and preserve a
stable row/message anchor while models update. Rebuild before diagnosing an old
binary:

```bash
make desktop
./desktop/build/whatsappgo
```

If a current build still jumps, note whether it is the conversation or chat
list, whether older history was loading, and whether an avatar, preview, unread
badge, or new message changed at the same moment. Include the Qt version,
display scale, mouse versus touchpad, and a short screen recording. Do not send
message text, phone numbers, or profile databases.

## Duplicate conversations for one person

Restart a current build so directory synchronization can apply the verified
phone↔LID mapping. Conversations are never merged by display name alone, so two
different people with the same name remain separate.

## QR code is blank or expired

The initial QR should generate automatically. If it is blank, wait for the
backend connection and press **Generate QR code**. Use **Refresh QR code** after
expiration. Ensure the computer has network access and the profile is not
already linked.

## `failed to get ... app state key` or snapshot verification errors

WhatsApp has not yet shared a key required to verify that app-state snapshot.
The application defers that optional backfill rather than deleting state. Keep
the official phone and desktop online and reconnect later. Do not remove
`device.db`; doing so unlinks the local profile.

## `mismatching LTHash` while syncing call history or chat settings

The collection WhatsApp sent does not add up to the hash it signed. Asking again
produces the same bytes, so the daemon asks the phone for a plain copy of that
collection instead and says nothing on screen; it asks at most once a day. Keep
the phone online and reconnect later. Nothing is deleted, and the rest of the
conversation is unaffected.

## The update button says WhatsAppGo is up to date

Check, in this order:

- This copy was built from source. Its tooltip says so instead of naming a
  version, and a working copy is never behind a release; `git pull` updates it.
- The newest release is still a draft. GitHub's "latest release" ignores drafts,
  so there is nothing to compare against until it is published.
- The release has no `SHA256SUMS`. An artifact whose checksum the release does
  not publish is never installed, so the release offers nothing.

`whatsappctl --profile <name> call update.status '{}'` answers all three: it
reports the running version, the newest release it found, and the error from the
last check, if any.

## The check says GitHub is limiting checks from this address

GitHub answers sixty anonymous requests an hour per address, and every program
on that address shares the allowance - a shared office line, a VPN exit, or
carrier-grade NAT can spend it without WhatsAppGo asking for anything. Each
start of the daemon looks once, so restarting it many times in an hour - a day
of rebuilding, or testing releases - spends the allowance as well. The
answer names how long is left; nothing is asked of GitHub again until then, so
pressing the button meanwhile repeats the same wait rather than making it
longer. Releases are still installable by hand from the releases page while the
allowance is spent.

## The AppImage does not start

It mounts itself with FUSE. Where FUSE is missing or an unprivileged mount is
refused, run the contents from a temporary directory instead:

```bash
./WhatsAppGo-x86_64.AppImage --appimage-extract-and-run
```

Debian and Ubuntu install FUSE with `sudo apt install libfuse2t64` (older
releases call it `libfuse2`); Fedora uses `sudo dnf install fuse-libs`. An
update also needs the file to be writable by the account running it: a copy in
`/opt` owned by root can only be replaced by hand.

## Calls, statuses, channels, or communities are empty

These pages display data that WhatsApp exposes to linked devices. App-state key
availability and server-side sync determine what appears. Voice/video call
placement is not implemented even when call records are visible.

## Photos and videos show no picture

Current builds display the preview that WhatsApp embeds in each media message.
Conversations synced by an older build are filled in once, in the background,
the next time the account connects; reopen the conversation afterwards. A
message whose preview WhatsApp never sent still shows the descriptive row with
a **Download** action.

## An image is cropped or its open-image menu is missing

Current builds open pictures inside WhatsAppGo at aspect-fit 100%. The whole
source should be visible before zooming, including portrait and ultrawide
screenshots. Clicking the open image with either mouse button shows **Copy
image** and **Save image as…**. Rebuild if the image opens through `xdg-open` or
is still cropped. If only a thumbnail looks soft, press **Download** and allow
the message to refresh to the full-resolution cached source.

## Clicking a notification does not open the conversation

When the system tray is available, the desktop owns notifications and clicking
one reopens its conversation directly. Without a tray, the backend uses the
desktop notification service or portal. A click first activates the running
WhatsAppGo instance. If no instance is listening, it can launch only a
`whatsappgo` installed next to the backend that no other account can replace.
If neither activation route is safe, the backend logs the reason and does not
attach an action to the notification.

## Incoming-message notifications do not appear

WhatsAppGo first uses the freedesktop notification service, asks D-Bus to
activate it when necessary, and then falls back to the desktop portal. Some
minimal X11 sessions install `notification-daemon` without starting it;
WhatsAppGo starts a trusted system copy automatically. If banners still do not
appear, confirm that notifications are enabled in the desktop settings and run
`notify-send "WhatsAppGo test"`. A `ServiceUnknown` error means the desktop did
not install any notification provider; install `notification-daemon` or the
notification component supplied by your desktop environment.

## The WhatsAppGo icon is missing from the GNOME top bar

GNOME Shell requires a StatusNotifier/AppIndicator extension to display tray
icons. On Debian/Ubuntu, install `gnome-shell-extension-appindicator`, enable
**AppIndicator and KStatusNotifierItem Support** in GNOME Extensions, and sign
out and back in. WhatsAppGo notices a tray host that appears after it starts;
the icon then offers connection status, **Open/Hide**, and **Quit WhatsAppGo**.
If the desktop exposes no tray host, WhatsAppGo keeps normal minimize behavior,
quits when its last window closes, and still sends notifications through the
desktop service or portal.

## A voice note or video opens a web browser

Older builds handed media to `xdg-open`. On a system with no registered handler
for Opus audio that falls through to the default browser. Current builds play
audio and video inside the window and never call out to the desktop for them.
Rebuild with `make desktop`.

## A media item says Download but does not open

Press **Download** and wait for the message to update to a cached path. Old
media may no longer be downloadable from WhatsApp. Confirm the cache directory
is writable and has free space. Clipboard image sending requires an image MIME
type, not only copied filename text.

## Safe diagnostic information

When reporting a problem, include the application version, Linux distribution,
Qt versions, the exact error, and whether software or RHI rendering is
used. Do not include QR contents, pairing codes, `device.db`, `messages.db`,
message text, phone numbers, or complete JIDs.
