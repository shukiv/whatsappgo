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

`whatsappctl` can control that same app-owned backend from a shell or bot. It
does not start another daemon. See [Command-line and bot API](API.md).

## Linking an account

### QR code

1. Start WhatsAppGo and wait for the QR code.
2. On the official phone application, open **Settings → Linked devices**.
3. Choose **Link a device** and scan the QR code.
4. Keep both devices online while the initial history and directory sync run.

Use **Refresh QR code** if it expires. The first QR is generated automatically.
New QR-linked registrations appear as **WhatsAppGo** in the phone's linked-device
list. An entry created by an older build keeps its original name until it is
logged out and paired again.

### Phone pairing code

Enter an international phone number using digits only, without `+`, spaces, or
the domestic leading zero. Enter the resulting code in the official phone
application's linked-device flow.

## Multiple accounts

Use the **+** button beside the account tabs, enter a local profile name, and
link the second account. Each tab has separate credentials, history, cache, and
connection. Selecting a tab starts its backend if it is not already running.

The internal profile key identifies local storage and does not change the
WhatsApp account name. Use the pen beside an account in the switcher to give it
a local display name; this label may contain spaces and non-Latin characters.

In **Settings → Privacy**, **Last seen** and **Online** are separate choices.
To hide both, set **Last seen** to **Nobody** and **Online** to **Same as last
seen**. Privacy settings are refreshed for the selected account after connecting
or switching accounts and when the settings page is reopened.

## Chats and history

The chat list supports **All**, **Unread**, **Favorites**, and **Groups**
filters. At narrower sidebar widths, **Groups** moves into the chevron overflow
instead of squeezing the direct filters. The **Unread** label stays stable; the
unread total is shown on chat and navigation badges. Search above the list
filters conversations; the search icon in the navigation rail opens and focuses
chat search. The **Archived** row follows the filter strip.

Opening a conversation shows its newest messages. Scroll upward to load older
history in pages. All messages delivered to the linked device are persisted in
the profile's `messages.db` SQLite database; the UI does not keep the complete
database in memory.

WhatsApp sometimes sends the same contact under a phone-number identity and an
LID identity. WhatsAppGo consolidates verified pairs automatically so their
history appears as one conversation. It never merges contacts merely because
their names match.

Consolidation preserves deletions, edits, stars, and attachment metadata. Old
attachments remain recoverable from their original archive identity even after
their cached files have been removed. **Mark all as read** covers all stored
active and archived conversations, not just the first page.

The linked-device protocol controls how much old history is supplied. Messages
that WhatsApp never sends to this device cannot be reconstructed locally.

WhatsAppGo intentionally keeps as much local conversation history as possible.
Disappearing-message timers do **not** automatically delete messages already
stored by this app, hide them from local history/search, or expire stored
attachments. The **Disappearing messages** setting changes the WhatsApp chat
timer, not WhatsAppGo's local retention policy. This is intentional, not a bug.
Explicit deletion actions and message revocations remain separate; this policy
does not change them or the 24-hour visibility of Status stories. See
[Security and privacy](SECURITY.md) before relying on disappearing messages to
remove local copies.

Click the avatar or contact name in the conversation header to open **Contact
info**. It shows the locally known avatar, phone number, shared-content count,
mute state, encryption information, and archive action. Select **Media, links
and documents** to open the three category tabs. Those views query the selected
chat's SQLite history and page through it without loading the full conversation
into memory. Pictures open in the native viewer, documents open in their normal
Linux application, uncached media is downloaded first, and links open in the
system browser. Press **Escape** or use the close/back button to leave the
drawer.

**Starred messages** in Contact info shows stars from that conversation only.
The main menu's **Starred messages** shows stars across the current account.

Mute and archive changes are synchronized with WhatsApp. Calling, blocking,
reporting, and editing Favorites are not exposed
reliably by the linked-device API, so the drawer identifies or omits those
actions instead of displaying controls that would fail silently.

## Sending messages

- **Enter** sends a message; **Shift+Enter** inserts a line break.
- The paperclip opens the compact attachment menu. **Document**, **Photos and
  videos**, and **Audio** are functional. Camera, Contact, Poll, Event, and New
  sticker are listed but report that the linked-device workflow is not supported
  yet.
- The smile button opens the native emoji picker.
- **Document** sends the selected file as a document even when it is a photo
  or video. **Photos and videos** retains the normal inline media presentation.
- The microphone records a voice note; stop it to send. Switching conversations
  or accounts cancels the recording without sending it.
- Right-click a message to copy, reply, react, edit eligible sent text, or
  delete an eligible sent message for everyone.
- Select text inside a bubble and copy it normally. HTTP and HTTPS links open
  in the system browser.

## Keyboard

| Key | What it does |
| --- | --- |
| **Enter** | Sends the message. With **Enter is send** turned off in Settings the roles swap: Enter opens a line and **Ctrl+Enter** sends. |
| **Shift+Enter** | Always opens a line. |
| **Up arrow** | On an empty composer, opens the last message you sent for editing. Received messages, deleted ones, and anything that is not text are stepped over. A composer with something in it keeps the arrow for moving the cursor. |
| **Escape** | Closes the open menu or emoji picker. |

## Reading a conversation

A conversation is dated the way WhatsApp Web dates one: a pill between one
calendar day and the next says **Today**, **Yesterday**, the weekday for the
rest of the week, and a date for anything older. Loading older history moves
the pill onto the message that now opens that day.

Your own messages sit on the right with a tail on their top-right corner;
messages you received sit on the left. A tick beside your own time is one mark
for sent, two for delivered, and two blue for read.

## Images and media

Photos, videos, and stickers appear as pictures as soon as the conversation
loads. WhatsApp sends a small preview inside the message itself, so the image
is visible before the full file is fetched. **Download** on the preview gets the
full-size file; afterwards, clicking the picture opens the native aspect-fit
viewer. At 100% the whole source remains visible, including tall and wide
screenshots. Scroll up over an opened image to zoom in and down to zoom out;
no modifier key is needed. Wheel zoom stays anchored at the pointer and ranges
from 100% to 500%. The toolbar zoom buttons remain available, and scrolling
over chat lists or message thumbnails keeps its normal behavior. Clicking an
open image exposes **Copy image** and **Save image** actions.

Selecting an item in the account-wide media library opens its conversation and
jumps to that message, loading older history as needed. To forward an attachment,
download it first. If its file is unavailable, forwarding reports that it needs
downloading rather than sending only its caption.

A message containing a link shows a preview card with the page title,
description, and picture. Normally WhatsApp resolves that preview on the
sending device and includes it in the message. While composing a new link,
WhatsAppGo resolves its card so it can be reviewed before sending.

Some historical YouTube messages arrive with generic text but no picture.
WhatsAppGo performs one background, YouTube-only metadata pass for those rows,
caches the resulting thumbnail locally, and updates the existing SQLite message
without changing its time or delivery state. Historical links to other sites
are not fetched and remain plain when WhatsApp supplied no preview.

Photos and videos that arrived through history synchronisation carry no
preview either, so the conversation downloads the attachments it is showing, a
few at a time: pictures up to 8 MB, videos and documents up to 25 MB. Anything
larger keeps its **Download** action. Voice notes are fetched when you play them.

Pinned conversations stay at the top of the list with a pin beside their time.
Right-click a conversation to archive it, mute it, pin it, or mark it read or
unread; each change is sent to WhatsApp, so it applies to your phone too.
Archived conversations live behind the **Archived** row below the filters.

Hover a message to reveal its reaction button and action arrow. Reactions are
grouped below the bubble with a count when several people choose the same emoji;
click the badge to see who left which one. A thumb is a thumb whoever left it,
so the skin tones people choose are grouped together.
Menus are repositioned when opened near an edge so their final actions remain
inside the application window.
The message menu can pin a message for 24 hours, 7 days, or 30 days. A pinned
message appears above the conversation; click it to go to the message or unpin
it. These actions are synchronized with WhatsApp rather than kept only locally.

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
starting another stops the previous one. Starting supported audio or video
playback marks the message as played/read. Nothing is handed to a web browser.

Paste a copied image into the composer to open the media preview. The preview
supports a caption and basic rotation before sending. Downloaded documents and
media are cached on disk. **Download** fetches an attachment that has not yet
been cached; **Open** launches the cached file with its default Linux
application.

Text, quoted-reply context, and pasted-image previews remain available if a send
fails. A successful acknowledgement clears only the draft that was sent, not
newer typing or another chat's draft. An image preview is hidden when leaving its
account/chat and shown again on return. These are in-session drafts, not a
persistent outbox; check delivery before retrying after a connection loss.

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

Incoming messages use the native desktop notification service or portal unless
the chat is muted; notification delivery does not depend on a tray icon. If a
minimal desktop session installed but did not start `notification-daemon`,
WhatsAppGo safely starts the trusted system copy when its backend starts.
Clicking a notification opens its conversation. When the desktop provides a
system tray, WhatsAppGo also places its icon there with connection status,
**Open/Hide**, and **Quit WhatsAppGo** actions. Minimizing hides the window
behind that icon. Opening a chat sends read receipts for its incoming messages.
Typing and presence updates depend on what the other account and WhatsApp expose.
Status-broadcast updates do not create desktop notifications.

With a tray available, minimizing or closing the window hides it and keeps notifications and
the linked-device connection active. Use **Quit WhatsAppGo** in the tray menu to
stop the application and its backend. Without a tray host, closing the window
still quits normally. Neither action unlinks the device.

## Updating

A packaged build looks for a newer release every three hours and on the first
start of the day, and asks once when it finds one. The circular arrow in the
left-hand rail checks now; its tooltip names the version this copy is running,
and a dot appears on it while an update is waiting. **Settings -> Help** has the
same button.

Accepting an update downloads the file, checks it against the checksums the
release publishes, and installs it: on Linux the AppImage replaces itself and
the window reopens, on Windows the installer takes over, and on macOS the disk
image opens for you to drag across.

A build made from source reports itself as built from source and is never
offered an update, because a working copy is not behind anything. `git pull` is
the update there.

## Reporting a problem

**Report a problem** submits your description and the displayed environment
details to the `whatsappgo` project at `bugs.jabali-panel.com`. Reports require
an intake key configured for the app's backend; GitHub CLI/login is no longer
used. Errors leave the report editable. No chat logs or account identities are
automatically attached. See [bug reporting and key setup](BUG_REPORTING.md).

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
