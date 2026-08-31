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
application.

A message containing a link shows a preview card with the page title,
description, and picture. WhatsApp resolves that preview on the sending device
and includes it in the message, so opening a conversation never contacts the
linked sites. Messages that were synced from your phone's history before this
version arrived without their preview and keep showing the plain link.

Photos and videos that arrived through history synchronisation carry no
preview either, so the conversation downloads the attachments it is showing, a
few at a time: pictures up to 8 MB, videos and documents up to 25 MB. Anything
larger keeps its **Download** action. Voice notes are fetched when you play them.

Pinned conversations stay at the top of the list with a pin beside their time.
Right-click a conversation to archive it, mute it, pin it, or mark it read or
unread; each change is sent to WhatsApp, so it applies to your phone too.
Archived conversations live behind the **Archived** row above the filters.

Beyond what is on screen, the application keeps collecting in the background:
older messages first, then the attachments belonging to them. Both run slowly on
purpose and continue across restarts, so a freshly linked account fills in over
hours rather than all at once.

A shared contact appears as a card with the name and number from the card the
sender sent, and a **Message** action that opens a conversation with that
number. A shared place appears with the map picture the sender included;
clicking it opens the location in your usual map site.

Voice notes and audio messages play in the conversation. The bubble draws the
waveform the sender recorded, with its length beside the timestamp. Press the
play control; the waveform can be clicked to seek. When a recording ends, the
next one in the conversation plays automatically, and the run stops as soon as
the conversation returns to text. Videos play in the
window: click the preview to open the player, then use the controls at the
bottom or press **Escape** to close it. One recording plays at a time, and
starting another stops the previous one. Nothing is handed to a web browser.

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

Attachments are kept in `media.db` next to the message history. The media cache
directory only holds copies that the interface reads, so deleting it frees disk
space without losing pictures, voice notes, or documents: they are written back
from the database the next time they are opened. Deleting the cache also removes
downloaded avatars, which are fetched again.

Back up `device.db`, `messages.db`, and `media.db` together. Never share any of
them, a QR payload, a pairing code, or logs containing full JIDs.

See [Troubleshooting](TROUBLESHOOTING.md) for common problems and
[Security and privacy](SECURITY.md) before using a sensitive account.
