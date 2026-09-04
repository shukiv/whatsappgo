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

All parameter objects reject unknown fields.

| Method | Parameters | Result/action |
| --- | --- | --- |
| `rpc.discover` | `{}` | API metadata |
| `status.get` | `{}` | Connection status |
| `connection.connect` / `connection.disconnect` | `{}` | Connect or disconnect this linked device |
| `pairing.start` | `{}` | Start QR pairing and emit pairing events |
| `pairing.phone` | `phone` | Return a phone-pairing code |
| `account.logout` | `{}` | Unlink the profile; destructive |
| `chats.list` | `limit`, `offset`, `query`, `archived` | Chat array |
| `chats.archived_count` | `{}` | Archived count |
| `chats.unread_count` | `{}` | Exact unread-message total for the profile's visible chats |
| `chat.info` | `chat_jid` | Contact/chat metadata, phone alias, exact shared-content counts, and a six-item preview |
| `chat.shared` | `chat_jid`, `category`, `offset`, `limit` | Page local `media`, `documents`, or `links` for one chat; `all` is also accepted |
| `chat.pin` / `chat.mute` / `chat.archive` / `chat.set_read` | `chat_jid`, `value` | Change synchronized chat state |
| `chat.read` | `chat_jid`, `sender_jid`, `message_ids`, `timestamp` | Send receipts and clear local unread state |
| `chat.typing` | `chat_jid`, `typing` | Set composing/paused presence |
| `chat.avatar` | `chat_jid` | Fetch/cache avatar and return its path |
| `statuses.list` | `{}` | Active (last 24 hours) status stories grouped by sender; each group contains resolved identity fields and chronologically ordered `items` |
| `calls.list` | `{}` | Locally synchronized call records |
| `channels.list` | `{}` | Followed channels |
| `communities.list` | `{}` | Joined communities |
| `messages.list` | `chat_jid`, `before`, `limit` | Message page with pagination cursor |
| `messages.search` | `query`, `limit` | Local text-search results |
| `link.preview` | `text` | Open Graph metadata and thumbnail bytes |
| `history.request` / `history.refresh` | `chat_jid`, `limit` | Ask WhatsApp for older/recent linked-device history |
| `message.download` | `chat_jid`, `message_id` | Download/cache media and return its local path |
| `message.send` | `chat_jid`, `text`, `reply_to`, `reply_chat_jid`, `link_preview` | Sent message; `reply_chat_jid` identifies a quoted message stored in a different chat |
| `message.send_media` | `chat_jid`, `path`, `caption`, `reply_to`, `voice` | Sent media message; path must be local to WhatsAppGo |
| `message.react` | `chat_jid`, `message_id`, `sender_jid`, `emoji` | Add reaction; empty emoji removes it |
| `message.pin` | `chat_jid`, `message_id`, `sender_jid`, `duration_seconds` | Pin for 86400, 604800, or 2592000 seconds |
| `message.unpin` | `chat_jid`, `message_id`, `sender_jid` | Remove the chat's pinned message |
| `message.star` | `chat_jid`, `message_id`, `sender_jid`, `from_me`, `starred` | Star or unstar for the whole account |
| `messages.starred` | `limit` | Starred messages across every chat, newest first |
| `message.forward` | `chat_jid`, `message_id`, `to_chat_jid` | Re-send into another chat, marked as forwarded |
| `message.edit` | `chat_jid`, `message_id`, `text` | Edit eligible sent text |
| `message.delete` | `chat_jid`, `message_id`, `sender_jid` | Delete an eligible message for everyone |
| `contact.resolve` | `phone` | Validate number and return/create its chat |
| `contact.save` | `phone`, `name` | Resolve number and save a local chat label |

The `params_example` values returned by `rpc.discover` are canonical examples
for the installed version.

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
