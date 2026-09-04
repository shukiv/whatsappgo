# Desktop application override

This file is the authoritative visual specification for the native desktop
client and overrides the generic product palette in `../MASTER.md`. The current
installed WhatsApp Web PWA is the density and interaction reference; dimensions
were measured at a `1007 × 686` viewport with DPR `1.25` on 2026-09-03.
WhatsAppGo retains its own name and green identity and does not copy private web
implementation details.

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

Use `Theme.qml` semantic tokens in QML rather than repeating literal colors.
The light selected-filter treatment is `#D9FDD3` with `#008069` text. Use the
platform font and bundled line-icon set, with a visible keyboard focus ring on
every interactive control.

## Measured desktop geometry

| Component | Geometry |
| --- | ---: |
| Navigation rail | `64 px` wide |
| Rail action | `40 × 40 px` |
| Sidebar, conversation, drawer, media, and feature headers | `64 px` high |
| Chat filter strip | `42 px` high |
| Filter chip | `32 px` high |
| Archived row | `49 px` high |
| Chat row | `72 px` high |
| Chat avatar | approximately `49 × 49 px` |
| General menu row | `36 px` high |
| App menu | `229 px` wide |
| Chat context menu | `238 px` wide |
| Message menu | `196 px` wide |
| Attachment menu | `154 px` wide; eight `35 px` rows |
| Message-options icon | `16 px` glyph in a transparent `32 px` hit target |
| Quick-react control | `26 px` visible circle in a `36 px` hit target |
| Quick-reaction tray | `44 px` high; `32 px` cells |
| Contact drawer | `540 px` wide |
| Contact avatar | `120 × 120 px` |

These values are component baselines, not permission to hard-code unrelated
layouts. Preserve relative density when a platform font or scale factor changes.
Message bubbles are content-driven, cap at 68% of the conversation pane and at
620 px on a wide window, and must not resize when an asynchronous thumbnail is
replaced with a higher-quality source.

## Responsive behavior

- The direct filter row is **All**, **Unread**, and **Favorites**. **Groups** is
  also direct at `440 px` or wider and moves into the chevron overflow below
  that width.
- Do not append the unread total to the **Unread** chip label.
- The filter strip precedes the **Archived** row.
- Popup menus are laid out to their final visibility-dependent height and then
  clamped to an 8 px window margin. Opening near the composer or a window edge
  must never cut off the final action.
- Keep `40 px` rail actions and larger invisible pointer targets where needed;
  the visible message-hover chrome remains compact and transparent.
- The chat wallpaper appears only behind an open conversation. Empty content
  areas use the solid empty surface. Light-mode pattern contrast remains lower
  than the message content; dark mode uses its own calibrated opacity.

## Interaction rules

- Let each `ListView` own wheel physics. Preserve the top visible identity and
  pixel offset when rows are inserted or reordered; model refreshes must not
  reset the user's scroll position.
- Follow new messages only while the conversation is already at the latest
  message. Loading older history preserves the visible anchor.
- Menus opened by a hover affordance and by right-click expose the same action
  family and eligibility rules.
- Image media opens in the native viewer at aspect-fit 100%; zoom remains
  centered and Copy/Save are available without cropping the source.
- Playback marks supported audio/video as played and message information shows
  only receipt rows actually supplied by the backend.
- Every icon-only action has an accessible name. Unsupported linked-device
  actions are disabled or explain their limitation instead of failing silently.

The complete measured reference and remaining functional gaps are documented in
`../../../docs/WHATSAPP_WEB_PWA_CONTROL_INVENTORY.md` and
`../../../docs/WHATSAPP_WEB_PWA_GAP_AUDIT.md`.
