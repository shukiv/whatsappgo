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

### Backend

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

## Memory and rendering

- The conversation is a `QAbstractListModel`. Reporting insertions precisely
  keeps the viewport anchored when an older page is prepended, and stops every
  delegate from being rebuilt whenever one receipt arrives.
- One shared media player serves the whole window, created on first use so the
  multimedia backend is not started by an application that never plays anything.
- QML `ListView` reuses chat and message delegates.
- SQLite returns bounded pages rather than entire conversations.
- Media is file-backed and downloaded outside the UI process.
- Qt Quick uses the software backend by default to avoid unreliable GLX/EGL
  drivers; `QT_QUICK_BACKEND=rhi` opts into hardware rendering.
- Bubble tails are drawn directly without rotated negative-z scene nodes,
  avoiding scene-graph artifacts on software/hybrid-GPU systems.
