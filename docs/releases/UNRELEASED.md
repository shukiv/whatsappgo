# Unreleased changes

These changes are committed after the `v0.1.13` tag and are not in any
published artifact. Build from source containing them and restart that build to
use them; a rebuild does not replace an already running process.

## Fixed

- **Chat list:** a conversation holding a message that has not gone yet is
  listed, and the row says so. The list only ever showed a conversation that
  already held a message, was pinned, or was a favourite, so scheduling the
  first message to somebody left no trace outside the conversation itself —
  and nothing about a scheduled message exists for the other person, so the
  row is the only place it can show. A row with several waiting says how many.
  An unsent draft still wins the line, the way WhatsApp shows one thing. The
  row sorts by whichever is later, the last message or the most recent
  scheduling, so it appears where the reader just acted rather than at the
  bottom of the list. Scheduling the first message to somebody who has never
  been written to now makes the conversation to hold it.

- **File picker:** choosing a document, photo or audio file opens the
  desktop's own file chooser instead of a window drawn by the application. The
  AppImage shipped no Qt platform theme at all, so Qt had no native chooser to
  call and fell back to drawing one; it now carries the portal theme, which
  reaches whatever chooser the desktop provides.

## Changed

- **Long messages:** a message long enough to fill the window is shown as its
  first twelve lines, or its first 700 characters, with **Read more** under it;
  opening it offers **Read less**. Collapsed is the default, so one wall of
  text no longer pushes the conversation around it off the screen.
- **Quoted replies:** answering a long message quotes an excerpt of it rather
  than the whole thing.
- **Right-to-left rows:** a contact name in Hebrew or Arabic starts beside the
  avatar in the chat list, the conversation header and the channels list.
- **Emoji messages:** a message that is only emoji stays on one line at the
  enlarged size, and an emoji-only caption under a photo stays at the body
  size.
- **Avatars:** the full-resolution picture is requested when a contact's avatar
  is about to be drawn large.
- **Links:** opening a link no longer hands the browser this application's
  terminal, and only `http`, `https`, `mailto`, `tel` and `geo` addresses are
  followed.

## Known issues

- `desktop-resize-rendering` aborts intermittently with `QFontDatabase: Must
  construct a QGuiApplication before accessing QFontDatabase`. It is a race,
  not a regression: measured on 2026-10-04 it passed 1 run in 5 both with and
  without the changes above. Tracked as `whatsappgo-8hd`.
