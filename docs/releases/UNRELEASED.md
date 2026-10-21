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

- **File picker, again:** the AppImage now also carries the GTK platform theme,
  so a desktop whose portal is missing or failing to start still gets its own
  file chooser rather than one drawn by the application. Carrying only the
  portal theme was not enough: a portal that never finishes starting leaves Qt
  with nothing native to call.

- **Sticker picker:** an animated sticker moves in the picker's grid, the way
  it already did in the preview below it and in the conversation. The grid drew
  a still image, so a wall of animated stickers looked like a wall of static
  ones. Only stickers that actually animate get a decoder; the rest keep their
  picture.
- **Animated stickers on Windows:** the Windows build now carries the libwebp
  library that decodes them, so an animated sticker moves there as it does
  elsewhere. Every published Windows ZIP and installer was built without it and
  told the reader animated stickers were unavailable; Linux and macOS have
  insisted on the library since `0df4db9`, and the Windows job was the one left
  out. All three builds, and the Windows packaging script, now stop at
  configuration rather than quietly producing a client that refuses them. This
  is **not verified on a Windows machine**: the change was written and reviewed
  on Linux, and the first Windows run of the suite is what will prove it.
- **A sticker the decoder refuses now gives its tile up.** The refusals that
  need no decoding — a build without libwebp, a sticker that is not a file on
  this computer — were reported from inside the item's own construction, which
  in the picker's grid happens inside the binding that decides whether to build
  it: the fallback wrote the property that binding was reading, Qt broke the
  loop, and the tile kept a dead decoder for the life of the panel while
  logging a binding loop. The refusal now goes out on the next turn of the
  event loop. This is what the Windows suite had been failing on, and it was
  never a Windows fault: built without libwebp on this machine it failed with
  the same numbers.
- **Account menu:** the ring marking where you are lands on the account you are
  using. The menu takes focus when it opens and focus went to whichever account
  came first, which contradicted the tick beside the account actually in use.

- **Sticker picker contents:** each sticker is listed once, and a sticker whose
  file was never downloaded is fetched rather than left as a grey box. The
  picker listed sticker *messages*, so one sticker sent in four conversations
  filled four tiles, and a row with no local file had nothing to draw. Stickers
  are now told apart by WhatsApp's own hash of the file, taken from the payload
  already stored for that message; a sticker whose hash is not known yet is
  never merged with another, since that would hide one that is not a duplicate.
- **Shared contacts:** a contact card for a number that is on WhatsApp now
  arrives with **Message** beside it instead of **Invite**, and the recipient's
  client can show that contact's profile photo. The card's telephone line was
  missing the `waid` parameter that tells the receiving client the number
  belongs to an account, so every card we sent looked like a stranger's number.
  The number is confirmed against WhatsApp before the card goes out: a number
  that is genuinely not registered still arrives as an invitation, and if the
  check itself fails the card is sent without the parameter rather than not
  sent at all. Where WhatsApp registers a different form of the number than the
  one you dial, the card carries the form WhatsApp answers with.
- **Tappable prompts, unverified:** a new `message.send_buttons` asks a question
  with up to three buttons, and a tap on one arrives as the new `message.tapped`
  event alongside the ordinary message. It is sent as an interactive message
  carrying a native flow, which is the only shape a linked device may send; the
  legacy buttons and list shapes are refused by WhatsApp outright. Whether the
  buttons actually draw on the reader's phone is **not established** — no prompt
  has been sent to a live recipient from this project, and the reports that say
  this shape renders are other people's. `poll.create` remains the official way
  to ask for a choice. See `docs/API.md` for what a refusal looks like.
- **Taps on someone else's prompt:** a tap on a prompt sent by a business account
  is now shown. All four answer shapes are read, and each appears in the
  conversation as a reply carrying the chosen title, which is how WhatsApp shows
  it. Until now none of them was handled at all, so a tap matched nothing in
  the message reader and was discarded as an empty message.
- **Lists:** `message.send_list` asks a question whose answers are too many to
  be buttons - up to ten choices behind one button, with headings and
  descriptions. It travels the way a button prompt does, as an interactive
  message carrying a native flow, because the legacy list shape is refused for
  a linked device.
- **An answer now says whether it can be trusted.** `message.tapped` carries
  `verified`, which is `1` only when the quoted message is a question this
  account itself asked in that same conversation and the choice is one that
  question offered. Everything in an answer is written by whoever sent it,
  including which message it claims to answer and which button it claims was
  pressed, so without this anyone able to write into a conversation could send
  something shaped like an approval. An answer that fails the check still
  arrives, marked `0`.
- **Clearing a conversation now takes the earlier wording of an edited message
  with it.** Those are kept against the conversation rather than the message
  rows, and clearing keeps the conversation, so until now they survived a clear
  and left behind the text of messages the reader had asked to be rid of.
- **A message this client cannot read now says so**, as the new
  `message.unmapped` event naming the fields it carries, and as a line in the
  log. Anything the reader could not place was dropped without a trace: on a
  live phone a tapped button was delivered, acknowledged to WhatsApp, and then
  vanished, with nothing anywhere to show it had ever arrived. Key exchange and
  other transport envelopes stay quiet, since those are machinery rather than
  something that failed to be read.

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
