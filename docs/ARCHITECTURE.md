# Architecture

## Overview

WhatsAppGo is one user-facing desktop application implemented by two native
processes:

```text
whatsappgo (Qt 6 / QML)
   │ starts, monitors, and stops
   ├── whatsappd --profile default
   ├── whatsappd --profile work
   │
   ├── JSON-lines RPC over per-profile Unix sockets
   │        ▲
   │        └── whatsappctl / authorized same-user bots
   │
   └── app-owned backend lifecycle
          │
          ▼
       whatsmeow ── encrypted WhatsApp multi-device connection
          │
          ├── device.db      keys, sessions, app state
          ├── messages.db    chats, messages, aliases, receipts
          ├── media.db       durable attachment bytes
          └── media/         materialized media, previews, and avatars
```

The Go helper remains a separate process for fault isolation and because the
protocol implementation is Go, but it is an internal application component.
The user never starts or manages it. The desktop launches a helper when an
account socket is unavailable, keeps helpers alive while the application is
open, and terminates the helpers it owns during shutdown. Switching account
tabs starts that account's helper if necessary. Lightweight monitor connections
keep every saved account online and its unread total current until the desktop
closes, including while another account tab is selected.

A helper cannot outlive the client that started it. A clean exit stops the
helpers it owns, and three mechanisms cover the exits that are not clean - a
crash, a SIGKILL, a machine that runs out of memory. The client starts every
helper with `--exit-with-parent`, so the helper watches its own parent process
id and shuts down once it has been reparented (see `internal/parentwatch`); on
Linux the child also sets `PR_SET_PDEATHSIG`, which arrives immediately; on
Windows, where an orphan keeps its parent id, the client puts every helper in a
job object that terminates its members when the client's handle closes.

An already-running compatible helper can still be used during development,
but normal packages do not install or require systemd user units.

## Responsibilities

### Desktop

- owns the application window and backend child-process lifecycle
- renders virtualized chat/message lists and native controls
- keeps only visible/paged message data in memory
- preserves list anchors across pagination and in-place model refreshes
- renders aspect-fit images and audio/video playback inside the application
- sends typed RPC requests and consumes live events
- manages account tabs, theme preference, clipboard, file dialogs, system tray,
  and interactive desktop notifications

The desktop treats `chat.presence` composing/recording events as transient hints.
A single-shot 10-second timer renews only on activity for the selected chat, not
on general online updates. Expiry removes only activity fields and emits the
presence notification, preserving online/last-seen information. Paused/offline
events clear activity immediately; navigation, account switches, and WhatsApp or
daemon disconnects cancel the timer. This prevents a lost stop event from
leaving the header stuck on “Typing…”. Group activity is keyed by normalized
sender JID, with an independent expiry per member. One member pausing or timing
out does not clear anyone else. The daemon adds `sender_name` using local saved
names, contacts and PN/LID mappings, without fetching group rosters or creating
chat rows. The header names typing/recording members, with a cached-roster or
identity fallback, and never treats a group as online or last seen.

### Backend

Pin state in `messages.db` has two clocks: `pinned_at` orders visible pins and
is zero when unpinned; `pin_action_at` retains the latest explicit pin **or unpin**
timestamp. History snapshots can seed pins only before an explicit action is
known. App-state writes atomically reject older actions, and alias consolidation
merges this versioned state inside its existing transaction. The v7 chat-settings
backfill replays settings once for older profiles whose history import could
overwrite synchronized pins. An unsuccessful replay is not marked complete;
existing phone-recovery backoff remains in force. It does not send pin mutations.

- owns the WhatsApp connection, encryption sessions, and reconnection
- converts whatsmeow events into application models
- imports history and app-state records
- persists messages before emitting UI events
- extracts the preview picture embedded in each media message
- uploads/downloads media and emits notification events; it uses the desktop
  notification service or portal when no desktop tray host is available
- exposes a small local RPC API; it does not expose HTTP or a network port

Native notifications are always owned by the backend. Sender avatars and the
desktop's instant-message sound are included in the platform payload. It first uses or asks
D-Bus to activate the freedesktop notification service. Minimal X11 sessions
sometimes install `notification-daemon` without activating it; in that case the
backend starts only a trusted system copy and waits for it to own the service.
The desktop portal remains the fallback for sandboxed environments. Tray
availability affects only window lifecycle: the desktop registers an icon
eagerly, polls for a late tray host, and hides a minimized window only while
that icon is actually available. This prevents losing the window on GNOME
installations without an AppIndicator host.

Notification text is escaped for a server that advertises `body-markup`, and a
refused notification is re-emitted as `notification.received` with `handled=0`
so the window presents it with the platform's own API rather than the reader
being told nothing. On Linux that platform API is the tray balloon, which the
same notification service draws, so the second attempt can fail for the same
reason; the message still reaches the chat list and the conversation. The alert-deduplication table drops its oldest entry when
full, so a run of status updates cannot silence a call or security-code alert.

Conversation sends - `message.send`, `message.send_media`, `message.send_contact`,
`message.forward` and `sticker.send` - accept only person and group addresses.
A status update arrives as an ordinary message whose chat is the status
broadcast address, so answering the chat an event came from would otherwise
publish a status update. `status.post` remains the way to publish one, and a
reply to a status is addressed to the person with the status named in
`reply_chat_jid`.

## Account isolation

Every account is a profile name matching
`^[a-z0-9][a-z0-9_-]{0,31}$`. A profile has its own device database, message
database, cache directory, socket, and backend process. Cryptographic state is
never shared between profiles.

The default profile uses the root application data directories. Other profiles
use `profiles/<name>/`. Profile display names are stored locally in Qt settings;
no account name is hardcoded in the program.

## Storage and history

`device.db` is owned by whatsmeow and contains linked-device credentials,
Signal sessions, app-state keys, and protocol state. `messages.db` is owned by
WhatsAppGo and contains chats, messages, media payload references, reactions,
call records, aliases, and migration metadata.

SQLite uses foreign keys, WAL mode, a busy timeout, and one connection per
store. Directories are mode `0700`; databases and sockets are mode `0600`.

Messages are written before `message.upsert` is emitted. History pages contain
at most 50 messages by default. The desktop opens at the newest page, requests
older local pages, and asks WhatsApp for more linked-device history at a local
boundary. WhatsAppGo can store only history WhatsApp sends to the linked device.

### Local history retention policy

The product requirement is to preserve as much available conversation history
as possible. WhatsApp disappearing-message timers are conversation settings,
not local retention deadlines: messages already received and stored by
WhatsAppGo remain in local history after the timer elapses. This is intentional
behavior, not a missing expiration-cleanup feature.

Do not add timer-based deletion, expiry filtering in history/search, or
automatic removal of stored attachments solely because a conversation's
disappearing-message timer elapsed. Changing that timer must not retroactively
prune local history. Explicit deletion actions and the existing handling of
message revocations are separate and are not changed by this policy. The
24-hour visibility of Status stories is also separate from conversation history.

Retention cannot recover messages or attachments that WhatsApp never supplied
to this device. See [Security and privacy](SECURITY.md) for the implications of
keeping local copies beyond a disappearing-message timer.

### Conversation settings

Mute, pin, and archive state belongs to WhatsApp. Only an initial, recent, or
full history sync carries it, and only those replace the local values. Contact
directory synchronisation, group metadata events, and starting a chat by phone
number know a conversation's identity but not its settings, so they merge the
title, avatar, and activity time and leave the rest untouched. On-demand
history pages are treated the same way: an absent field there means "not
included", never "cleared".

### Attachment storage

`media.db` holds the bytes of every attachment that has been downloaded or
sent, split into chunks so a large video is never read or written as a single
value. It is a separate database because attachments are large and would
otherwise inflate the write-ahead log of the message index.

The database is the durable copy; the media cache directory is a disposable
materialisation that exists because the desktop reads files, not blobs. Opening
an attachment whose cached file is missing restores it from the database instead
of asking WhatsApp for it again, which old media no longer allows. Attachment
bytes never travel over the RPC socket; only paths do.

Identity consolidation merges all message metadata transactionally. A tombstone
wins over either live copy; an edited revision wins over unedited history, and
preview fields stay with their body and URL. Receipt milestones do not regress.
The separate media archive is not rewritten: cache recovery checks the canonical
JID and its recorded aliases. This also repairs lookup for profiles merged by
older versions without copying or deleting durable attachment bytes. Metadata
already discarded by an older merge cannot be reconstructed by this change.

### Application integration settings

`AppSettings` is a desktop-only QML singleton for application-wide integration
preferences. The top-toolbar gear opens `WhatsAppGoSettings`, separate from
WhatsApp account settings. Provider keys never cross RPC or enter profile
databases or diagnostic context. Explicit Save validates fields and atomically
writes `AppConfigLocation/private/gif-providers.json` with owner-only file and
directory permissions on Unix; the file is not encrypted. The panel holds
temporary edits, discards them on close, and masks keys again when reopened.
`GifCatalog` makes direct, cancellable GIPHY/KLIPY requests on the desktop.
It retains response ordering, pages up to 120 results, loads at most three
bounded thumbnails concurrently, and restricts HTTPS media URLs and redirects
to provider CDN hosts. Save does not verify credentials; the first search does.
Legacy Tenor credentials remain storage-only. The expression picker lazily
creates its player and video sink only for a downloaded preview, so opening
emoji or starting the app does not initialize multimedia.

The GIF/sticker send preview captures the account, chat and reply before
selection. `RpcClient` rechecks the target before submission and holds selected
temporary GIF files through acknowledgement. GIFs use MP4 with the protocol's
GIF playback flag. The optional `gateway.StickerSender` restores an original
WebP from durable media or the stored download payload; PNG display thumbnails
are never sent as stickers. Recent/starred sticker lists reuse bounded shared
media queries and remain account-scoped.

`gif_playback` is an additive persisted message flag (the media kind remains
`video`). Upserts and phone/LID alias merges preserve it, and forwarding copies
it into `MediaRequest.GIF`. Existing rows default to false; filenames are not
used to guess whether an ordinary video was a GIF.

`InlineAnimation` lazily creates a silent MP4 player or `StickerAnimation` only
for a selected, visible animation. Main owns one selected chat ID; the picker
pauses chat animation. Offscreen delegates unload their decoder. Sticker PNGs
remain static fallbacks. Transient `sticker_source` and `sticker_animated` fields
point to the retained WebP original, including after a live download.
The optional libwebp decoder reconstructs one composited RGBA frame at a time
on a single-thread pool, copies it before returning to the UI, and uses a
single-shot timer. Generation/cancellation guards discard late frames. No
whole-animation frame cache is retained.

### Send acknowledgements

Message and image-caption editors remain plain text. `ComposerText` applies a
presentation-only syntax highlighter to emoji runs so mixed text uses the color
emoji font without introducing HTML into drafts, the clipboard, or RPC payloads.
On a space/newline key release it replaces the completed standalone ASCII
emoticon; before sending it converts remaining eligible tokens before capturing
the acknowledgement's draft text. Backtick-delimited code, escaped tokens, and
URL/path fragments stay literal. Replacement uses the editor's native input
event path as one undoable operation, preserving cursor/selection and leaving
active IME composition alone. Individual tokens are replaced rather than the
entire span between the first and last token, so unchanged mention character
formats retain their recipient identities. Existing message history is not rewritten.

The composer parks text/reply drafts until the daemon acknowledges a send.
Completion signals carry the captured profile, chat, text and quote; QML clears
only an unchanged matching draft. Clipboard images stay in the scoped preview
on failure; successful sends clean up temporary files. Rotated send copies are
discarded after completion while the original remains available for retry on
failure. Pending previews cannot be sent to a newly selected conversation.

File and voice-note sends also retain the selected reply until acknowledgement.
Their completion signal carries the original profile, conversation, and quote;
only a successful send clears a still-matching quote in that conversation's
draft. A newer reply or another conversation's draft is left alone.

An image download started by **Copy image** may finish after navigation. Its
clipboard action can complete, but the downloaded message is inserted into the
visible model only if the original account and conversation still match.
A newer image/text copy increments the clipboard-intent generation and clears
the pending image identity. Late callbacks check the generation and profile;
media-event fallback also matches the source chat. Obsolete downloads cannot
overwrite the newer clipboard content or surface their old copy error.
When the backend disconnects, queued media work is discarded and cancellation
callbacks release their slots before the active-download count is reset. This
keeps the three-download limit intact after reconnection.

### Desktop request ordering and ownership

These rules describe the [v0.1.9 reliability fixes](releases/v0.1.9.md):

- Preferences and notification settings have separate request generations and
  event revisions. A response predating an update event cannot overwrite its
  state; a completed write still clears its busy flag. Invalidating reads must
  not accidentally strand pending-write UI state.
- `RpcClient::editMessage` reports `messageEditFinished` with an editor token,
  profile, chat and message identity. The dialog stays open on rejection and
  only accepts completion for the current editing session. This is a desktop
  completion contract, not a change to the daemon's `message.edit` wire format.
- `WhatsAppDialog.chatScoped` opts confirmations into captured chat/profile
  ownership. A mismatch dismisses the popup and its acceptance guard rejects
  stale activation. Unscoped dialogs keep their normal behavior; closing a
  submitted operation does not cancel its RPC.
- Status reply state follows profile/sender/status identity, not QVariant map
  object replacement. Media-path upgrades and same-status refreshes preserve
  drafts and pending replies.
- Star events update loaded results and invalidate older read snapshots.
  Global media-library requests use a generation per request/reload and reject
  overlapping same-category append requests. Account changes invalidate and
  clear the library. These guards do not add server-wide sorting or pagination
  beyond the existing APIs.
- Failed automatic media downloads release their deduplication keys and impose
  a two-second retry cooldown. Chat callbacks check the chat-open generation;
  status cooldown keys include the account. A later request may retry; the
  cooldown does not schedule an endless background retry loop.

### Bug-report intake

The desktop's toolbar and Help actions open the WhatsAppGo GitHub issues
URL directly through the external browser opener, without invoking a backend
RPC or collecting report data. Both share one action and browser-failure notice.

`internal/bugreport` performs authenticated HTTPS submission to the fixed Jabali
endpoint with `program: "whatsappgo"`. The service exposes safe environment
disclosure, a credential-availability boolean, and submission through the local
RPC for separately configured tooling, not the desktop report actions.
Keys are runtime configuration, never RPC arguments or bundled credentials.
The submitter enforces `Retry-After` and tracker-error
backoff on manual retries, but never retries a POST automatically.
See [bug reporting](BUG_REPORTING.md)
for the request/response contract and operational setup.

### Shared contacts and places

A shared contact travels as a vCard. The name and the first telephone number
are read out of it once, when the message is stored, so the interface never
parses a card while drawing. A shared place carries its coordinates and a small
map picture, which is cached like any other preview. Neither is fetched from a
network service.

### Link previews

A link preview belongs to the message: the sending client resolves the page and
puts the title, description, and a small picture inside the protobuf. WhatsAppGo
stores those fields and renders them as they arrived.

Some senders embed a picture too small to fill a card. For those, and only for
cards an open conversation is already showing, the daemon fetches the linked
page once to read its preview image. That request is made from this computer,
so opening such a conversation does tell those sites the link was looked at.
Each message is attempted once per session, and a card whose stored picture is
already wide enough is never refetched.

History synchronisation strips the inline picture from media messages, and does
not carry link previews for messages that predate this feature. Pictures are
therefore fetched on demand for the messages the conversation is showing, a few
at a time, which bounds the work to what is on screen.

The daemon also collects attachments on its own, newest first, one file every
1.5 seconds, up to 400 files per connection and skipping anything above 50 MB.
Its position is written to the message database, so a restart resumes rather
than starting again, and everything it fetches goes into `media.db`.
Attachments WhatsApp no longer serves are skipped without stopping the scan.

### Media previews

Media messages carry a small inline preview. It is written to
`media/thumbnails/` when the message is stored, so photos, videos, and stickers
render without downloading the full file. Messages stored before this existed
are filled in once per profile from the message payloads already in
`messages.db`, without contacting WhatsApp. Downloading a file never discards
the preview, and re-receiving a message never discards an already downloaded
file.

### Phone-number and LID aliases

WhatsApp may identify the same person as both `number@s.whatsapp.net` and a
privacy-preserving `id@lid`. Directory synchronization reads whatsmeow's
verified mapping and transactionally consolidates both chat rows under the LID.
Message IDs are deduplicated, media/reactions move with their messages, the old
JID remains an alias, and future reads/writes resolve to the canonical chat.
Names alone are never used as proof that two people are the same.

## RPC protocol

Each packet is a UTF-8 JSON object followed by `\n`. Requests and responses use
protocol version 1:

```json
{"version":1,"id":"42","method":"message.send","params":{"chat_jid":"123@lid","text":"hello"}}
{"version":1,"id":"42","result":{"id":"message-id","status":"sent"}}
```

Events omit `id`:

```json
{"version":1,"event":"message.upsert","data":{"id":"message-id","chat_jid":"123@lid"}}
```

Operation groups include connection/pairing, account logout, chat/message
listing and search, history requests, media downloads, sending/editing/deleting
messages, reactions, contact resolution, avatars, receipts, typing, statuses,
calls, channels, and communities. Parameter decoding rejects unknown fields.

The socket exists only below the user's runtime directory and is never exposed
on the network. `whatsappctl` is a thin same-user client for this protocol; it
does not start a daemon. `rpc.discover` provides the installed method and event
catalogue. See [Command-line and bot API](API.md).

## Group information and membership

`chat.info` stays a cheap local metadata/shared-content query. Opening group info
also requests `group.info` through the optional `gateway.GroupManager` interface.
The WhatsApp implementation obtains live metadata from whatsmeow, resolves
participant PN/LID aliases against cached contacts/chats, and derives permissions
from the current user's membership and group member-add mode. Member photos are
requested through the existing avatar cache only for displayed rows.

`RpcClient` keeps group state separate from chat history, coalesces in-flight
metadata refreshes, and ignores responses from earlier chat/profile activations.
`group.updated` refreshes previously opened group metadata without refetching it
on every ordinary message update. Group actions have an in-flight guard and
report errors independently of their confirmation dialog's lifetime.

Name/description writes use a separate optional `gateway.GroupInfoEditor`
capability and `group.set_info`, leaving existing offline/member-only gateways
compatible. Shared model validation counts Unicode code points. The live adapter
rechecks current membership, admin-only metadata permissions, and expected old
field text before calling whatsmeow's name/topic setter. Community/announcement
and suspended groups are excluded. No message history or member avatars are
loaded for editing. A saved name updates the local chat title and change events.

`GroupMetadataDialog` opens from compact inline pencils next to copyable group
names/descriptions. The draft belongs to a captured account/chat; request tokens
and group generations isolate late acknowledgments and pre-save refreshes.
Explicit Save closes only after success, errors retain the draft, and Escape
returns to group info with focus restored. Other group actions keep their own
existing confirmation flow.

`GroupInfoContent` renders a bounded eight-member preview; `GroupActions` uses a
virtualized full list for search and member selection. Permission-aware controls
are backed by fresh server-side authorization checks. Per-participant failures
are surfaced even when a batch partially succeeds. Leaving emits a group update
but never removes locally retained history. Advanced chat privacy, member tags,
and reporting remain explicitly unsupported rather than fabricated controls.

## Updates

The daemon asks GitHub for the newest published release every three hours and
publishes `update.available` when the version on offer changes. A draft or a
pre-release is not an answer: the release workflow publishes a draft first, and
a person decides when it becomes a release.

The comparison is against the version the binary was stamped with at build
time, by `scripts/version.sh` through `-X main.version`. A build with no stamp
calls itself `dev` and is never behind a release, so a working copy is never
told to update - and an unstamped release build would silently have no update
check at all, which is why `scripts/version-test.sh` runs in CI.

`update.download` fetches the artifact for the running platform. Nothing about
the answer is trusted: https only, every host in the redirect chain has to be on
an allow list, the size is capped, the name on disk is the one this build
expects rather than the one the server offered, and the contents have to match
the release's own `SHA256SUMS`. A file that fails any of that is deleted.

Installing is the desktop's job, because the daemon has no idea what the window
is running from and cannot restart it. An AppImage is replaced in place - copy
beside it, then a rename, so either the old version or the new one is there -
and the window relaunches itself. Windows starts the installer and closes.
macOS opens the disk image for the reader to drag across, which is as far as an
unsigned bundle can go.

## App-state collections

Call history lives in WhatsApp's regular app-state collection and conversation
settings in the low-priority one. Each is fetched once in full for installations
that linked before those were imported, and the fact that it was done is written
to `messages.db` metadata so it is not repeated.

Two failures are expected and neither is shown to the reader. A missing
app-state key means WhatsApp has not shared it yet, so the backfill is deferred
rather than retried. A mismatching LTHash means the collection the server sent
does not verify against the hash it signed; a retry produces the same bytes, so
the daemon sends the phone the peer message whatsmeow builds for this case,
asking for an unencrypted copy of that collection. The reply is applied by
whatsmeow and arrives as `AppStateSyncComplete`, which is where the backfill is
marked done. The request is remembered so a phone that has not answered is asked
at most once a day.

Protocol messages that are not revocations - ephemeral-timer changes, history
notifications, key shares - are machinery rather than conversation and are not
stored. Ones written by earlier versions are left in place and filtered out of a
conversation.

## Event flow

Incoming message:

```text
WhatsApp → whatsmeow event → normalize identity/model → SQLite transaction
         → media cache/notification event → RPC → tray/QML message bubble
```

Outgoing message:

```text
QML composer → RPC request → whatsmeow send → SQLite transaction
             → RPC result/event → QML model and receipt updates
```

### Conversation model

`MessageListModel` stores a conversation chronologically and exposes it
newest-first, so a bottom-to-top view keeps row zero at the composer while older
history grows away from the reader. It also marks the message that opens each
calendar day, which is what the date pills are drawn from; a page of older
history moves that mark rather than leaving two. Recalculation is limited to
the changed interval and its next dated neighbour; body/receipt/media changes
that keep the same date do not rescan the conversation. Undated rows preserve
the preceding valid date when calculating the next boundary.

The newest `messages.list` page includes an unread-count/first-message-ID
snapshot before the desktop acknowledges the chat. SQLite reads this metadata
and the page in one transaction. The boundary is the oldest of the latest N
incoming content messages, where N is the account's unread total (history need
not have individual receipt metadata). If that many messages are not stored
locally, the boundary is unknown rather than guessed. A boundary outside the
newest page appears when its older page is loaded.

`MessageListModel` keeps this divider as presentation state, not in cached
message payloads. Only the boundary row exposes `unread_separator_count` to
QML. New incoming IDs extend the batch; receipt, reaction, and media updates
do not count again. Opening another chat/account or reselecting the chat resets
the batch, and a new outgoing message clears it. Events arriving during the
initial page request are replayed after its snapshot, and a generation guard
rejects stale responses from earlier chat activations.

`Main.qml` binds `RpcClient::conversationActive` to the visible, foreground,
non-minimized Chats section, excluding photo/video/status viewers. Initial-page
and live incoming read receipts defer while this is false; returning to the
conversation acknowledges its loaded incoming messages. This is window-level
eligibility, not per-bubble visibility detection. Standalone RPC clients retain
explicit-open behavior by default, so other consumers must provide their own
visibility binding if needed. Opening a different chat or switching accounts
resets deferred-page state.

`RpcClient::chatOpened` signals explicit conversation activation, including
reselecting the current chat. It is distinct from `selectedChatChanged`, which
also reports title/avatar refreshes and must not interrupt a reader's position.
Activation and text-send actions re-enable tail following; explicit quoted or
search-message targets take precedence over the normal opening position. A
coalesced one-shot timer flushes pending `ListView` layout and positions at row
zero's bottom edge. Model resets (including same-count cached-page refreshes)
reschedule this only while following. Wheel/drag input releases following and
cancels queued positioning; there is no continuous content-height scroll loop.

## Memory and rendering

- The conversation is a `QAbstractListModel`. Reporting insertions precisely
  keeps the viewport anchored when an older page is prepended, and stops every
  delegate from being rebuilt whenever one receipt arrives.
- One shared media player serves the whole window, created on first use so the
  multimedia backend is not started by an application that never plays anything.
- QML `ListView` reuses chat and message delegates.
- Message actions and reaction popups are created on first interaction, not
  for every idle bubble. Pooled/reassigned delegates release them. Audio
  controls and waveform bars are created only for audio messages.
- Inline image/link previews limit decoded resolution to the displayed size
  multiplied by screen pixel density. Original media files remain unchanged;
  the full-screen viewer still uses the originals for zooming.
- Sidebar refreshes are coalesced over 50 ms, with at most one snapshot in
  flight and one follow-up if more changes arrive. Continuous traffic does not
  restart the delay indefinitely. Unchanged selected-chat metadata emits no
  redundant header update.
- The reopening cache retains only the latest 50 messages per conversation,
  up to 12 conversations and a combined 2 MiB of serialized message payloads
  (a payload budget, not an exact heap-size limit). Older pages remain in
  SQLite. If the refreshed page has the same IDs/order, update its rows in
  place instead of rebuilding all cached bubbles a second time.
- SQLite returns bounded pages rather than entire conversations.
- Media is file-backed and downloaded outside the UI process.
- Qt Quick uses the software backend by default to avoid unreliable GLX/EGL
  drivers; `QT_QUICK_BACKEND=rhi` opts into hardware rendering.
- Bubble tails are drawn directly without rotated negative-z scene nodes,
  avoiding scene-graph artifacts on software/hybrid-GPU systems.
