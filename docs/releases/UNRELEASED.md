# Unreleased changes

These changes are committed after the `v0.1.9` tag and are not in any published
artifact. Build from source containing them and restart that build to use them;
a rebuild does not replace an already running process.

## Fixed

- **Photo viewer:** dragging a zoomed photo is bounded by the photo instead of
  by the surface around it. A photo whose shape differs from the viewer is
  letterboxed, and the old bound was the viewer's box, so a drag could pull the
  picture off its own edge and leave the reader looking at empty space beside
  it. Panning, its limits, and the click that opens the action menu now have a
  regression that delivers real mouse events.

## Changed

- **Chat names:** the contact name in the chat list and in the conversation
  header starts beside the avatar whatever script it is written in. A name in
  Hebrew or Arabic was aligned to its own writing direction, which pushed it
  across the row to the timestamp while a name in Latin script stayed on the
  left. This repair is applied but not yet covered by a test: the offscreen
  harness renders the same name left-aligned either way, so the regression is
  tracked separately in `whatsappgo-ogw`.

## Documentation

- **Account risk:** the README, the security document and the bot API reference
  now state plainly that connecting with a client built on a reverse-engineered
  protocol breaches WhatsApp's Terms of Service, that enforcement lands on the
  phone number rather than on the software, and which behaviours actually draw
  a ban. The API page says outright that automation is the fastest route to a
  banned number.

## Known issues

- `desktop-resize-rendering` aborts in roughly one run in three with a
  `QFontDatabase` ordering message. It is a race, not a regression, and is
  tracked as `whatsappgo-524`.
