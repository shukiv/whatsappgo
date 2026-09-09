# Command-line and bot API

`whatsappctl` is the supported automation interface for WhatsAppGo. It talks to
the backend already owned by the desktop application; it never starts a second
WhatsApp connection or a separate service. Keep WhatsAppGo running and select
the account profile once before controlling it.

The client prints one JSON value to standard output and machine-readable errors
to standard error. Successful commands exit 0 and failures exit non-zero.

> WhatsAppGo is an unofficial client. Automate only accounts and conversations
> you are authorized to use. Do not send spam, bypass consent, or build reply
> loops. WhatsApp can restrict accounts that behave abusively.

## Start here

```bash
# The default account
whatsappctl status
whatsappctl --pretty chats --limit 20
whatsappctl messages --chat '15551234567@s.whatsapp.net' --limit 50
whatsappctl search --limit 20 invoice

# A named account tab
whatsappctl --profile work status

# Send by international number or by the JID returned from `chats`
whatsappctl send --to +15551234567 'Hello from my bot'
printf 'First line\nSecond line\n' |
  whatsappctl send --to '15551234567@s.whatsapp.net'

# Files, pictures, audio, and voice notes
whatsappctl send --to +15551234567 --file ./report.pdf --caption 'Report'
whatsappctl send --to +15551234567 --file ./note.ogg --voice

# Validate a WhatsApp number and open its local chat
whatsappctl contact --phone +15551234567

# Save a name in WhatsAppGo's local chat database
whatsappctl contact --phone +15551234567 --name 'Alice'
```

`contact --name` cannot edit the phone's address book. The linked-device
protocol does not offer a supported address-book write operation. It saves a
local WhatsAppGo label so a bot can identify the conversation without inventing
an unsupported phone-side capability.

## Share one contact card

`contacts.shareable` searches locally stored direct contacts, including archived
chats and known phone aliases. It returns at most 100 entries; an empty query
lists the first page. Opaque identities without a phone mapping are omitted.

```bash
whatsappctl call contacts.shareable '{"query":"Alice"}'
whatsappctl call message.send_contact '{"chat_jid":"15559876543@s.whatsapp.net","reply_to":"","contact":{"name":"Alice","phone":"+15551234567"}}'
```

The second call sends immediately. Review the destination and card first.
Only the supplied name (1–100 characters, single line) and international phone
number (`+`, 7–15 digits; spaces, parentheses, dots and dashes are normalized)
are encoded into a vCard contact message. Direct and group conversations are
supported, not channels or status broadcasts. A successful result is stored as
`kind: "contact"` and emits `message.upsert`. This method does not modify either
party's address book. Multiple-contact cards are not supported yet.

## Live events

`events` is a continuous JSON Lines stream. Each line is independently valid
JSON, making it suitable for `jq`, Python, Go, or a process supervisor.

```bash
whatsappctl events
whatsappctl --profile work events --event message.upsert --event message.receipt

whatsappctl events --event message.upsert |
  jq -c 'select(.data.from_me == false) |
         {chat: .data.chat_jid, id: .data.id, text: .data.body}'
```

Stop it with `Ctrl+C`. A production bot should ignore its own messages, keep a
deduplication set keyed by chat JID and message ID, rate-limit replies, and
persist its last processed IDs before taking an external action.

## Raw calls

Every backend operation is available through `call`. Parameters can be a
literal JSON object, `-` for standard input, or `@path` for a JSON file.

```bash
whatsappctl call chat.typing '{"chat_jid":"15551234567@s.whatsapp.net","typing":true}'
whatsappctl --pretty call chat.info '{"chat_jid":"15551234567@s.whatsapp.net"}'
whatsappctl --pretty call chat.shared '{"chat_jid":"15551234567@s.whatsapp.net","category":"links","offset":0,"limit":60}'
whatsappctl call chat.set_read '{"chat_jid":"15551234567@s.whatsapp.net","value":true}'
whatsappctl call message.react '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","sender_jid":"","emoji":"👍"}'
whatsappctl call message.pin '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","sender_jid":"","duration_seconds":604800}'
whatsappctl call message.unpin '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","sender_jid":""}'
whatsappctl call message.star '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","sender_jid":"","from_me":true,"starred":true}'
whatsappctl --pretty call messages.starred '{"limit":50}'
whatsappctl call message.forward '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","to_chat_jid":"15559876543@s.whatsapp.net"}'
whatsappctl call message.edit '{"chat_jid":"15551234567@s.whatsapp.net","message_id":"ID","text":"Corrected"}'
whatsappctl call message.delete @delete.json
printf '{"query":"contract","limit":25}' | whatsappctl call messages.search -
```

Use discovery instead of hardcoding an undocumented list:

```bash
whatsappctl --pretty discover
whatsappctl discover | jq -r '.methods[].name'
```

Discovery reports the protocol version, method names, whether they mutate
state, example parameters, and every event name supported by this build.

## Convenient commands

| Command | Purpose |
| --- | --- |
| `status` | Connection, login, user JID, and last state change |
| `discover` | Machine-readable API and event catalogue |
| `chats` | List/search chats; supports `--limit`, `--offset`, `--query`, `--archived` |
| `messages` | Page a chat with `--chat`, `--before`, and `--limit` |
| `search` | Full-text search of locally stored message bodies |
| `contact` | Resolve an international number and optionally save a local label |
| `send` | Send text, stdin, files, images, audio, voice notes, replies, and link previews |
| `download` | Download one attachment by chat JID and message ID |
| `call` | Invoke any raw API method |
| `events` | Stream live backend events as JSON Lines |

Global flags must precede the command:

```text
--profile NAME       account tab; default is "default"
--socket PATH        explicit Unix socket, primarily for development
--timeout DURATION   call/dial deadline; default is 30s
--pretty             indent one-shot JSON output
--version            print the client version
```

Text sends automatically resolve an Open Graph preview. Add `--no-preview` to
skip that lookup. `--reply-to ID` replies to a message. With `--file`, use
`--caption` for text and `--voice` only for an audio file.

## Method reference

Local per-profile notification preferences:

- `notifications.get` returns booleans: `messages`, `groups`, `calls`, `previews`,
  `sounds` (incoming master), `messages_sound`, `groups_sound`, `calls_sound`, `statuses_sound`
  default true; `statuses`, `messages_reactions`, `groups_reactions`, `outgoing_sound`, `security` default
  false. Existing master-sound preferences are preserved.
- `notifications.set` takes `{"name":"previews","value":false}` and returns
  the saved preferences. Only the names above and a boolean value are valid.
- `notifications.updated` broadcasts the saved values. These settings affect
  desktop alerts, not WhatsApp privacy settings or message/history delivery.
  `sounds` is honored by the Linux notifier; other desktops use OS controls.
- Opt-in `statuses` alerts cover new incoming text/photo/video/audio status
  messages under five minutes old, not history, edits, replays, own updates or
  likes/mentions. Muted senders stay quiet; `previews` and the sound switches
  apply. Their `notification.received` click target is `status@broadcast`, which
  the desktop opens as the Status page without selecting a broadcast chat.
- `notifications.test_sound` takes `{"kind":"incoming"}` or `"outgoing"`.
  It plays the Linux desktop sound without sending a WhatsApp message and
  reports missing audio dependencies/output failures.
- `preferences.get` / `preferences.set` use the same name/boolean contract for
  `download_image`, `download_video`, `download_audio`, `download_document`,
  `download_sticker` (default true) and `disable_link_previews` (default false).
  Updates emit `preferences.updated`. Manual downloads bypass auto-download
  preferences; existing files are preserved and in-flight requests may finish.
- `profile.set_name` takes `{"name":"New name"}` (one line, 1–25 Unicode
  characters), sends a WhatsApp app-state change and updates connection status.
- `profile.get` returns the connected account's `name`, `about`, and locally
  cached `avatar_path`, refreshing the photo with the server's picture ID.
  Missing/private photos do not prevent About readback.
- `profile.set_photo` takes `{"path":"/absolute/prepared.jpg"}` to change the
  connected account's own photo, or `{"remove":true}` with no path to remove it.
  The upload must be a complete 640×640 JPEG under 2 MiB. A recipient/group JID
  is not accepted. Success returns `{"ok":true}` after the server acknowledges
  and emits `profile.changed` with `{"photo":true}`. The desktop prepares a
  private metadata-free copy and requires a preview/Save or removal confirmation;
  API callers must supply their own prepared file and explicit consent.
- `status.audience` returns the default broadcast audience: `type` (`contacts`,
  `blacklist`, or `whitelist`) and `jids`. This is read-only; edit exceptions on
  the phone. It is separate from About visibility.
- `privacy.default_timer.set` takes `{"seconds":86400}`. Only `0` (off), `86400`,
  `604800`, or `7776000` are accepted. Success returns `{"ok":true,"seconds":...}`.
  This changes the WhatsApp account default for **new chats**, not existing chat
  timers. The library has no current-default getter; the acknowledgement must
  not be treated as a current-value read. WhatsAppGo retains local history even
  when disappearing messages are enabled.
- `privacy.get` exposes `about`; `privacy.set` accepts `about` for About
  visibility. The old `status` field/name remains a compatibility alias for
  **About**, not the status-broadcast audience. `contact_blacklist` can be read
  but cannot be selected without an exception-list editor; use the phone.
- Enabling `security` produces recent security-code-change alerts (silent on Linux),
  respecting chat mute and suppressing repeated events for five minutes.
- `privacy.set` additionally accepts `call_add` (`all`, `known`), `messages`
  (`all`, `contacts`) and `defense` (`on_standard`, `off`). These are account
  settings, unlike local notification and download preferences. Unsupported
  values from a server remain unknown rather than becoming a default toggle.

All parameter objects reject unknown fields.

Media message results retain `kind: "video"` for MP4 GIFs and carry
`gif_playback: true` when the protocol marks them as looping animations. The
flag survives persistence, pagination, search and forwarding. It defaults to
false for older records. Sticker display results can additionally include
`sticker_source` (a retained local WebP path) and `sticker_animated: true`;
`media_path` remains the static PNG fallback for clients without WebP support.
The sticker fields are display metadata, not upload paths accepted from a peer.

| Method | Parameters | Result/action |
| --- | --- | --- |
| `rpc.discover` | `{}` | API metadata |
| `status.get` | `{}` | Connection status |
| `bugreport.environment` | `{}` | Safe environment `fields`, `rendered` text, fixed `program: "whatsappgo"`, intake `endpoint`, public `public_url`, and `authenticated_available` (local key present and readable, not server-validated) |
| `bugreport.submit` | `subject`, `body` | Submit to Jabali Bugs Intake and return `{ "url": "..." }`; requires a daemon-side intake key. See [bug reporting](BUG_REPORTING.md) |
| `update.status` | `{}` | The installed version, any newer release, and whether this build can install one |
| `update.check` | `{}` | Ask GitHub now instead of waiting for the next three-hourly look |
| `update.download` | `{}` | Start downloading this platform's artifact; reports itself with `update.progress`, `update.ready` and `update.failed` |
| `connection.connect` / `connection.disconnect` | `{}` | Connect or disconnect this linked device |
| `pairing.start` | `{}` | Start QR pairing and emit pairing events |
| `pairing.phone` | `phone` | Return a phone-pairing code |
| `account.logout` | `{}` | Unlink the profile; destructive |
| `chats.list` | `limit`, `offset`, `query`, `archived` | Chat array |
| `chats.archived_count` | `{}` | Archived count |
| `chats.unread_count` | `{}` | Exact unread-message total for the profile's visible chats |
| `chat.info` | `chat_jid` | Contact/chat metadata, phone alias, exact shared-content counts, and a six-item preview |
| `group.info` | `chat_jid` | Live group description, creation metadata, participants, roles and permissions; requires a connected WhatsApp session |
| `group.invite_link` | `chat_jid`, optional `reset` (default false) | `{ "link": "..." }`; permitted members can fetch, only admins can reset/revoke the old link |
| `group.members` | `chat_jid`, `action`, `participants` | Add/remove/promote/demote 1–100 user JIDs; checks current membership and permissions; partial failures return an error |
| `group.leave` | `chat_jid` | Leave the group; preserves local messages and media |
| `chat.shared` | `chat_jid`, `category`, `offset`, `limit` | Page local `media`, `documents`, or `links` for one chat; `all` is also accepted |
| `chat.pin` / `chat.mute` / `chat.archive` / `chat.set_read` | `chat_jid`, `value` | Change synchronized chat state |
| `chat.read` | `chat_jid`, `sender_jid`, `message_ids`, `timestamp` | Send receipts and clear local unread state |
| `chat.typing` | `chat_jid`, `typing` | Set composing/paused presence |
| `chat.avatar` | `chat_jid` | Fetch/cache avatar and return its path |
| `statuses.list` | `{}` | Active (last 24 hours) status stories grouped by sender; each group contains resolved identity fields and chronologically ordered `items` |
| `calls.list` | `{}` | Locally synchronized call records |
| `channels.list` | `{}` | Followed channels |
| `communities.list` | `{}` | Joined communities |
| `messages.list` | `chat_jid`, `before`, `before_id`, `limit` | Message page with pagination cursor; newest page also supplies `unread_count` and `first_unread_id` when available, for a stable unread divider |
| `messages.search` | `query`, `limit` | Local text-search results |
| `link.preview` | `text` | Open Graph metadata and thumbnail bytes |
| `history.request` / `history.refresh` | `chat_jid`, `limit` | Ask WhatsApp for older/recent linked-device history |
| `message.get` | `chat_jid`, `message_id` | One stored message with its reactions and quoted line, for applying a small change without reloading a page |
| `message.download` | `chat_jid`, `message_id` | Download/cache media and return its local path |
| `message.send` | `chat_jid`, `text`, `reply_to`, `reply_chat_jid`, `link_preview` | Sent message; `reply_chat_jid` identifies a quoted message stored in a different chat |
| `message.send_media` | `chat_jid`, `path`, `caption`, `reply_to`, `voice`, `document`, `gif`, `sticker` | Sent media message; path must be local. `document` preserves file semantics, `gif` sends a looping MP4 (up to 16 MiB), `sticker` sends WebP (up to 1 MiB, at most 512×512). These flags and `voice` are mutually exclusive and default to false. Stickers have no caption. |
| `sticker.send` | `chat_jid`, `message_id`, `to_chat_jid`, optional `reply_to` | Reuse a non-deleted sticker from this account's history. Restores/downloads the original WebP, not its PNG display thumbnail; does not add a forwarded label. |
| `message.react` | `chat_jid`, `message_id`, `sender_jid`, `emoji` | Add reaction; empty emoji removes it |
| `message.pin` | `chat_jid`, `message_id`, `sender_jid`, `duration_seconds` | Pin for 86400, 604800, or 2592000 seconds |
| `message.unpin` | `chat_jid`, `message_id`, `sender_jid` | Remove the chat's pinned message |
| `message.star` | `chat_jid`, `message_id`, `sender_jid`, `from_me`, `starred` | Star or unstar for the whole account |
| `messages.starred` | `limit`, optional `chat_jid` | Starred messages newest first; omit `chat_jid` or leave it empty for all chats. The chat filter resolves aliases and is applied before the limit. |
| `message.forward` | `chat_jid`, `message_id`, `to_chat_jid` | Re-send into another chat, marked as forwarded. Media must be downloaded first; a missing attachment returns an error, never a caption-only text forward. |
| `message.edit` | `chat_jid`, `message_id`, `text` | Edit eligible sent text |
| `message.delete` | `chat_jid`, `message_id`, `sender_jid` | Delete an eligible message for everyone |
| `contact.resolve` | `phone` | Validate number and return/create its chat |
| `contact.save` | `phone`, `name` | Resolve number and save a local chat label |

The `params_example` values returned by `rpc.discover` are canonical examples
for the installed version.

`group.info` returns `jid`, `name`, `description`, `created_at` (Unix milliseconds,
zero if unknown), `creator`, `participant_count`, `participants`, `is_member`,
`can_add`, `can_invite`, and `can_manage`. Each member has `jid`, `aliases`, `name`,
`phone` (without `+`) and `avatar_path` when available, `is_self`, `is_admin`,
and `is_owner`. LID and phone aliases refer to the same person. An uncached avatar
can be requested using `chat.avatar`; group info does not download every photo.

`group.members.action` is one of `add`, `remove`, `promote`, or `demote`.
Removing/changing your own role or the group owner's role is rejected; use
`group.leave` to exit yourself. A partial server failure may already have changed
some members: refresh `group.info` before retrying. Successful membership changes
and upstream group updates emit `group.updated` with `{ "jid": "...@g.us" }`.
A local leave additionally supplies `left: "true"`. Invitation resets and group
membership writes affect the real WhatsApp group, not just this device.

To reply to a status, send the text to the status owner's direct JID while
quoting the status message from `status@broadcast`:

```bash
whatsappctl send --to alice@lid --text "Great photo" \
  --reply-to STATUS_MESSAGE_ID --reply-chat status@broadcast
```

## Socket protocol

Programs that cannot invoke a subprocess may connect directly to the same Unix
socket. Packets are UTF-8, newline-delimited JSON. Requests and responses use
protocol version 1; events can be interleaved at any time.

```json
{"version":1,"id":"bot-42","method":"message.send","params":{"chat_jid":"15551234567@s.whatsapp.net","text":"hello","reply_to":"","link_preview":{}}}
{"version":1,"id":"bot-42","result":{"id":"MESSAGE_ID","status":"sent"}}
{"version":1,"event":"message.upsert","data":{"id":"MESSAGE_ID","chat_jid":"15551234567@s.whatsapp.net"}}
```

Match responses by `id`; do not assume the next packet is the response. The
default profile socket is `$XDG_RUNTIME_DIR/whatsappgo/whatsappd.sock`. Named
profiles use `whatsappd-<profile>.sock` in the same directory.

## Security and deployment

The runtime directory is mode `0700` and the socket is mode `0600`, so only the
logged-in Unix user can control the account. The API intentionally has no TCP
listener, HTTP server, access token, or permissive network bind. Any program
running as that Unix user can nevertheless send messages or unlink the account;
run bots with the same care as the desktop application.

For automation on another machine, prefer SSH execution:

```bash
ssh desktop.example whatsappctl --profile work status
```

Do not expose or proxy the socket to a network. With Flatpak, invoke the bundled
client inside the sandbox:

```bash
flatpak run --command=whatsappctl org.whatsappgo.Desktop status
```

The desktop application must remain open (it may be hidden in the tray), because
it owns and supervises the backend. `whatsappctl` does not create a second daemon.
