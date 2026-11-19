# Unreleased changes

These changes are committed after the `v0.1.15` tag and are not in any
published artifact. Build from source containing them and restart that build to
use them; a rebuild does not replace an already running process.

## Fixed

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

## Changed

- Nothing yet.

## Known issues

- Nothing yet.
