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

Opening and scrolling a conversation does not request arbitrary sites mentioned
in messages. Most link previews are rendered from data the sender included.
When a user types or pastes a link into the composer, WhatsAppGo requests that
page and its preview image so the card can be reviewed before sending; the site
can therefore observe the computer's public IP address.

There is one deliberately narrow historical exception. Once per profile,
WhatsAppGo sends the URLs of YouTube messages whose stored cards have no image
to YouTube's public oEmbed endpoint and downloads the returned thumbnails. This
repairs old YouTube cards in SQLite without crawling arbitrary historical
links. YouTube can observe the public IP address and requested video URL during
that pass.

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
