# User guide

## Starting WhatsAppGo

Run the desktop executable or select WhatsAppGo from the application menu:

```bash
./desktop/build/whatsappgo
```

The application starts its bundled backend automatically. Do not start
`whatsappd` separately. When WhatsAppGo closes, it stops the backend processes
that it started. Messages and notifications therefore arrive on this computer
only while the application is open.

## Linking an account

### QR code

1. Start WhatsAppGo and wait for the QR code.
2. On the official phone application, open **Settings → Linked devices**.
3. Choose **Link a device** and scan the QR code.
4. Keep both devices online while the initial history and directory sync run.

Use **Refresh QR code** if it expires. The first QR is generated automatically.

### Phone pairing code

Enter an international phone number using digits only, without `+`, spaces, or
the domestic leading zero. Enter the resulting code in the official phone
application's linked-device flow.

## Multiple accounts

Use the **+** button beside the account tabs, enter a local profile name, and
link the second account. Each tab has separate credentials, history, cache, and
connection. Selecting a tab starts its backend if it is not already running.

The profile name identifies local storage; it does not change the WhatsApp
account name. Allowed characters are lowercase letters, digits, `_`, and `-`,
with a maximum of 32 characters.

## Chats and history

The chat list supports **All**, **Unread**, **Favorites**, and **Groups**
filters. Search above the list filters conversations; the search icon in the
navigation rail opens and focuses chat search.

Opening a conversation shows its newest messages. Scroll upward to load older
history in pages. All messages delivered to the linked device are persisted in
the profile's `messages.db` SQLite database; the UI does not keep the complete
database in memory.

WhatsApp sometimes sends the same contact under a phone-number identity and an
LID identity. WhatsAppGo consolidates verified pairs automatically so their
history appears as one conversation. It never merges contacts merely because
their names match.

The linked-device protocol controls how much old history is supplied. Messages
that WhatsApp never sends to this device cannot be reconstructed locally.

## Sending messages

- **Enter** sends a message; **Shift+Enter** inserts a line break.
- The paperclip attaches files and images.
- The smile button opens the native emoji picker.
- The microphone records and sends a voice note.
- Right-click a message to copy, reply, react, edit eligible sent text, or
  delete an eligible sent message for everyone.
- Select text inside a bubble and copy it normally. HTTP and HTTPS links open
  in the system browser.

## Images and media

Photos, videos, and stickers appear as pictures as soon as the conversation
loads. WhatsApp sends a small preview inside the message itself, so the image
is visible before the full file is fetched. **Download** on the preview gets the
full-size file; afterwards, clicking the picture opens it in the default Linux
application. Videos show a play badge and open in the system video player.

Paste a copied image into the composer to open the media preview. The preview
supports a caption and basic rotation before sending. Downloaded documents and
media are cached on disk. **Download** fetches an attachment that has not yet
been cached; **Open** launches the cached file with its default Linux
application.

Copying an image from a message places decoded image data on the desktop
clipboard, not merely its local filename.

## Navigation sections

- **Chats:** conversations, filters, search, and message history.
- **Calls:** call records supplied through WhatsApp app-state synchronization.
  Placing voice/video calls is not supported.
- **Statuses:** synchronized status messages available to the linked device.
- **Channels:** followed WhatsApp newsletters/channels exposed by the protocol.
- **Communities:** communities inferred from joined group metadata.
- **Profile:** account information and local appearance/settings controls.

Some sections can be empty until WhatsApp sends the corresponding data. An
empty call list does not mean calling is implemented.

## Appearance

Choose **System**, **Light**, or **Dark** mode from the application controls.
System mode follows the current Qt desktop color scheme. The chat wallpaper,
bubbles, text, icons, selection, menus, and scrollbars use matching semantic
colors.

Qt's software renderer is the default for consistent behavior on Linux. Users
with a known-good GPU driver may launch with `QT_QUICK_BACKEND=rhi`.

## Notifications and presence

Incoming messages use the freedesktop notification service unless the chat is
muted. Opening a chat sends read receipts for its incoming messages. Typing and
presence updates depend on what the other account and WhatsApp expose.

Because the backend belongs to the desktop lifecycle, closing WhatsAppGo stops
desktop notifications and disconnects its linked-device sockets until the next
launch. This does not unlink the device.

## Logging out and local data

Logging out asks WhatsApp to unlink that profile. Back up `device.db` and
`messages.db` together while WhatsAppGo is closed for a consistent local copy.

Deleting the cache removes downloaded media and avatars but not text history or
linked-device credentials. Never share either database, a QR payload, pairing
code, or logs containing full JIDs.

See [Troubleshooting](TROUBLESHOOTING.md) for common problems and
[Security and privacy](SECURITY.md) before using a sensitive account.
