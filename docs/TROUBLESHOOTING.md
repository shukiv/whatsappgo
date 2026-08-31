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
`qml6-module-org-kde-desktop`. Kirigami is a Qt component library and does not
require the Plasma desktop session.

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

## Clicking a notification does not open the conversation

The backend only launches a `whatsappgo` that is installed next to it and that
no other account can replace. In a development tree whose directory is shared
with another user, notification actions are disabled on purpose and the backend
logs the reason. Installed packages place both programs in a root-owned
directory, where the action works normally.

## A media item says Download but does not open

Press **Download** and wait for the message to update to a cached path. Old
media may no longer be downloadable from WhatsApp. Confirm the cache directory
is writable and has free space. Clipboard image sending requires an image MIME
type, not only copied filename text.

## Safe diagnostic information

When reporting a problem, include the application version, Linux distribution,
Qt/Kirigami versions, the exact error, and whether software or RHI rendering is
used. Do not include QR contents, pairing codes, `device.db`, `messages.db`,
message text, phone numbers, or complete JIDs.
