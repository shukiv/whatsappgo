# Security and privacy

WhatsAppGo is an unofficial client built on WhatsApp's linked-device protocol.
It is not reviewed, endorsed, or supported by Meta or WhatsApp. Protocol
changes may interrupt service, and using an unofficial client may carry account
risk under WhatsApp's terms.

Messages are end-to-end encrypted in transit by whatsmeow. Decrypted message
history is stored in `messages.db`, and decrypted attachments, including photos,
videos, voice notes, and documents, are stored in `media.db`.
The current preview protects those files with Unix user permissions, not with a
second application-level encryption layer. Anyone who can read the user's
account or an unlocked disk can read the local cache.

View-once content is an exception: new protected envelopes and intentionally
unavailable view-once events are reduced to metadata-only timeline placeholders.
Protected captions, thumbnails and download payloads are not retained; download
and forwarding are refused. A one-time local repair checks protection flags in
payloads retained by older builds, replaces recognized rows with placeholders,
and removes their stored payloads and media references. It does not fetch media
or securely erase old cache files, attachment-database bytes or backups. Messages
whose events were discarded and left no metadata cannot be reconstructed locally.

Clicking a document card explicitly saves a separate copy to the system
Downloads folder, without opening it. Remote filenames are sanitized and
created exclusively with owner-only Unix permissions; existing files and
symlinks are not overwritten. Downloads are outside the profile's media cache
and remain until the user removes them, including after chat deletion or a
message revocation. Treat downloaded attachments as untrusted files.

Local conversation history is intentionally retained to preserve as much
available history as possible. A WhatsApp disappearing-message timer is **not**
a deletion guarantee for this app: stored messages and attachments can remain
on this computer after that timer elapses. Do not rely on the timer to erase
these local copies. This policy does not change explicit deletion actions or
the existing handling of message revocations. Protect local databases, media,
and backups accordingly. The [architecture retention policy](ARCHITECTURE.md#local-history-retention-policy)
records this deliberate difference from timer-based history expiration.

Most link previews are rendered from data the sender included, and are shown
without contacting anyone. WhatsAppGo makes outbound requests to sites named in
messages in three cases, all of which reveal this computer's public IP address
to the site:

1. When a user types or pastes a link into the composer, the page and its
   preview image are requested so the card can be reviewed before sending.
2. When an open conversation shows a card whose stored image is too small to
   display, that page is requested once to read a larger preview image. This
   happens for whichever links the reader is looking at, so opening a
   conversation can tell those sites the link was viewed.
3. Once per profile, the URLs of YouTube messages whose stored cards have no
   image are sent to YouTube's public oEmbed endpoint to repair old cards.

**Privacy → Disable link previews** blocks new requests through all three
backend paths and strips supplied previews from new message sends. Already
running requests may finish; cached/sender-supplied previews are not erased.
This local setting does not block links explicitly opened by the user or
third-party tools that independently resolve URLs before calling the RPC.

Requests are restricted to HTTP and HTTPS on ports 80 and 443, are refused if
the host resolves to a loopback, private, link-local, or otherwise non-public
address, connect to the resolved address directly so a name cannot be rebound
between the check and the connection, cap redirects and revalidate each one,
and bound both the response body and the time spent.

The desktop-owned backend accepts commands on a Unix socket below
`$XDG_RUNTIME_DIR/whatsappgo`. Its directory is mode `0700` and the socket is
mode `0600`; no TCP or HTTP listener is opened. `whatsappctl` and any other
process running as the same Unix user can use that socket to read history, send
messages, or unlink the profile. Do not expose the socket to a network, and do
not run untrusted programs under the logged-in account. See the
[command-line and bot API](API.md) for the complete control surface.

Before reporting a vulnerability, avoid attaching device databases, QR payloads,
pairing codes, message contents, or logs containing JIDs. Rotate the linked
device from the official WhatsApp application if credentials may be exposed.

## Local spelling and photo preparation

Optional composer spell checking sends words only to the installed local
`aspell` process over standard input. It uses no spelling network service or
temporary draft files, and dictionary output is not logged. URLs, addresses,
paths and inline backtick code are excluded. This is not protection against
other software running as the same user.

Standard/HD photo preparation makes an owner-only temporary JPEG and removes
that copy after the upload request completes. It honors orientation and uses a
fresh pixel image without source EXIF/text metadata. The source file is never
overwritten. **Original** quality and document sends retain original bytes and
metadata; converting a photo does not redact information visible in its pixels.

Security-code notifications are advisory, local and disabled by default. A
quiet alert is not proof of compromise or identity verification. Verify contact
security codes in the official app. Default message timers do not change the
local-history retention policy described above.

## GIF provider credentials

Inline animated stickers accept only local WebP originals, at most 1 MiB,
512×512 pixels, 600 frames and 120 seconds per loop. Decoding runs incrementally
off the UI thread with a bounded worker count and at most 50 frame advances per
second. Hidden/paused playback releases the decoder; invalid inputs report an
error and retain the existing PNG fallback. Builds without libwebp retain
static sticker display. This does not change attachment retention or send the
PNG fallback instead of the original sticker.

WhatsAppGo settings save user-supplied GIF provider keys separately from WhatsApp
profiles, in `AppConfigLocation/private/gif-providers.json`. This is **not
encrypted storage**. On Unix, the private directory is 0700 and the file is
0600; other processes running as the same user can still read it. Protect
configuration backups and never attach this file to bug reports. Clear a key
and Save to remove it from the current settings file; backups are unaffected.
Saving is local only, with no provider validation, RPC transmission, or
diagnostic collection. Searching sends the chosen provider its key, search
terms and the desktop's IP address; no chat content or recipients are sent.
API redirects are rejected, and media redirects are restricted to known
provider CDN hosts over HTTPS. Responses, image dimensions and concurrent
thumbnail downloads are bounded. Provider errors never log request URLs,
keys or response bodies. Search results/thumbnails are memory-only. A selected
MP4 is downloaded to an owner-only temporary file and held until the explicit
send finishes; successful sends enter the normal local message/media history.

## Bug-report intake

The desktop's **Report a problem** actions open only
`https://github.com/shukiv/whatsappgo/issues` in the browser, without report data,
credentials, or automatic issue creation. Users choose what to post on GitHub;
do not include private chats or secrets in public issues.

The optional intake RPC remains available for tooling. Only explicit submission
sends a report to `bugs.jabali-panel.com`, program `whatsappgo`. The technical
environment accompanies the caller's text. Private logs, media, account names, and chat
identifiers are not automatically collected. The client redacts the configured
intake key from report text, but not arbitrary secrets. The intake redacts known
secret patterns, but that is not a guarantee that all private information will
be removed.

An operator-issued key is read by the daemon from `WHATSAPPGO_BUGREPORT_TOKEN`
or `WHATSAPPGO_BUGREPORT_TOKEN_FILE`; prefer an owner-only file outside the
repository. Authentication is sent as a Bearer header over HTTPS. Redirects are
refused, keys are never returned over RPC, and arbitrary server error bodies are
not echoed into the UI. Only a validated `X-Request-ID` and HTTP status are
logged on response failures, not the report or credentials. The environment RPC
exposes only credential availability, never the key or its path. No shared token
is included in the app. See
[key setup and retry behavior](BUG_REPORTING.md).
