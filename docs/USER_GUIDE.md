# User guide

This guide describes the current source implementation. The
[v0.1.9 release notes](releases/v0.1.9.md) identify fixes not included in the
existing v0.1.8 draft artifacts; rebuilding alone does not update a running app.

## Polls, invitations and community tools

Choose **Poll** in the attachment menu to review a question and 2–12 distinct
answers before sending. Received poll cards have **View poll**: select answers
and explicitly **Save vote**, or deselect your answers to remove your vote.
Results reflect the votes this device has received, not a server-wide tally.
The open poll refreshes periodically. Votes waiting for a decryption key are
labelled, and old polls without saved details may need another history sync.
Unsupported poll variants report an error rather than accepting an invalid vote.
If saving a vote fails, the selected choices remain available with an error so
you can retry. Check the result before retrying an uncertain connection failure.

Received **Event** cards open the name, dates, location and description, including
a cancellation flag when present. RSVP, event creation and edits are unavailable.

Group info → **Invite via link → QR code** displays the current invitation as a
scannable image. Treat it like the invite link: anyone with it can join or request
approval. **Join a group with a link** now previews the group before confirmation;
an approval request does not open a conversation until membership is confirmed.

Privacy → **Status audience → View audience contacts** lists the contacts included
or excluded by the current audience, with search. “My contacts” has no individually
enumerated list. This screen is read-only; change the audience on your phone.

The Calls, Channels and Communities search fields filter their loaded lists.
Open a community to read its description and linked groups. Current admins can
edit the description and link/unlink eligible groups after confirmation. The
announcement group cannot be unlinked here; linking requires admin rights in
both the community and the group. Permissions are rechecked when submitting.

Dropped-file captions use the same spelling preference and language as the chat
composer. Right-click a misspelled word, or use Menu / Shift+F10, for suggestions.
Escape closes the current review or details dialog. Closing a submitted action
does not cancel it: check its result before retrying after an uncertain error.

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

## View-once messages

View-once messages appear in chats as a dashed **1** icon and a notice to open
WhatsApp on your phone. They cannot be opened, downloaded or forwarded from
WhatsAppGo. Quoted view-once messages show a safe label, never their caption.
Older entries that were completely discarded need WhatsApp to supply their
metadata again; the app does not reconstruct protected content or invent rows.

## WhatsAppGo settings

Click the **gear at the top of the chat list** to open WhatsAppGo settings,
separate from WhatsApp account/privacy settings. Enter your own **GIPHY** or
**KLIPY** API key and choose a preferred provider, then click **Save**. Choose
**None** to disable the preference while retaining keys. Closing without saving
discards edits; clearing a key field and saving removes that key. A selected
provider requires a nonempty key.

Open the chat smile button and choose **GIFs**, or press **Ctrl+Alt+G**.
GIPHY opens with trending GIFs; KLIPY opens with featured GIFs. Both support
keyword search. Clear the search with the **×** button to return to browsing.
Select a result to preview its silent animation, then click **Send**. Saving a key does
not validate it; search reports invalid keys, connection problems and rate limits.
Google retired the
[Tenor API on June 30, 2026](https://support.google.com/tenor/answer/10455265?hl=en).
A legacy Tenor key can be retained, but Tenor cannot be selected for search.

Keys are masked in the form and stored in the desktop application's config
directory, in `private/gif-providers.json`. The file is **not encrypted**; on
Linux it is owner-readable/writable only (0600), in an owner-only directory
(0700). These settings apply across all local accounts, are not synced to
WhatsApp, and are not sent through the backend or included in diagnostics.

Provider search words and your IP are shared with the chosen provider; chat
contents, contact names and account identifiers are not. Search results are not
kept after closing the picker. A selected GIF is downloaded to a temporary file;
sent media is then kept as part of normal message history. Each search loads up
to 24 results; **Load more** adds another page, up to 120 per search.

The **Stickers** tab reuses stickers from this account's local history. Choose
**Starred** to find sticker messages you starred, select one and click **Send**.
No provider API key is needed. Unavailable originals may need downloading again.
Animated stickers preview in the picker when their original is cached. Use
**Pause** to stop a preview.

Choose **Create** in the Stickers tab, or **New sticker** from the paperclip
menu, to make a static sticker from a JPEG or PNG. The system file picker opens,
then WhatsAppGo prepares a private copy locally and shows a preview. It preserves
transparency and fits the whole image without cropping. Nothing is uploaded
until you click **Send**. Escape returns to the sticker list; switching accounts
or conversations discards an unsent sticker selection without changing your
text draft. A failed send keeps the preview available for retry.

Input is limited to 20 MiB and 32 megapixels. The result is a 512×512 WebP
within the static-sticker size limit; the original stays untouched. Creation
requires the build's optional libwebp support, not a GIF API key. Animated
sticker creation, background removal, drawing tools and provider sticker packs
are not available yet. Successfully sent stickers can be reused from history.

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

Group previews show the latest sender's name (or **You**). Unsent text appears
as **Draft:** in normal and archived chat lists, scoped to its account; search
results still show the matching conversation preview. Drafts are kept for the
current session only. A muted chat has a muted-notifications icon, which clears
when its mute expires. Hover a truncated chat name, or focus the row with the
keyboard, to see the full name. In selection mode, clicking the avatar selects
the chat instead of opening it or its status.

The chat menu's **Add to list** submenu shows current memberships with checks.
Choose a checked list to remove the chat, or an unchecked list to add it.

Opening a conversation, including selecting the same chat again, shows the
bottom of its newest message. Sending text with **Enter** or the send button
also returns to the bottom, even when you were reading older messages. Scrolling
up releases automatic following; incoming messages and contact-detail refreshes
do not pull you back down. Quoted-message and search-result navigation still
opens at the selected message. Scroll upward to load older history in pages.
While you are away from the latest message, a circular down-arrow appears at
the bottom-right of the conversation, above the composer. Click it to jump to
the newest message and resume following new messages. It disappears at the
bottom; scrolling down manually to the bottom also resumes following.
All messages delivered to the linked device are persisted in
the profile's `messages.db` SQLite database; the UI does not keep the complete
database in memory.

WhatsApp sometimes sends the same contact under a phone-number identity and an
LID identity. WhatsAppGo consolidates verified pairs automatically so their
history appears as one conversation. It never merges contacts merely because
their names match.

In a group, type **@** to find a member by name or phone number. Choose with
the mouse, or use **Up/Down** and **Enter/Tab**. Selecting inserts a highlighted
name; it does not send the message. **Escape** closes the suggestions first.
Selected tags retain their identities through undo/redo and per-chat drafts.
Converting emoticons before or after a tag also preserves its recipient;
Undo reverses that conversion as one operation.
Editing a tagged name or pasting its plain text does not silently tag someone.
Received tags show locally known names in bubbles and chat previews, including
older numeric tags when a cached identity can resolve them. Unknown identities
stay unresolved. This picker supports individual members, not **@everyone**.

Click a tagged name in a message to open that person's chat. Navigation uses
the stored member identity, so people with the same display name remain distinct.
Your group draft is preserved. You can still drag across a tag to select and
copy text. With keyboard focus on the message, **Tab/Shift+Tab** moves between
its tags and **Enter** opens the focused person's chat.

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
into memory. Photos and videos are grouped by month, with duration labels on
videos and a GIF badge on animations. Pictures open in the native photo viewer,
videos in the native player, and links in the system browser. Clicking a document
saves a copy in **Downloads**, fetching an uncached file first; existing files
are not overwritten.

Use **Select shared items** or a row/thumbnail checkbox to select photos, videos,
documents or links, then **Forward** to choose a destination and confirm sending.
In selection mode, clicking an item toggles selection rather than opening it.
Tab focuses controls; Enter/Space activates a focused item. **Escape** first
closes a viewer or forwarding dialog, then cancels selection, then returns from
shared content to Contact info and finally to the chat. Changing the category,
chat or account clears the selection. **View … from all chats** opens the
account-wide library at the same category without discarding your chat draft.
Use **Load more** when older local history is available. Bulk star/delete/download
controls in this drawer are not yet implemented.

**Starred messages** in Contact info shows stars from that conversation only.
The main menu's **Starred messages** shows stars across the current account.

Mute and archive changes are synchronized with WhatsApp. Calling, blocking,
reporting, and editing Favorites are not exposed
reliably by the linked-device API, so the drawer identifies or omits those
actions instead of displaying controls that would fail silently.

## Sending messages

- **Enter** sends a message; **Shift+Enter** inserts a line break.
- The paperclip opens the compact attachment menu. **Document**, **Photos and
  videos**, and **Audio** are functional. **New sticker** opens local image
  selection and a preview before sending. **Contact** searches locally known
  contacts or lets you enter a name and international phone number. Camera, Poll and Event are
  listed but report that the linked-device workflow is not supported yet.
- The smile button opens the native emoji picker.
- **Document** sends the selected file as a document even when it is a photo
  or video. **Photos and videos** retains the normal inline media presentation.
- File selection and saving use your system's native file picker (GTK on
  GNOME), including attachment and status selection, image saving and chat
  exports. If native integration is unavailable, Qt uses its built-in picker;
  see [file-picker setup](TROUBLESHOOTING.md#file-selection-does-not-use-the-system-picker).
- The microphone records a voice note; stop it to send. Switching conversations
  or accounts cancels the recording without sending it.
- Right-click a message to copy, reply, react, edit eligible sent text, or
  delete an eligible sent message for everyone.
- Select text inside a bubble and copy it normally. HTTP and HTTPS links open
  in the system browser.

When **Save** fails in the message editor, the correction remains in the open
dialog with an error. Retry after addressing the error; saving disables further
submissions until it finishes. A successful save closes the editor. Cancelling
or switching chats/accounts closes it without submitting another request.

Chat-scoped confirmations are bound to the conversation and account that opened
them. Switching either dismisses a pending confirmation instead of applying it
to the newly selected chat. Closing a dialog cannot undo a request already sent.

## Keyboard

Long drafts expand the message textbox up to its height limit, then scroll
inside it. A scrollbar stays visible while the text overflows; use the mouse
wheel or drag its handle to review earlier lines without scrolling the chat.

| Key | What it does |
| --- | --- |
| **Enter** | Sends the message. With **Enter is send** turned off in Settings the roles swap: Enter opens a line and **Ctrl+Enter** sends. |
| **Shift+Enter** | Always opens a line. |
| **Up arrow** | On an empty composer, opens the last message you sent for editing. Received messages, deleted ones, and anything that is not text are stepped over. A composer with something in it keeps the arrow for moving the cursor. |
| **Escape** | Goes back one level: close the current menu/dialog or viewer first, then a nested panel/settings page, then the previous section. Chats is the starting page; Escape there never quits or discards the draft. |
| **Menu / Shift+F10** | Opens the focused chat row's context menu, outside selection mode. |
| **Up / Down / Home / End in menus** | Moves between available actions; tall menus scroll to keep the focused action visible. Escape from a chat submenu returns to its parent action. |
| **Ctrl+,** | Opens Settings. |
| **Ctrl+Alt+P** | Opens your profile details. |
| **Ctrl+Alt+E** | Toggles the emoji picker in a conversation. |
| **Ctrl+Alt+Shift+P** | Pins or unpins the selected chat. |
| **Ctrl+Alt+/** | Searches all chats, including when no conversation is selected. |
| **Ctrl+Alt+Shift+] / Ctrl+Alt+Shift+[** | Opens the next / previous chat in the displayed list, respecting the active filter and preserving drafts. Stops at either end. |
| **Ctrl+Alt+Shift+L** | Opens the selected chat's list-membership picker. Checked lists already contain the chat. |
| **Ctrl+Alt+S** | Opens the sticker tab in the composer picker. |
| **Alt+I** | Opens the selected chat's contact or group information. |
| **Alt+A** | Opens attachments. Escape closes the menu. |
| **Alt+R** | Replies to the focused or hovered message without replacing the draft. |
| **Ctrl+Alt+D** | Opens the forwarding chooser for the focused or hovered message. Nothing is sent until confirmed. |
| **Alt+8** | Stars or unstars the focused or hovered message. |
| **Ctrl+Up** | Edits the last own text message when the composer is focused and empty, like the existing unmodified Up shortcut. |

Message shortcuts use keyboard focus before mouse hover and only target visible
messages in the current conversation. Deleted/system messages are ineligible;
view-once messages cannot be forwarded or starred. Conversation shortcuts are
disabled behind dialogs, viewers, selection modes, and non-chat pages. The
shortcut reference in Settings scrolls in smaller windows.

## Reading a conversation

A conversation is dated the way WhatsApp Web dates one: a pill between one
calendar day and the next says **Today**, **Yesterday**, the weekday for the
rest of the week, and a date for anything older. Loading older history moves
the pill onto the message that now opens that day.

A centered **1 unread message** (or **N unread messages**) pill on a faint
horizontal band marks the first unread incoming message. Its position stays
fixed as more messages arrive and read receipts update. Reopening the chat
starts a fresh unread batch; replying clears the previous divider. The marker
does not move you away from the bottom or interrupt scrolling through history.

Your own messages sit on the right with a tail on their top-right corner;
messages you received sit on the left. A tick beside your own time is one mark
for sent, two for delivered, and two blue for read.

## Images and media

Chat search shows who sent each result, its date, and a useful filename or media
label when there is no text. Use Up/Down and Enter to choose a result. Loading
and failed requests have separate states, and failed searches can be retried.
The calendar button finds the first locally stored message on a chosen day;
it does not request missing history from your phone. Escape closes the calendar
before closing search.

**Starred messages** searches the latest 100 loaded stars by text, sender,
filename, or chat name. Activating a result locates that message in its source
conversation. Older-star pagination is not available yet.
Removing a star on another device also removes it from the loaded results;
an older pending refresh cannot reintroduce that item. Reloading the global
media library similarly ignores pages requested before the reload.

In either a contact's shared-content drawer or **Media from all chats**, select
items to star/unstar them or save their originals to Downloads. The toolbar
shows the selected byte total and operation progress. Work is sequential;
failed items are reported and selection remains available for retry. Cancelling
selection stops queued work, not a request already submitted. These controls
do not delete messages. Local bulk deletion remains unavailable.

Videos have mute and volume controls, playback speed, a draggable position
slider, and fullscreen. With the viewer focused, Space plays/pauses, Left/Right
seek five seconds, M toggles mute, and F toggles fullscreen. Home/End on the
volume slider select minimum/maximum volume. Escape leaves viewer-owned
fullscreen first, then closes the video. These settings do not change voice-note
volume or speed.
Switching accounts stops active playback and prevents a pending download from
starting playback in the next account. A video viewer closes if its source
chat/account changes or the source message is revoked or removed.

Click a **GIF** or an animated sticker to play it directly in the conversation;
click again to stop it. GIFs loop silently. Only one chat animation runs at a
time, and it unloads when scrolled out of view, when another page/picker is
shown, or when the app loses focus. There is no automatic animation of every
message in a long chat. GIF identification is retained for newly received,
sent and forwarded messages; old records without that flag still use the video
viewer unless history supplies it again.

Animated WebP playback requires a build with libwebp/libwebpdemux. The local
Linux build includes it. Invalid or oversized stickers keep their static
preview and show an error if playback is requested. Sending always preserves
the original sticker, independent of local playback support.

Documents appear as compact cards with a file-type icon, wrapping filename,
type and available size. Click anywhere on the card (or focus it and press
Space) to save the file to your system **Downloads** folder. There is no
separate **Open** button, and saving does not launch another application.
An existing filename is preserved; another download uses a numbered name such
as `report (1).pdf`. A spinner indicates work in progress, and a notice shows
the saved path. Cached files can be saved while offline; missing files must
first be downloaded or recovered by the linked device. The shared-content
drawer retains its separate open-in-application action.

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

Photo/video bubbles stay compact: their outer width is capped at 336 logical
pixels and shrinks with the conversation pane. Long captions wrap below the
preview without widening the bubble or leaving an empty panel beside the image.
This limit applies only to chat previews, not the full-size viewer.
Missing or unreadable previews fall back to a descriptive attachment row (or a
video placeholder) without repeatedly restarting image loading. Replacing a
thumbnail with the full image keeps the same metadata-based bubble dimensions.

The account-wide media library has Media, Documents and Links tabs. Its sort
menu filters items sent by **Everyone**, **You**, or **Others**, and orders them
by **Newest**, **Oldest**, or **Largest** file size. Search, sorting and filters
apply to loaded items; **Load more** retrieves another page when older results
are available.

Clicking an item normally opens its conversation at that message, loading older
history as needed. In selection mode, clicking media, documents or links selects
them for forwarding instead. Changing tabs cancels the selection. Each item's
menu also offers Select, Go to message, Forward and Star/Unstar, plus Download
for attachments. Actions always use the item's source conversation. Download
saves the original file to Downloads without overwriting an existing filename.
If an attachment's file is unavailable, forwarding reports that it needs
downloading rather than sending only its caption.

Media tiles support Tab focus, Enter/Space activation and Shift+F10 (or the Menu
key) for item actions. Escape dismisses the open menu or selection first. Leaving
the library or switching accounts dismisses pending library menus and selections.

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
Pin and unpin changes synchronize across linked devices. Delayed history cannot
replace an explicit pin/unpin, and contact-ID merging preserves the newer action.
After upgrading from an affected version, WhatsAppGo requests one fresh chat
settings sync when connected to repair previously missing pins. If the phone
must supply recovery data, repair waits for that response; it does not repin
conversations on your behalf. Every computer needs the updated build.
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

To share a contact, choose **paperclip → Contact**, search by name or number,
and select a result. You can also enter the two fields manually. Review or edit
the name and phone number, then press **Send contact**. The number needs `+`
and its country code (7–15 digits). Only those two fields are shared, not the
rest of the address-book entry. This currently sends one card at a time.
Search includes archived contacts and resolves known phone aliases, but never
mistakes an opaque WhatsApp identity for a phone number. Results are limited to
100; narrow the search for larger address books. Escape returns from the review
to the list, then back to chat. Closing or changing accounts/conversations
before sending discards the selection without altering the composer draft.
A failed send keeps the reviewed fields for retry; closing a send already in
progress does not recall it.

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
supports a caption and basic rotation before sending. The caption field grows
as you type, then scrolls for longer text while keeping the cursor visible.
Use **Shift+Enter** for another line and **Enter** to send the image. The preview
image shrinks as needed so the caption and send controls remain accessible.
Downloaded documents and media are cached on disk. Click a document card to
save the original file to Downloads; there is no separate **Open** button on
the card. Photos and supported audio/video open inside WhatsAppGo. Explicit
download actions remain available for uncached attachments. Failed automatic
chat/status downloads can be requested again after a two-second cooldown;
expired or unavailable media may still fail.

Text, quoted-reply context, and pasted-image previews remain available if a send
fails. This includes the selected reply for file attachments and voice notes;
the quote is cleared only after the send succeeds. A successful acknowledgement
clears only the draft that was sent, not
newer typing or another chat's draft. An image preview is hidden when leaving its
account/chat and shown again on return. These are in-session drafts, not a
persistent outbox; check delivery before retrying after a connection loss.

Copying an image from a message places decoded image data on the desktop
clipboard, not merely its local filename.
A newer image or text copy takes precedence over an older image download that
finishes afterward.

## Creating a group

Open **New chat → New group** (also available in the chat-list menu or with
**Ctrl+Shift+N**). Search names or numbers and select members, then choose
**Next**, enter a group name, and press **Create**. Nothing is sent until Create.
Search includes recent and archived chats and locally saved contacts.

Back or Escape from the name step returns to member selection without losing
your choices. Escape again returns to New chat. A failed request keeps the name
and selection available for retry; successful creation opens the new group.
Closing during creation does not cancel a request already sent to WhatsApp.
If the connection drops before confirmation, check your chats before retrying
to avoid making a second group. Changing accounts closes the creator.

Group names support up to 100 characters, including emoji. You can select up
to 1023 other members; WhatsApp includes you automatically and applies members'
privacy settings.

## Creating a community

Choose **New chat → New community**, or the **+** on the Communities page.
Enter a name (up to 100 characters, including emoji) and choose **Create**.
WhatsApp adds the announcement group. Successful creation opens Communities;
a failed request keeps the name and an inline error available for retry.

Escape or Cancel returns to the page you came from. Closing a pending creator
does not cancel an operation already sent to WhatsApp, and another submission
is blocked until it finishes. If confirmation is lost, check Communities before
retrying to avoid creating a duplicate. Switching accounts closes the creator.

Community photos, descriptions and linking groups are not configurable in this
creator yet; manage those details from WhatsApp on your phone.

## Group information

Click a group's name or avatar in the chat header to open **Group info**. The
panel fetches current membership from WhatsApp, separately from locally stored
chat history. It shows the description, creation details, member count, available
names/photos/phone numbers, **You**, and **Group admin** badges. An opaque WhatsApp
identity is never displayed as a phone number. Loading failures offer **Retry**.

Members allowed to edit group information see **Add group photo** or **Edit
group photo** below the avatar. **Choose photo** opens the system file chooser
(GTK on GNOME when its Qt integration is available). Select a still JPEG or PNG
up to 20 MiB and 32 megapixels. Review the centered square crop, then choose
**Save**; the original file is not changed. The crop is automatic, not adjustable.
**Remove photo** asks for confirmation and changes the photo for the whole group.
Canceling the file chooser keeps your preview. Escape backs out of removal
confirmation, then returns from the editor to Group info. Errors keep the
preview and require reopening before another upload. Closing during a submitted
save does not cancel it. Changes recheck membership and edit permission online.

The first eight members appear inline. Use the search icon or **View all members**
to open the full searchable list. Click a person to message them; group admins
also get confirmed remove/promote/demote actions for eligible members.
Member details have small inline copy icons directly beside the full name and
available phone number.
The name/number at the top of contact info are copyable too (including group names).
Use the copy icon for the whole value, or select text and press **Ctrl+C** or
right-click **Copy** for a selection. Phone copies include the international `+`;
copying your own member entry uses your name, not the label **You**.

- **Add member** selects from available local contacts. WhatsApp's group and
  invitation-privacy rules still apply; a rejected or partially applied request
  shows an error and refreshes membership.
- **Invite via link** loads and copies the group link for permitted members.
  Admins can reset it after confirmation, invalidating the old link.
- **Create similar group** starts with the current members selected (excluding
  yourself), lets you adjust the name and selection, and requires confirmation.
  Creation/addition is limited to 100 selected people per action.
- **Group permissions** shows who can send messages, edit group information,
  and add members, plus whether new members require admin approval. Current
  admins can select a setting and explicitly **Save** one change at a time;
  other members get a read-only overview. **Escape** cancels an unsaved choice
  back to the overview, then returns to Group info. Failed saves keep your choice
  and show an error. Closing while saving does not cancel a submitted change.
  Community containers, announcement groups and suspended groups are read-only
  here; unknown server values are marked unavailable. The approval toggle sets
  the joining policy; individual applications are reviewed separately.
- **Pending join requests** lets current group admins search applicants by name
  or known phone number, then confirm **Approve** or **Reject** for one person.
  Approve adds that person to the group; Reject dismisses their current request
  without blocking them. **Escape** or **Cancel** returns to the list without
  sending a decision, and Escape from the list returns to Group info.
  Each decision rechecks admin rights and the request's identity/time. If a
  request has changed, or a response is uncertain, use **Refresh requests**
  before deciding again. Unknown request times cannot be reviewed. Closing an
  already-submitted decision does not cancel it; no background retry is made.
- **Notification settings**, starred messages, encryption information,
  disappearing-message timers, favorites, custom lists, archive, export, and
  clear-chat actions are available in the group panel.
- **Exit group** asks for confirmation and leaves the group without deleting
  this computer's local history or media. Deleting the retained chat is separate.

Advanced chat privacy, member tags, and group reporting are not implemented by
this client. Their entries explain the limitation and direct you to the official
WhatsApp app; they do not pretend to change settings or submit a report.
Disappearing timers likewise do not expire WhatsAppGo's locally retained history.

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

### Viewing statuses

Open a contact's status from the Status page or their chat avatar. **Escape**
closes the emoji picker first if it is open, then the viewer, returning focus to
the page/control you came from. Opening a status from a chat does not replace
that chat or its draft; opening from Status returns to the Status page.

The viewer has **Pause/Resume**, **Previous**, **Next**, and **Close** controls.
Tab and Shift+Tab cycle within the viewer. Space pauses/resumes when the viewer
itself is focused; in the reply field it types a space as usual. Typing a reply,
choosing emoji or waiting for a reply to send also pauses automatic advancement.
The explicit pause choice remains in effect when you move to another status,
and resets when you reopen the viewer. Controls remain legible over bright
photos and videos, and the overlay blocks input to the chat underneath.
Refreshing the same status or loading its media does not erase a reply draft
or reset a pending reply. Moving to a different status or account resets the
reply state; it is not saved as a persistent outbox.

### Viewing chat photos

Open a chat photo to browse the photos currently loaded for that conversation
or its shared-media panel. Use **Previous/Next**, the **Left/Right** keys, or the
thumbnail strip; the sender, timestamp and caption follow the selected photo.
This gallery does not yet combine videos/GIFs or fetch all historical photos.

The toolbar offers **Go to message**, **Reply**, **Star/Unstar**, **Pin**,
**React**, **Forward**, and **Download**. Narrow windows put these actions in
the photo-actions menu. Reply preserves your existing draft. Pin asks for
24 hours, 7 days or 30 days; opening a reaction or forwarding picker does not
send anything. Download saves the original file to Downloads, keeping existing
files when names collide. **Copy image** and **Save image as…** remain available.

Zoom with the wheel or the zoom buttons. Drag an enlarged photo to pan it, or
use **Shift+Arrow** keys. Moving to another photo resets zoom and pan. Escape
closes an open picker/menu first, then returns from the viewer to the chat.
Changing chats or profiles closes message-action dialogs. View-once and revoked
images are excluded; avatar previews never offer message actions.

## Appearance

Click the moon/sun icon directly below the update icon in the navigation rail
to switch between light and dark mode. Your choice is remembered after restart.
Choose **System**, **Light**, or **Dark** from **Settings → Chats → Appearance**.
System mode follows the current Qt desktop color scheme. The chat wallpaper,
bubbles, text, icons, selection, menus, and scrollbars use matching semantic
colors.

Qt's software renderer is the default for consistent behavior on Linux. Users
with a known-good GPU driver may launch with `QT_QUICK_BACKEND=rhi`.

## Notifications and presence

Open the profile icon, then **Notifications**, to configure this account's
desktop alerts. **Messages** and **Groups** each have notification, reaction-alert
and sound switches. Reaction alerts default off and only apply to reactions to
your own messages. **Status** offers optional alerts for newly received status
updates and a separate sound switch. Status alerts default off; your own updates,
muted contacts, edits and history sync stay quiet. Clicking an alert opens the
Status page without changing the current chat or its draft. Likes and mentions
are not supported yet. **Calls** enables one-time incoming-call alerts; answer on
your phone, not in WhatsAppGo. **Show
message previews** controls the notification body (the sender name still
appears). **Allow incoming sounds** is the incoming-sound master switch.
**Play a sound when sending messages** optionally plays a tone after successful
text, attachment and forwarded sends, not failures or history sync. The two
**Test sound** rows let you check the output without sending a message.
Sound switches and previews are implemented on Linux; on other
platforms use the operating system's notification settings. These preferences
are saved on this computer per profile and do not change WhatsApp phone
settings or unmute individual chats. Disabling alerts never stops history sync.

Settings now includes search and Profile, Account, Privacy, Chats,
Notifications, Keyboard shortcuts, and Help and feedback. Profile names and
phone numbers are copyable; **Edit profile name** changes the WhatsApp name.
**Chats → Media auto-download** controls photos, video/GIFs, audio/voice notes,
documents and stickers independently. Manual downloads remain available, and
already-running transfers may finish. Re-enabling a type also allows it in the
next background history scan. **Privacy → Disable link previews** prevents new
website requests for composer and upgraded chat previews; cached previews stay
visible. Privacy also exposes who may call/message you and protection against
high message volumes from unknown accounts, when WhatsApp returns those values.
**Chats** includes system/light/dark appearance,
doodle wallpaper, emoticon replacement and Enter-to-send preferences. Items
not implemented in this client open an explanation, not a pretend toggle:
account-report/deletion controls, app lock and call-IP protection remain
unavailable here. Status like/mention alerts and answering voice/video calls are
not implemented. Username editing still uses the official app.

**Account → Security notifications** optionally alerts you when a contact's
security code changes. Alerts are silent on Linux, respect muted chats and suppress
duplicates. A code change can happen after reinstalling or changing phones;
verify the code in the official app. Encryption does not depend on this toggle.
Other desktops use their operating system's notification sound settings.

**Profile** reads your current About and available profile photo. About edits
remain in the field if saving fails. **Privacy → About** controls About
visibility; **Status audience** separately displays the broadcast audience and
the number of included/excluded contacts. Edit audience exception lists on the
phone.

**Profile → Change profile photo** opens the system file chooser (GNOME on a
configured GNOME desktop). Select a still JPEG or PNG under 20 MiB and 32
megapixels, review the centered square preview, then press **Save**. Preparation
runs off the UI thread, respects photo orientation, removes location/EXIF
metadata from the upload, and leaves your original unchanged. Transparent areas
become white. Nothing is uploaded just by selecting a file; Escape/Cancel
discards the selection. Failed saves keep the preview for retry. **Remove profile
photo** requires a separate confirmation. Switching accounts discards an unsaved
selection; closing the dialog after Save does not undo an already-submitted change.

**Privacy → Default message timer** offers Off, 24 hours, 7 days and 90 days.
Select a duration, then Apply and confirm. The current account default cannot
be read by this linked-device library, so the picker starts without an assumed
value. This changes new-chat defaults only. **WhatsAppGo retains its local
history even when disappearing messages are enabled.**

**Chats → Photo upload quality** applies to attached and pasted JPEG/PNG photos:
Standard uses JPEG quality 80 with a 1600-pixel maximum edge; HD uses quality 90
and 3840 pixels. Small images are not enlarged. Conversion honors camera
orientation, flattens transparency to white and leaves the source file intact.
Original is the default and sends unchanged bytes, including metadata.
Documents, animations and videos are unchanged. These are WhatsAppGo's local
resize presets, not a claim to reproduce WhatsApp's HD encoding or badge.

**Chats → Spell check** enables offline Aspell checking in the main chat
composer. Choose an installed dictionary, then right-click an underlined word
for suggestions or Ignore word. Shift+F10 opens the same edit menu. Corrections
support normal Undo. No draft text goes to a spelling service; URLs, addresses
and inline backtick code are skipped. Captions and message-edit dialogs are not
spell-checked yet. The feature is off by default and requires Aspell plus a
dictionary; no packages are installed automatically. Drafts over 20,000
characters pause spelling to keep typing responsive.

Incoming messages use the native desktop notification service or portal unless
the chat is muted; notification delivery does not depend on a tray icon. If a
minimal desktop session installed but did not start `notification-daemon`,
WhatsAppGo safely starts the trusted system copy when its backend starts.
Clicking a notification opens its conversation. When the desktop provides a
system tray, WhatsAppGo also places its icon there with connection status,
**Open/Hide**, and **Quit WhatsAppGo** actions. Minimizing hides the window
behind that icon. In the current development build, opening a chat or receiving
messages there sends read receipts only while the conversation is active and
visible. Receipts wait while the window is hidden, minimized, inactive, or
showing another section or a photo/video/status viewer. Returning to the chat
acknowledges its loaded incoming messages, not only the individual bubbles
currently onscreen. Another linked device can still mark messages read.
Typing and presence updates depend on what the other account and WhatsApp expose.
Typing and audio-recording indicators clear immediately when a stop/offline event
arrives, or after 10 seconds without a fresh activity update if the stop event is
lost. The header then returns to the available online/last-seen information, or
stays blank when none is known. Changing chats or losing the connection also
clears transient activity.
In group chats the header identifies who is typing or recording audio, including
multiple members at once. Each member's activity clears independently. Saved
names are preferred; unknown contacts use a known phone number or a labelled
member identifier, never an opaque WhatsApp ID presented as a phone number.
Long names are shortened in the header; hover over the header to read the full
status. One-to-one chats keep the usual typing, recording and last-seen labels.
Status-broadcast updates stay quiet unless enabled in **Notifications → Status**.

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

**Report a problem**, from the toolbar or Help, opens
[WhatsAppGo GitHub issues](https://github.com/shukiv/whatsappgo/issues) directly
in your default browser. There is no in-app form or intake key to configure.
Your browser controls whether the page opens in a new tab or window.

Review existing issues or create a new issue on GitHub. Opening the page does
not submit a report or attach app data, logs, or account details. If the browser
cannot launch, the app shows the URL so you can open it manually.
See [bug reporting](BUG_REPORTING.md).

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
