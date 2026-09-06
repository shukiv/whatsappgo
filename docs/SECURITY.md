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

## Bug-report intake

Reports are sent only on explicit submission to `bugs.jabali-panel.com`, program
`whatsappgo`. The displayed technical environment accompanies the user's text;
private logs, media, account names, and chat identifiers are not automatically
collected. Do not put secrets in the description. The intake redacts known
secret patterns, but that is not a guarantee that all private information will
be removed.

An operator-issued key is read by the daemon from `WHATSAPPGO_BUGREPORT_TOKEN`
or `WHATSAPPGO_BUGREPORT_TOKEN_FILE`; prefer an owner-only file outside the
repository. Authentication is sent as a Bearer header over HTTPS. Redirects are
refused, keys are never returned over RPC, and arbitrary server error bodies are
not echoed into the UI. No shared token is included in the app. See
[key setup and retry behavior](BUG_REPORTING.md).
