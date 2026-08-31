# Security and privacy

WhatsAppGo is an unofficial client built on WhatsApp's linked-device protocol.
It is not reviewed, endorsed, or supported by Meta or WhatsApp. Protocol
changes may interrupt service, and using an unofficial client may carry account
risk under WhatsApp's terms.

Messages are end-to-end encrypted in transit by whatsmeow. Decrypted message
history and downloaded media are stored locally for fast, low-memory access.
The current preview protects those files with Unix user permissions, not with a
second application-level encryption layer. Anyone who can read the user's
account or an unlocked disk can read the local cache.

Before reporting a vulnerability, avoid attaching device databases, QR payloads,
pairing codes, message contents, or logs containing JIDs. Rotate the linked
device from the official WhatsApp application if credentials may be exposed.
