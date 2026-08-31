# Desktop application override

This native messaging interface overrides the generic purple product palette in
`MASTER.md`. The application uses a calm, high-contrast green identity that is
distinct from the official WhatsApp logo while retaining familiar messaging
semantics.

## Tokens

| Role | Light | Dark |
| --- | --- | --- |
| Primary | `#1DAA61` | `#00A884` |
| Primary container | `#D9FDD3` | `#005C4B` |
| App surface | `#FFFFFF` | `#111B21` |
| Navigation rail | `#F7F5F3` | `#111B21` |
| Header/composer | `#FFFFFF` | `#202C33` |
| Muted/search surface | `#F6F5F4` | `#2A3942` |
| Chat canvas | `#F5F1EB` | `#0B141A` |
| Empty conversation | `#F7F5F3` | `#0B141A` |
| Primary text | `#0A0A0A` | `#E9EDEF` |
| Muted text | `#54545A` | `#8696A0` |
| Border | `#DDDAD6` | `#2A3942` |
| Read receipt | `#53BDEB` | `#53BDEB` |
| Danger | `#EA0038` | `#F15C6D` |

Use the platform font and icon theme. Layout follows an 8 px rhythm, controls
retain visible keyboard focus, chat/message lists reuse delegates, and motion is
limited to native state transitions. Message bubbles cap at 68% of the message
pane width, and at 620 px on a wide window. Every icon-only action has an
accessible name.

The desktop shell follows the current spacious WhatsApp Web geometry: an 80 px
navigation rail, 72–80 px headers, 84 px chat rows, at least 44 px toolbar hit
areas, a 52–56 px avatar, pill-shaped searches, and a persistent composer.
Navigation uses a pale rounded selected tile with a dark active glyph. Bundled
line icons share a two-pixel rounded stroke. Profile images are circularized once
in the media cache rather than masked per frame. The chat wallpaper is shown only
for an open conversation; empty content areas use the solid empty surface.
