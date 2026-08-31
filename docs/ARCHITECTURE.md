# Architecture

## Overview

WhatsAppGo is one user-facing desktop application implemented by two native
processes:

```text
whatsappgo (Qt 6 / QML / Kirigami)
   │ starts, monitors, and stops
   ├── whatsappd --profile default
   ├── whatsappd --profile work
   │
   └── JSON-lines RPC over per-profile Unix sockets
          │
          ▼
       whatsmeow ── encrypted WhatsApp multi-device connection
          │
          ├── device.db      keys, sessions, app state
          ├── messages.db    chats, messages, aliases, receipts
          └── media/         downloaded media and avatars
```

The Go helper remains a separate process for fault isolation and because the
protocol implementation is Go, but it is an internal application component.
The user never starts or manages it. The desktop launches a helper when an
account socket is unavailable, keeps helpers alive while the application is
open, and terminates the helpers it owns during shutdown. Switching account
tabs starts that account's helper if necessary and leaves already selected
accounts connected until the desktop closes.

An already-running compatible helper can still be used during development,
but normal packages do not install or require systemd user units.

## Responsibilities

### Desktop

- owns the application window and backend child-process lifecycle
- renders virtualized chat/message lists and native controls
- keeps only visible/paged message data in memory
- sends typed RPC requests and consumes live events
- manages account tabs, theme preference, clipboard, and file dialogs

### Backend

- owns the WhatsApp connection, encryption sessions, and reconnection
- converts whatsmeow events into application models
- imports history and app-state records
- persists messages before emitting UI events
- extracts the preview picture embedded in each media message
- uploads/downloads media and sends desktop notifications
- exposes a small local RPC API; it does not expose HTTP or a network port

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
on the network.

## Event flow

Incoming message:

```text
WhatsApp → whatsmeow event → normalize identity/model → SQLite transaction
         → media cache/notification → RPC event → QML model → message bubble
```

Outgoing message:

```text
QML composer → RPC request → whatsmeow send → SQLite transaction
             → RPC result/event → QML model and receipt updates
```

## Memory and rendering

- QML `ListView` reuses chat and message delegates.
- SQLite returns bounded pages rather than entire conversations.
- Media is file-backed and downloaded outside the UI process.
- Qt Quick uses the software backend by default to avoid unreliable GLX/EGL
  drivers; `QT_QUICK_BACKEND=rhi` opts into hardware rendering.
- Bubble tails are drawn directly without rotated negative-z scene nodes,
  avoiding scene-graph artifacts on software/hybrid-GPU systems.
