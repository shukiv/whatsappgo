# Unreleased changes

These changes are committed after the `v0.1.15` tag and are not in any
published artifact. Build from source containing them and restart that build to
use them; a rebuild does not replace an already running process.

## Fixed

- Opening the archived chats after scrolling the chat list could show chats
  that are not archived, a different few each time, in place of the archived
  ones. The list reused rows it had set aside while scrolling, and they kept
  the chats they were showing. The archive now always shows the archived
  chats.

- A video sent from the window showed on phones as a grey box with no
  picture and no length until it was downloaded. The window now sends each
  video with a small still of its first frame, its length and its size, as
  WhatsApp's own apps do (`thumbnail`, `seconds`, `width` and `height` on
  `message.send_media`). The background service has to be this build too: an
  older one is sent the video without them, as before. Videos forwarded,
  posted as a status, sent from the GIF picker, the terminal client or the
  MCP server still go without them.

- Notifications appear when the window joins a background service that was
  started without them. The MCP server starts its service that way, and so
  does the systemd example in the API guide; a window opened while one was
  running used it, left notifications to it as it does on Linux, and nothing
  reached the screen - on GNOME, where there is no tray to fall back on, not
  even the window's own. The window now asks every service it connects to,
  for the profile it shows and for the others it watches, to present them for
  as long as it stays connected (`notifications.present`), and a service that
  nobody asks still posts none. The service has to be this build too: the MCP
  server runs the `whatsappd` installed beside it, not the one in the
  AppImage.

- On GNOME, notifications stopped appearing altogether - from WhatsAppGo and
  from every other program - until the next log-in. A second message from a
  chat replaced that chat's notification, and when the one replaced was the
  banner GNOME Shell was showing, its message tray failed with
  "this._notification is null" in the session log and showed no banner again.
  GNOME Shell 48 and 49 both do this. WhatsAppGo now hands GNOME Shell every
  message as a new notification and never replaces or closes one there;
  other notification services are unchanged.

## Changed

- On GNOME, a chat that sends several messages shows several notifications
  instead of one that updates. GNOME Shell keeps the latest ten for each
  program and drops older ones itself.

## Known issues

- Nothing yet.
