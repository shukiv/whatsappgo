# Unreleased changes

These changes are committed after the `v0.1.14` tag and are not in any
published artifact. Build from source containing them and restart that build to
use them; a rebuild does not replace an already running process.

## Fixed

- A contact somebody shares now arrives with its number. WhatsApp's phone
  clients write the vCard's number as a grouped property, which the reader
  skipped, so the card had a name and nothing else on it. A conversation
  re-synchronised after this change fills the number in.

- Opening the window no longer draws "info query returned status 400:
  bad-request" in red across it. A profile picture was being asked for by
  phone - a status sender arrives with its device attached - and WhatsApp
  refuses an address it cannot hold a picture for. The picture is now asked for
  by person, and a row that cannot show a face quietly shows initials.

## Changed

- A shared contact is drawn the way WhatsApp Web draws one: the name beside
  the picture, and under a rule two actions of equal width, Message and Save
  contact. Saving is new in the window; the daemon could already do it.

- Opening a conversation puts the cursor in the message box, the way WhatsApp
  Web does. A row clicked in the list used to leave the cursor where it was, so
  a letter typed straight after the click went nowhere. Channels are left
  alone: they have no box to write in.

## Known issues

- `desktop-message-interactions` fails intermittently when the machine is
  busy (`whatsappgo-jy9`). It reads a popup 20ms after opening it, before the
  animation has finished.

- An AppImage run waits 25 seconds for a window when the session bus it
  inherits has no desktop portal on it (`whatsappgo-8ug`). Starting it with
  `QT_QPA_PLATFORMTHEME=gtk3` avoids the wait.
