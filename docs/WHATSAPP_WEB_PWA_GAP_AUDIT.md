# WhatsApp Web PWA parity audit

Date: 2026-09-03 (measurements refreshed 2026-09-04; second pass 2026-09-04 against the matching account)

Implementation status updated: 2026-09-03 after the compact-geometry,
responsive-filter, scroll-anchor, media, and popup-boundary changes. A finding
marked **implemented** has code and automated coverage in this repository; it
does not imply that unrelated PWA actions in the same surface exist.

## Objective

This document compares WhatsAppGo with the installed, authenticated WhatsApp Web PWA. The PWA is the desktop reference: it removes normal browser chrome, uses the same desktop window constraints as WhatsAppGo, and exposes the actual authenticated menus and interaction states.

The audit covers visible features, menus, icons, geometry, enabled/disabled behavior, hover states, media behavior, and scrolling. It does not attempt to copy private implementation code or network protocols.

## How the reference was measured

- The installed WhatsApp Web PWA was attached to through Playwright in read-only fashion.
- Panels, menus, hover controls, and drawers were opened in the live authenticated UI.
- CSS bounding boxes were measured at a `1007 × 686` viewport with device pixel ratio `1.25`.
- No messages, reactions, pins, blocks, deletes, calls, or other account mutations were performed.
- WhatsAppGo was inspected from its QML and Go implementation, the supplied screenshots, and the existing desktop test suite.

The full traversal ledger is in [WHATSAPP_WEB_PWA_CONTROL_INVENTORY.md](./WHATSAPP_WEB_PWA_CONTROL_INVENTORY.md). It records each top-level surface, menu, submenu, selection state, confirmation boundary, and the message-state matrix. This audit is the prioritized interpretation of that inventory.

WhatsApp Web changes frequently. The dimensions below are a reference snapshot, not immutable constants. Preserve the relationships and density before chasing single pixels.

## Executive result

WhatsAppGo already has a substantial messaging core, but it is not yet behaviorally or structurally equivalent to the PWA.

The most important problems are:

1. **Scrolling has an implemented stabilization pass but remains a manual release check.** Conversation and chat-list updates now preserve user position and the desktop suite sends Qt wheel events during model changes. Real mouse hardware and long live histories still require release validation.
2. **Several visible controls are dead.** Calls, Channels, Communities, profile settings, status creation/menu, new group, new community, and six attachment actions are either placeholders or do nothing.
3. **Core menus are functionally incomplete.** Their compact row/width geometry and window-edge clamping now match the measured baseline, but the app, chat-row, chat-header, and message menus still omit many PWA actions.
4. **The conversation header is incomplete.** Video call, voice call, and the conversation overflow menu are absent.
5. ~~**There are unresolved icon references.**~~ Withdrawn on 2026-09-04: the icons are drawn on a `Canvas` from the reference's basename, so the absent `.svg` files change nothing. See P0-3.
6. **The contact drawer is only a subset.** It lacks edit, starred messages, lists, export, disappearing messages, privacy, moderation, and destructive actions.
7. **Some features that were previously reported missing now exist.** Presence formatting, played receipts, message info, in-app media viewing, image zoom/copy/save, optimistic reactions, status-notification suppression, and link-preview refresh should be regression-tested rather than reimplemented.

## Measured token reference (2026-09-04)

Taken from the live authenticated client with `getComputedStyle` on rendered
elements, at a `1007 × 627` viewport. WhatsAppGo runs at the same `1.25` scale,
so these numbers compare directly.

| Surface | WhatsApp Web | Was in WhatsAppGo | Status |
|---|---|---|---|
| Interface face | `Roboto` (shipped as a webfont) | `Segoe UI` stack → DejaVu Sans here | **Fixed**: Roboto leads the stack; packages depend on it |
| Read receipt (`--icon-ack`) | `#007BFC`, identical light and dark | `#53BDEB` | **Fixed** |
| Filter chip label | `13.33 px`, weight `400` | `14 px`, DemiBold/Medium | **Fixed** |
| Filter chip selected label | `#15603E` | `#008069` | **Fixed** |
| Filter chip border | `0.8 px rgba(0,0,0,.2)`, same in both states | `#D1D7DB` / `#A9DCA4` split by state | **Fixed** |
| Chat name / preview / time | `16/400`, `14/400 rgba(0,0,0,.6)`, `12/400` | matches | Aligned |
| Bubble body / meta | `14 px` on `20 px`, meta `11 px` | matches | Aligned |
| Outgoing bubble | `#D9FDD3`, radius `7.5 px`, tail corner squared | matches | Aligned |
| In-bubble link | `#1B8755` | uses `Theme.primary` `#1DAA61` | Open, minor |
| Rail / header / row / avatar | `64` / `64` / `72` / `48` | matches at scale | Aligned |

Dark-mode counterparts were **not** measured. Reading them requires switching the
account's appearance setting, which is a live account mutation, so the dark
values in `Theme.qml` are unchanged and remain unverified.

## Priority definitions

- **P0 — release blocker:** breaks a primary interaction or presents a visibly broken control.
- **P1 — core parity:** expected in daily desktop messaging.
- **P2 — secondary parity:** important but can follow the stable messaging core.
- **P3 — polish/optional:** exact styling, advanced integrations, or deliberate product extensions.

## Reference geometry

| Element | WhatsApp Web PWA | WhatsAppGo now | Assessment |
|---|---:|---:|---|
| Navigation rail | `64 px` | `64 px` | Matched |
| Rail action | `40 × 40 px` | `40 × 40 px` | Matched |
| Sidebar header | `64 px` high | `64 px` high | Matched |
| Conversation header | `64 px` high | `64 px` high | Matched |
| Chat sidebar | `403 px` in measured viewport | responsive `360–520 px` | Reasonable, but reference density differs |
| Filter strip | `42 px` high | `42 px` high | Matched |
| Filter chip | `32 px` high | `32 px` high | Matched |
| Archived row | `49 px` high | `49 px` high | Matched |
| Chat row | `72 px` | `72 px` | Correct |
| Chat avatar | `48 × 48 px` | approximately `49 × 49 px` | Correct |
| General menu item | about `36 px` high | `36 px` high | Matched |
| Main app menu | `229 px` wide | `229 px` | Matched |
| Chat menus | about `238 px` wide | `238 px` | Matched |
| Message menu | about `195 px` wide | `196 px` | Within measurement tolerance |
| Attachment menu | `153 × 298 px` | `154 px`; eight `35 px` rows | Matched density |
| Message-options hover button | `24 × 18 px`, transparent | `16 px` glyph in transparent `32 px` hit target | Matched visual treatment; accessible hit target retained |
| React hover button | `26 × 26 px` | `26 px` visual in `36 px` hit target | Matched visual treatment |
| Reaction-palette cell | about `29 × 29 px` | `32 × 32 px` | Within practical tolerance |
| Contact drawer | `540 px` wide | `540 px` | Matched |
| Contact avatar | `120 × 120 px` | `120 × 120 px` | Matched |

## P0 findings

### P0-1 — Conversation scrolling stabilization is implemented; validate live

**PWA behavior:** Native-feeling wheel/trackpad movement. Opening a conversation starts at the latest message. Older history loads without repeatedly pulling the viewport to either end. New messages only keep the view pinned when the user is already at the bottom.

**WhatsAppGo status:** The current implementation lets the list process wheel
input, follows new messages only near the latest edge, and preserves an anchor
while older rows are inserted. A Qt integration test sends multiple wheel events
during updates. Long, live histories and physical mouse/trackpad input remain a
release check because the original defect was timing-sensitive.

**Regression requirements:**

- Let `ListView` own normal wheel physics.
- Preserve an anchor message and its pixel offset when older rows are inserted.
- Track a small explicit state: `atLatest`, `readingHistory`, or `restoringAnchor`.
- Follow new messages only in `atLatest`.
- Do not call `positionViewAtBeginning/End` in response to unrelated model refreshes.
- Keep the Qt mouse-wheel integration test using multiple discrete wheel events
  during model inserts, and repeat it manually with a live account before release.

**Primary target:** `desktop/qml/Main.qml` message `ListView` and its loading/positioning helpers.

### P0-2 — Chat-list scroll preservation is implemented; validate live

**PWA behavior:** The list remains where the user placed it while previews, avatars, unread counts, presence, or message ordering refresh.

**WhatsAppGo status:** Row refreshes now preserve the visible position instead
of forcing the list to the beginning, and an integration test applies chat-row
updates while wheel scrolling. Reordering and high-frequency avatar refreshes on
a large live account remain manual regression cases.

**Required outcome:**

- Update rows in place whenever ordering has not changed.
- If sorting changes, preserve the top visible chat identity and its pixel offset.
- Never reset `currentIndex` or `contentY` merely because an avatar or preview changed.
- Add a real wheel test while chat rows, avatars, and unread counts update.

**Primary targets:** the chat model update path plus the chat `ListView` in `desktop/qml/Main.qml`.

### P0-3 — WITHDRAWN: icon references are not broken

**This finding was wrong and is retained only so it is not re-filed.**

It is true that `qml/icons/` contains no `calls.svg`, `status.svg`,
`channels.svg`, `communities.svg`, `profile.svg`, `group-add.svg`, or
`user-add.svg`, and that `Main.qml` writes `iconSource: "calls.svg"` without an
`icons/` prefix. The conclusion drawn from that — blank icons — does not follow.

`TintedIcon` never loads the file. It uses the URL only to derive a name:

```qml
readonly property string kind: {
    const parts = String(source).split("/")
    return parts[parts.length - 1].replace(".svg", "")
}
```

and then draws the glyph procedurally on a `Canvas`, which is what lets every
icon take an arbitrary tint. Every name listed above has a drawing branch, so
all of them render. A screenshot of the running rail confirms nine drawn icons
with no blank slots.

The real consequences are smaller and different:

- The `.svg` files under `qml/icons/` and their `RESOURCES` entries in
  `desktop/CMakeLists.txt` are dead weight for any icon drawn by `TintedIcon`.
- A name with no `Canvas` branch fails silently as an invisible icon rather than
  as a missing-file warning, so new icons need a visual check.

**Lesson for this audit:** a missing asset file is not evidence of a missing
icon until the code that consumes the reference has been read.

### P0-4 — Controls that look actionable do nothing

| Area | Visible control | Current result |
|---|---|---|
| Status | Add status and menu | No action |
| Calls | Call records / primary workflow | Generic, non-actionable section |
| Channels | Channel rows | No discovery/follow/open workflow |
| Communities | Community rows / creation | No creation/open workflow |
| Profile | Account, Privacy, Chats, Notifications | Rows have no action |
| New chat | New group, New community | Unavailable/dead path |
| Attachments | Camera, Contact, Poll, Event, New sticker | “Not supported yet” |

Until implemented, unsupported actions should be explicitly disabled with an explanation or hidden. A normal active affordance that silently does nothing is worse than a clearly unavailable feature.

## P1 findings — navigation and top-level screens

### P1-1 — Rail structure differs from the PWA

The PWA rail contains Chats, Calls, Status, Channels, Communities, a separator, Meta AI, global Media, and Profile. WhatsAppGo contains Chats, Calls, Status, Channels, Communities, Search, New Chat, Profile, and Theme.

Recommended parity changes:

- Add global Media.
- Move New Chat into the sidebar header, where it already exists in the PWA.
- Keep sidebar search in the sidebar instead of duplicating it in the rail.
- Move theme selection into settings rather than treating it as a primary navigation destination.
- Treat Meta AI as P3/optional unless a real supported integration exists.

### P1-2 — Calls screen is not equivalent

The PWA provides Favorites, Add favorite, Recents, and a central calls/video-calls state. WhatsAppGo currently presents a generic record list with no complete call workflow.

### P1-3 — Channels screen is not equivalent

The PWA separates owned/followed channels from discovery and provides actionable Follow rows. WhatsAppGo only exposes a generic channel list.

### P1-4 — Communities screen is not equivalent

The PWA has a prominent Create a community action and community workflow. WhatsAppGo only exposes a generic list.

### P1-5 — Profile/settings screen is incomplete

PWA settings observed in the live profile screen:

- Profile name/photo/username
- Account
- Privacy
- Chats
- Notifications
- Keyboard shortcuts
- Help and feedback
- Log out

“Quick actions” is the subtitle of Keyboard shortcuts in the current PWA, not a separate settings row.

WhatsAppGo displays only a subset, and most displayed rows have no behavior. Profile/account rename is an app-specific account alias feature today; it is not equivalent to editing the WhatsApp profile.

### P1-6 — Global media browser is missing

The PWA rail opens a **Media from all chats** dialog with Media, Documents, and Links tabs, grouped across conversations and date ranges. WhatsAppGo has per-contact media in the contact drawer but no global browser.

## P1 findings — conversation chrome and menus

### P1-7 — Conversation header actions are missing

The PWA header has profile/contact info, video call, voice call, search, and a 40 × 40 overflow button. WhatsAppGo exposes contact info and search only.

The overflow menu observed in the PWA contains:

1. Contact info
2. Search
3. Select messages
4. Mute notifications
5. Disappearing messages
6. Add/remove favorite
7. Add to list
8. Export chat
9. Close chat
10. Send call link
11. New group call
12. Report
13. Block
14. Clear chat
15. Delete chat

Unsupported protocol-dependent actions should be deliberately unavailable, not silently absent without product explanation.

### P1-8 — Main app menu contains the wrong command set

PWA app menu:

- New group
- Starred messages
- Select chats
- Mark all as read
- App lock
- Log out

WhatsAppGo menu:

- Search messages
- **Starred messages** (added 2026-09-04)
- Add another account
- Reconnect
- Appearance
- Unlink account

Account switching and reconnect are valid WhatsAppGo extensions, but they should not replace the daily PWA commands. Put extensions in a separate Accounts/Connection section or settings page.

Starred messages now opens a cross-conversation list backed by `messages.starred`;
clicking a row opens the chat it came from. New group, Select chats, Mark all as
read, and App lock are still missing.

### P1-9 — Chat-row menu is incomplete and lacks the hover affordance

PWA chat-row menu:

- Archive
- Mute notifications
- Pin/unpin
- Mark read/unread
- Add/remove Favorites
- Add to list
- Block
- Clear
- Delete

WhatsAppGo has only the first four. It opens on right-click, while the PWA also reveals a small dropdown affordance on row hover. The dropdown should use the same menu; left-click on the arrow and right-click on the row should be equivalent.

### P1-10 — Message menu is incomplete and has incorrect eligibility rules

The PWA menu is state-dependent; there is no single incoming-message menu. The observed matrix is:

| Message state | PWA action differences |
|---|---|
| Old outgoing text | Reply, Copy, React, Forward, Pin, Ask Meta AI, Star, Delete |
| Edit-eligible outgoing text | Same actions plus **Edit** |
| Incoming text | Adds Report when applicable; Delete is Delete-for-me |
| Incoming image | Adds Download, plus Forward/Star/Report/Delete |
| Outgoing with receipt state | Message info is available in the appropriate action path |

WhatsAppGo has Info, Copy, Copy image, Reply, React, Pin, Edit, Delete for
everyone, and — since 2026-09-04 — **Star/Unstar** and **Forward**. It still
lacks Delete-for-me for both directions, lacks Report, and does not match the
conditional order.

Star is two-way: the outgoing patch is `appstate.BuildStar`, and a star set on
the phone arrives as `*events.Star` or on a history page's
`WebMessageInfo.Starred`. A starred message shows a small star beside its clock,
as the PWA does.

Forward re-sends the message with `ContextInfo.IsForwarded` and an incremented
`ForwardingScore`, so the recipient sees the "Forwarded" label; the score is
persisted, and an incoming forward is labelled here too ("Forwarded many times"
from five hops). Known limits: the destination picker requires an explicit Send
press rather than sending on the first click (deliberate — a stray click would
be unrecoverable); an attachment must already be downloaded to be forwarded; and
a voice note forwards as ordinary audio, because the store keeps no PTT flag.

Edit is implemented but its current predicate—any outgoing, non-revoked text—is too broad. WhatsApp Web only exposes it within the server-supported edit window and editing uses a composer-integrated state with explicit accept/cancel controls. This is partial parity, not a missing feature.

Ask Meta AI is optional/P3. Forward, Star, correct Edit/Delete eligibility, and Delete-for-me are core/P1.

### P1-11 — Hover-control visual geometry is aligned

The message-options control now uses a small `16 px` chevron on a transparent
`32 px` hit target. The reaction control keeps a `36 px` hit target but renders
the visible circle at `26 px`. This matches the PWA's visual weight while
retaining accessible pointer and keyboard targets. Keep these layers separate.

### P1-12 — Reaction self-state is not represented

WhatsAppGo optimistically stores the user's reaction and aggregates reaction counts, but it does not visually identify the current user's selected reaction in the message summary or palette. The PWA makes the user's own reaction state apparent and allows replacing/removing it from the same control.

## P1 findings — contact and message information

### P1-13 — Contact drawer is structurally incomplete

The drawer now matches the measured `540 px` width, `64 px` header, and
`120 px` avatar. Its remaining gap is structural functionality, not shell
geometry.

Missing or incomplete PWA sections/actions:

- Edit contact/profile action
- Business information and share action when applicable
- Starred messages
- Disappearing messages
- Advanced chat privacy
- Phone/about detail
- Add to list
- Export chat
- Clear chat
- Block
- Report
- Delete chat

Already present: media/links/documents, search, favorite row, mute, archive, and encryption placeholder. Call/video are displayed but disabled.

### P1-14 — Message-info drawer geometry is aligned; state parity needs validation

The feature exists with the PWA-height `64 px` drawer header and includes Played
for audio/video, Read, and Delivered timestamps. Validate the message preview,
state icons, and conditional rows against live receipts; do not rebuild its data
model unless those receipts prove incorrect.

## P1/P2 findings — media and attachments

### P1-15 — Attachment-menu geometry is aligned

The PWA attachment menu measured about `153 × 298 px`. WhatsAppGo now uses a
`154 px` popup with eight `35 px` rows and compact padding. Preserve that density
while implementing the five placeholder workflows described below.

### P1-16 — Attachment options are placeholders

Document, Photos/videos, and Audio work. Camera, Contact, Poll, Event, and New sticker do not. These are P1 when the backend supports them; otherwise they must be visibly disabled. The PWA order is:

1. Document
2. Photos and videos
3. Camera
4. Audio
5. Contact
6. Poll
7. Event
8. New sticker

### P2-1 — Animated stickers are flattened

The backend converts unsupported WebP stickers to PNG, preventing Qt's “Unsupported image format” failure. Animated WebP currently falls back to its first frame, so it renders but does not animate.

### P2-2 — Link-preview quality needs an end-to-end check

The current code requests a higher-quality preview for visible cards and caps card width near `420 px`. That addresses the earlier low-resolution/wide-card complaint in principle. Verify that the final high-resolution source replaces the initial tiny inline thumbnail without resizing the bubble or shifting scroll position.

### P2-3 — Image viewer behavior exists but needs regression coverage

The viewer now uses aspect-fit, supports wheel/touchpad zoom, opens in-app, and provides Copy image and Save image from either mouse button. Add tests for portrait, landscape, ultrawide, very tall screenshots, and a source larger than the viewport. The image must never be cropped at 100% fit.

### P2-4 — Video/audio played state exists but must remain wired for every entry point

`markMediaPlayed` is invoked by the playback path, and message info has a Played row. Verify it fires once for audio, voice notes, and video whether playback begins from the message, preview, or media viewer.

## P2 findings — message presentation

### P2-5 — Quoted-message navigation requires a durable anchor

Clicking a quoted preview should load the required history if needed, jump to the original message, and briefly highlight it. The anchor must survive pagination and model inserts without reviving the scroll-jump bug.

### P2-6 — Chat bubble density needs visual calibration

Bubble padding was increased after user feedback, but it should be checked against the PWA with:

- one-line messages
- long RTL messages
- links with previews
- replies/quotes
- media captions
- reactions
- timestamps and receipts

Use content-driven width with a maximum, rather than a fixed broad card. Avoid changing bubble geometry after asynchronous thumbnails arrive.

### P2-7 — Conversation background contrast is calibrated

The light-mode pattern uses the reduced opacity calibrated against the PWA while
the dark theme retains its separate value. Keep screenshot coverage for both so
future palette work does not raise the light pattern above message content.

## Features already implemented — regression-test, do not duplicate

| Request/behavior | Current implementation status | Required verification |
|---|---|---|
| Account switcher dropdown | Implemented as a WhatsAppGo extension | Click target, active account, rename, counts, keyboard dismissal |
| Rename local account profile | Implemented | Clarify local alias vs WhatsApp profile name |
| Presence/last seen | Online, typing, recording, last-seen strings, and blank unknown state exist | Never show generic “Connected” or “Offline” |
| No status notifications | Backend excludes `status@broadcast` | Regression test incoming status events |
| Own reaction optimistic update | Backend update exists | Add selected/self visual state |
| Multiple reaction aggregation | Implemented | Verify identities and count popup |
| In-app image viewer | Implemented | Aspect-fit and high-resolution source |
| Wheel/touchpad image zoom | Implemented | Clamp and center behavior |
| Copy/save open image | Implemented | Clipboard MIME and save-dialog error handling |
| Images open inside app | Implemented | Ensure every thumbnail uses the same viewer path |
| Video playback in app | Implemented with Qt Multimedia | Codecs, poster frame, error state, played receipt |
| Played/read media receipt | Backend/UI wiring exists | Validate all media entry points |
| Message information drawer | Implemented | Conditional rows and PWA geometry |
| Per-contact media/links/docs | Implemented | Thumbnail quality and paging |
| Link-preview refresh | Implemented in code | Verify source replacement without layout jump |
| WebP sticker fallback | Implemented | Add animation support later |
| Attachment menu options | All labels are displayed | Five options remain non-functional |
| Compact rail/header/filter geometry | Implemented to measured baseline | Keep exact geometry tests |
| Responsive filter overflow | Implemented | Verify narrow and wide sidebars |
| Compact shared menu geometry | Implemented | Preserve per-menu widths and 36 px rows |
| Popup edge clamping | Implemented after final layout polish | Test bottom/right-edge action menus and reaction tray |
| Contact/message drawer shell geometry | Implemented | Remaining section/action gaps are tracked above |

## WhatsAppGo-specific features to keep

Parity does not mean removing useful product-specific capabilities. Keep these, but visually separate them from canonical WhatsApp commands:

- Multiple local accounts and account switching
- Reconnect/account health controls
- Local profile aliases
- Desktop-native file and media integration
- Any diagnostic controls intended for this linked-device client

## Recommended implementation order

### Phase 1 — stable foundations

1. Keep the implemented anchor-preserving scroll behavior covered by Qt wheel
   events and validate it on long live histories.
2. Resolve and package every visible icon.
3. Disable or label every unimplemented control.

### Phase 2 — daily messaging parity

1. Add the conversation-header call/video/overflow structure.
2. Implement the PWA main menu, chat-row menu, and message menu.
3. Add the chat-row hover dropdown; preserve the implemented compact
   message-hover geometry.
4. Add Forward, Star, Delete-for-me, Favorites, and Lists where supported.
5. Add explicit current-user reaction state.

### Phase 3 — information and media

1. Complete the missing contact-drawer sections on the implemented PWA-sized
   shell.
2. Add the global Media/Documents/Links browser.
3. Finish attachment workflows or mark them unavailable.
4. Validate high-resolution link previews, image fit, video playback, and played receipts.

### Phase 4 — top-level experiences

1. Complete Profile/settings navigation.
2. Complete Calls/Favorites/Recents.
3. Complete Communities creation/opening.
4. Complete Channels discovery/following.
5. Complete Status creation/menu behavior.

### Phase 5 — fidelity and optional integrations

1. Maintain the implemented compact geometry and light/dark wallpaper
   calibration with screenshot and component tests.
2. Animate stickers.
3. Consider App lock, advanced privacy, and Meta AI only when the product and backend can support them honestly.

## Acceptance criteria for parity work

- A visible enabled control always has a result and feedback.
- Menu contents and ordering match the PWA for the same message/chat/account state, except for clearly separated WhatsAppGo extensions.
- Real mouse-wheel scrolling remains stable while messages, avatars, previews, unread counts, and receipts update.
- Loading older messages preserves the same visible anchor and pixel offset.
- New messages follow automatically only when already at the latest message.
- Media opens in-app, is not cropped, upgrades to its best available quality, and marks playback correctly.
- Unknown presence renders as no subtitle; it never renders generic “Connected” or “Offline.”
- Unsupported linked-device features are disabled or omitted intentionally, never presented as working actions.
- Light and dark modes both retain readable contrast without inconsistent icon backgrounds.

## Typeface and script coverage

The interface carries its own fonts (`desktop/fonts/`, embedded through
`qt_add_resources`) instead of depending on distribution packages.

- **Roboto** Regular/Medium/Bold — the face WhatsApp Web renders, which it ships
  as a webfont. Bundling is what makes the two clients agree on a machine that
  has never installed it.
- **Noto Sans** for 21 further scripts, plus **Noto Sans CJK** for Chinese,
  Japanese and Korean. Roboto covers only Latin, Greek and Cyrillic; without
  these a Hebrew or Hindi conversation is drawn by whatever the machine happens
  to have. On a plain Debian desktop that meant Unifont for Telugu and Khmer,
  FreeSans for most Indic scripts and WenQuanYi for CJK — several unrelated
  typefaces in a single chat list.

`main.cpp` enumerates `:/fonts` rather than naming files, so adding a face to
the CMake list is enough; Roboto is then moved to the front because every other
bundled family is only a script fallback. Families that are still not bundled
resolve through the system list that follows.

`BIG_RESOURCES` on that resource target is required, not tuning. Without it
`rcc` emits the fonts as a C++ byte array — about 1.5 million lines for the CJK
collection — and the compiler exhausts its scratch space.

`desktop-bundled-font` covers this: it asserts Roboto loads with all three
weights, that it is what the interface font actually resolves to, and that
`QFontMetrics::inFont` can draw a sample character from each of 26 scripts.
A font that is packaged but never reached by the fallback chain still fails.

Licences ship beside the fonts: Roboto is Apache-2.0
(`fonts/Roboto-COPYRIGHT`), Noto is OFL-1.1 (`fonts/Noto-COPYRIGHT`,
`fonts/NotoCJK-COPYRIGHT`). Both are compatible with this project's
GPL-3.0-or-later.

## Defects found while measuring (2026-09-04)

### Menus near a window edge were not clamped

`WhatsAppMenuPopup.clampToParent()` was re-run from `onHeightChanged` and
`onImplicitHeightChanged`, but both were guarded by `opened`. `Popup.opened`
only becomes true once the 90 ms enter transition finishes, so a menu whose rows
were still being laid out during that animation kept the position it was given
before its final height was known — the exact overflow the clamp was added to
prevent. The message menu opened at `y = 415` with height `270` inside a `600`
tall overlay: `685` against a `592.5` limit.

The guard is now `visible`, which is true for the whole time the menu is on
screen. The horizontal and vertical clamps were also separated, so a menu taller
than its parent no longer skips the horizontal clamp as well.

### The message-interaction test depended on a hand-made file

The test pointed a delegate at `/tmp/whatsappgo-image-test.jpg` but nothing in
the repository created it. It passed only on machines where an earlier manual
run had left that file behind; on a clean `/tmp` the picture failed to load, the
preview control was never built, and the test failed with no indication that a
fixture was missing. The test now writes the image itself.

## Re-measured against the matching account (2026-09-04, second pass)

The first pass compared WhatsAppGo with a WhatsApp Web session that was signed in
as a *different* account. Chrome holds the `default` profile's account
(`573112522689`); the app under inspection was running the `israeli` profile
(`972525290986`). Layout and menu structure do not depend on the account, so the
earlier geometry stands, but every behavioural check that crossed the two — most
of all the starred-message round trip — had to be redone. This section records
the second pass, taken at a `1236 × 674` CSS viewport with device pixel ratio
`1.25`, with both sides on the same account.

The reference geometry table above was re-derived independently and held: rail
`64`, rail action `40 × 40`, sidebar header `64`, conversation header `64`, chat
row `72`, archived row `49`, filter chip `32`. The composer footer measures
`678 × 64` in the same viewport.

### Rail rhythm drifts by 2 px per slot

PWA rail actions sit at `y = 10, 54, 98, 142, 186` — a `44 px` pitch on a `10 px`
top inset. WhatsAppGo uses a `12 px` top margin and `6 px` spacing between `40 px`
actions, which is a `46 px` pitch. The error accumulates: by the fifth action
(Communities) WhatsAppGo is `10 px` lower than the reference. The bottom cluster
has the same `44 px` pitch (global Media `y = 580`, Profile `y = 624`) on a
`10 px` bottom inset.

**Implemented 2026-09-04.** The rail column now uses `anchors.topMargin: 10`,
`anchors.bottomMargin: 10` and `spacing: 4`, giving the reference `44 px` pitch.
The desktop smoke test asserts those three values so the rhythm cannot drift back.

### The rail carries two identity affordances, the PWA carries one

WhatsAppGo renders the account avatar as the **first** rail item and a Profile
action at the bottom. The PWA has nothing above Chats; identity lives only in the
bottom `Tú` avatar action. This is a structural difference, not a spacing one,
and it is what pushes the whole WhatsAppGo rail down relative to the reference.

This extends [P1-1](#p1-1--rail-structure-differs-from-the-pwa).

### Sidebar header has an extra control off the action grid

PWA: wordmark, then `Menú` (`40 × 40` at `x = 450`), then the green circular
`Nuevo chat` button (`40 × 40` at `x = 498`) — a `48 px` pitch with a `20 px`
right inset. WhatsAppGo matches the wordmark, the overflow button and the green
circular new-chat button, but inserts an account-switcher button between the
title and the overflow menu at `44 × 44`, which has no PWA counterpart and breaks
the `40 px` action grid.

The account switcher is a WhatsAppGo feature (see “WhatsAppGo-specific features
to keep”), so the recommendation is to keep the capability but move it onto the
bottom Profile action rather than the sidebar header, and size it `40 × 40` if it
stays.

### App menu overlaps the reference in two items out of six

| PWA (`Menú` in the sidebar header) | WhatsAppGo (`sidebarMenu`) |
|---|---|
| Nuevo grupo | — |
| Mensajes destacados | Starred messages |
| Seleccionar chats | — |
| Marcar todos como leídos | — |
| *separator* | *separator* |
| Bloqueo de aplicación | — |
| Cerrar sesión | Unlink this account |
| — | Search messages |
| — | Add another account |
| — | Reconnect |
| — | Appearance |

Missing: New group, Select chats, Mark all as read, App lock. `Search messages`
duplicates the sidebar search field; `Appearance` belongs in settings per
[P1-1](#p1-1--rail-structure-differs-from-the-pwa).

### Conversation header is missing three of four actions

PWA right cluster: `Videollamada`, `Llamada`, `Buscar`, `Menú` at
`x = 1012, 1068, 1124, 1180`, each `40 × 40` on a `56 px` pitch with a `16 px`
right inset, plus a `55 × 40` profile-info button at `x = 574` on the left.
WhatsAppGo has the profile-info affordance and `Search message history` only.

### Conversation overflow menu, enumerated

The PWA menu has fifteen entries, in this order:

1. Info. del contacto
2. Buscar
3. Seleccionar mensajes
4. Silenciar notificaciones
5. Mensajes temporales
6. Añadir a Favoritos
7. Añadir a la lista
8. Exportar chat
9. Cerrar chat
10. Enviar enlace de llamada
11. Nueva llamada en grupo
12. Reportar
13. Bloquear
14. Vaciar chat
15. Eliminar chat

**Partly implemented 2026-09-04.** The conversation header now has an overflow
menu (`conversationMenu`) carrying the four entries that have real behaviour
behind them: `Contact info` (`Group info` in a group, as the PWA does),
`Search`, `Mute notifications`/`Unmute notifications`, and `Close chat`. The
desktop smoke test asserts the button and all four entries exist.

The other eleven are still absent, and each is blocked on something rather than
on the menu: select-messages needs a multi-select mode; disappearing messages,
add-to-favorites, add-to-list, export, report, block, clear chat and delete chat
have no RPC in `internal/service/api.go`; the two call entries need the call
support discussed in [P1-2](#p1-2--calls-screen-is-not-equivalent). Entries were
deliberately not added as placeholders — dead controls are themselves a P0
finding here.

### Sidebar geometry, measured against the installed PWA

The earlier passes measured the reference in a browser tab. The installed PWA is
the better reference — no browser chrome, its own window metrics — so this pass
put both windows at an identical `1235 x 761` client area and compared captured
pixels. Both render at a `1.25` device pixel ratio, so one device pixel is
`0.8` logical pixels in the numbers below.

| Measurement (device px) | PWA | WhatsAppGo before | WhatsAppGo after |
|---|---:|---:|---:|
| Sidebar/conversation divider | `573` | `529` | `574` |
| Sidebar wordmark left edge | `107` | `97` | `107` |
| Search pill left/right | `107` / `545` | `96` / `513` | `109` / `544` |
| Chat row avatar left/right | `109` / `168` | `99` / `159` | `108` / `168` |

Three changes closed those gaps:

- **Sidebar width.** The PWA holds `~40 %` of the window: `394` logical px at a
  `988` px viewport and `493` at `1236`, the same ratio at both. WhatsAppGo asked
  for `30 %` and so sat on its own `360` px floor at ordinary window sizes. It now
  uses `Math.max(360, Math.min(560, window.width * 0.40))`.
- **Sidebar inset.** The PWA insets the wordmark, the search pill and the row
  avatars to the same `22` logical px from the rail. WhatsAppGo used `12` in the
  header and `12` on the search field, and its rows landed at `15`. Header is now
  `20` (`+8` from the label's own bearing), search `22`, and the chat list gains a
  `7` px left margin that puts the avatar at `22` while keeping the row's hover
  pill symmetric.

### Sidebar vertical rhythm still differs

With the horizontal geometry matched, chat rows still start about `24` device px
(`19` logical) higher than the PWA's. The PWA gives the search field and filter
strip `90` logical px between the header and the list (`#side` at `y = 64`,
`#pane-side` at `y = 154`); WhatsAppGo spends `64` on the search row alone and
reaches the list sooner. Not yet changed — the fix is in the search row's height
and padding, not in the list.

### Group messages had no sender label

Reported from a live group on 2026-09-04: incoming bubbles carried no name,
where WhatsApp Web labels every one. `messageFromEvent` takes the name from
`evt.Info.PushName`, and `ParseWebMessage` fills that from the history payload's
`pushName` — which WhatsApp routinely omits for group participants, so every
synced group message stored an empty `sender_name`.

Three changes, because the problem had three parts:

- **New messages.** `withSenderName` now resolves an empty name from the address
  book (`FullName`, `PushName`, `BusinessName`), then from that sender's own
  conversation title, and finally from the number.
- **Existing history.** A one-off migration labels rows already stored, joining
  against a grouped set of names the same senders used elsewhere and against the
  `chats` titles. The first version used correlated subqueries and was quadratic:
  it blew the store's ten-second open deadline on a 31 MB history and left the
  daemon crash-looping on `open message store: context deadline exceeded`. It is
  now two `UPDATE ... FROM` joins, with a new `idx_messages_sender` index.
- **Whatever is still unknown.** The delegate falls back to the sender's number,
  so a group bubble is never unlabelled.

### Chat-row menu is four entries short

The PWA row menu has, in order: Archivar chat, Silenciar notificaciones (with an
8 hours / 1 week / Always submenu), Fijar chat, Marcar como no leído, Añadir a
Favoritos, Añadir a la lista (submenu), then Vaciar chat and Eliminar chat.

Implemented 2026-09-04: the mute submenu (`chat.mute` now takes
`duration_seconds`, where zero means "until undone") and **Delete chat**, which
sends `appstate.BuildDeleteChat` and clears the local rows in one transaction,
behind a confirmation dialog.

Still absent, each blocked on the protocol rather than on the menu:

- **Añadir a Favoritos** — whatsmeow exposes the `favorites` index but no patch
  builder, so the app can read favourites and not set them.
- **Añadir a la lista** — needs label plumbing: `appstate.BuildLabelChat` exists,
  but nothing in this repository tracks label ids or names yet.
- **Vaciar chat** — no builder and no client call; deleting only the local rows
  would silently diverge from the account.

### Side-by-side against the PWA, second round of fixes

Implemented 2026-09-04 after comparing full-window captures:

- **Rail contents.** The account avatar at the top of the rail is gone; WhatsApp
  Web has nothing above Chats, and the extra slot pushed every destination down.
  Search, New chat and the theme toggle are gone too — the first two are sidebar
  controls in the PWA and already exist there, and appearance belongs in the
  menu. The rail is now Chats, Calls, Status, Channels, Communities with the
  account at the bottom, which is the PWA's structure minus Meta AI and the
  global media browser. The connection dot moved onto the bottom action with the
  identity it belongs to.
- **Chat rows.** The list now shows what WhatsApp Web shows: a type icon instead
  of the English word (`gallery`, `camera`, `mic`, `document`, `sticker`,
  `contact`, `pin`), the outgoing receipt state through the same `ReadReceipt`
  component the bubbles use, a voice note's length in place of the word "Audio",
  and relative timestamps — a clock time only for today, then `Yesterday`, then
  the weekday, then a short date. `model.Chat` carries `last_message_kind`,
  `last_message_from_me`, `last_message_status` and `last_message_duration` for
  this; the rendered preview string alone could not express any of it.

Still different, and deliberately so for now:

- The sidebar header keeps its account-switcher button. Multi-account is a
  WhatsAppGo feature with no PWA counterpart; the audit's recommendation is to
  fold it into the bottom rail action, which is a larger change than this pass.
- The filter strip's trailing control stays a chevron. The PWA's is `+`
  (Nueva lista); a `+` that opens a filter list rather than creating one would
  misdescribe itself, so the glyph waits for list support.
- The empty conversation pane still shows WhatsAppGo's own card rather than the
  PWA's promotional panel and its four quick actions.
- Meta AI and the global media browser remain absent, as does the PWA rail
  separator that groups them.

### Menu and destination parity, third round

Implemented 2026-09-04:

- **New group** (`group.create` → `whatsmeow.CreateGroup`). The app menu's first
  PWA entry now exists, with a name field capped at the 25 characters WhatsApp
  accepts and a multi-select member list drawn from existing one-to-one chats,
  because a free-text JID field would invite strangers on a typo. The new
  conversation opens as soon as the server confirms it.
- **Mark all as read** (`chats.mark_all_read`). Clears every unread conversation,
  archived ones included. One failure does not abandon the rest; the count that
  was actually cleared comes back.
- **Media from all chats** (`media.shared`). The PWA's global browser exists as a
  rail destination in the bottom cluster where the PWA keeps it, with the same
  Media / Documents / Links categories and end-of-list paging. The store's
  per-chat shared-content query was generalised rather than duplicated: both
  browsers now run through one `sharedMessages` helper, with the chat filter
  optional.
- **Chat rows and archived row.** The archived row's glyph and label were 20 px
  left of the chat titles below it; both now use the chat rows' own columns.
- **Search field.** The pill was 48 px tall against the PWA's 40.
- **Rail pills.** Selected destinations use the PWA's rounded square rather than
  a circle.

One change was made and then reverted: chat rows briefly gained a separator
hairline. Sampling the PWA capture showed no such line — what looked like a
separator was a hover edge. The rows have none again.

Measured after this round, at an identical `1235 x 761` client area: sidebar
divider `574` against the PWA's `573`, wordmark left edge `107` in both, search
pill `109..544` against `107..545`, row avatar `108..168` against `109..168`, and
rail actions on the same `56`-device pitch with ink boxes within two pixels of
the reference.

Still open, in rough order of visibility:

1. **Select chats** and **Select messages** — both need a multi-select mode the
   app does not have; the bulk actions behind them (star, delete, forward) all
   exist already.
2. **Add to Favorites** — `favorites` is a whole-list app-state action with no
   whatsmeow builder, so the collection and mutation version would have to be
   guessed. Given what a wrong star key already cost here, that is a protocol
   investigation, not a menu change.
3. **Add to list / Nueva lista** — `BuildLabelChat` exists but nothing tracks
   label ids or names yet, so the filter strip's trailing control stays a
   chevron rather than the PWA's `+`.
4. **Clear chat** — no builder and no client call.
5. **Video and voice call buttons** — the header has room for them, but placing
   calls is unsupported, and a dead control is its own P0 finding here.
6. **The empty conversation pane** still shows WhatsAppGo's card rather than the
   PWA's panel and four quick actions.
7. **Meta AI** and the rail separator that groups it.
8. **The sidebar header's account switcher**, which has no PWA counterpart.

### Selection modes, blocking and delete, fourth round

Implemented 2026-09-04:

- **Select messages** (conversation menu). The header swaps for a bar carrying
  the count and bulk Star, Forward and Delete. Whole message records are held
  rather than ids, because starring and deleting both need the sender and the
  direction. Hover actions and the context menu are suppressed while selecting,
  so one tap cannot both select and act. The forward dialog now takes a batch.
- **Select chats** (app menu). The sidebar header swaps for the same shape of
  bar with Mark as read and Archive, and rows become checkboxes.
- **Block / Unblock** (`contact.block`, `contacts.blocked` →
  `whatsmeow.UpdateBlocklist` / `GetBlocklist`). WhatsApp owns the list, so the
  client reads it back after every change rather than assuming its own write
  landed. The entry hides itself in a group, which cannot be blocked.
- **Delete chat** also reached the conversation menu, behind the same
  confirmation the row menu uses.
- The account switcher is now `40 x 40` rather than `44`, so it no longer breaks
  the header's action grid even though it has no PWA counterpart.

The conversation overflow menu now carries seven of the PWA's fifteen entries:
Contact info / Group info, Search, Select messages, Mute, Close chat, Block and
Delete chat.

**Withdrawn:** the empty conversation pane was listed as a gap. It is not. The
PWA's standard empty state is the same shape as WhatsAppGo's — a centred card
reading "Send and receive messages without keeping your phone online" above an
end-to-end-encryption line. The calls panel with four quick actions seen in the
2026-09-04 captures is a transient announcement, not the layout.

Still absent, each blocked on the protocol rather than on the interface:

- **Add to Favorites** — a whole-list app-state action with no whatsmeow
  builder; the collection and mutation version would have to be guessed.
- **Add to list / Nueva lista** — `BuildLabelChat` exists, but nothing tracks
  label ids or names, so the filter strip keeps its chevron.
- **Clear chat**, **Disappearing messages**, **Export chat**, **Report** — no
  builder and no client call.
- **Video and voice calls** — placing calls is unsupported.
- **Meta AI** and the rail separator that groups it.

### Chat lists, fifth round

Implemented 2026-09-04. Lists are what WhatsApp Web calls the labels it keeps in
app state, and they were the last of the missing menu entries with real protocol
support behind them.

- **Storage.** A `labels` table and a `chat_labels` join table. Deleting a list
  keeps a tombstone and drops its memberships in the same step, so a later patch
  cannot resurrect a list or leave a chat pointing at one that is gone.
- **Inbound.** `label_edit` and `label_jid` app-state indices are handled: the
  first records the list, the second its membership. The index carries which list
  and which chat; the action only says whether the chat is in it.
- **Outbound.** `label.create` (`appstate.BuildLabelEdit`) and `chat.label`
  (`appstate.BuildLabelChat`). WhatsApp keys lists by a small integer, so the id
  comes from the store's next free one rather than being invented per call.
- **Interface.** "Add to list" in the row menu with a submenu of the account's
  lists; the filter strip shows each list as a chip and its trailing control is
  now the PWA's `+` (New list) once the chips fit, collapsing back to the
  chevron over hidden filters when they do not — a `+` while chips are hidden
  would describe the wrong action.
- Chats carry `label_ids` from the list query, so filtering by a list costs no
  extra round trip.

`labels.list` answers on the live account; it is empty until the account's
`regular` collection syncs its own lists.

Still absent, all of them protocol limits:

- **Add to Favorites** — a whole-list app-state action with no whatsmeow
  builder; its collection and mutation version would have to be guessed, and a
  wrong key has already cost this project a day.
- **Clear chat**, **Disappearing messages**, **Export chat**, **Report** — no
  builder and no client call.
- **Video and voice calls** — placing calls is unsupported.
- **Meta AI** and the rail separator that groups it.

### Dropping files onto a conversation

Implemented 2026-09-04. WhatsApp Web attaches a file dropped onto the
conversation; WhatsAppGo had no `DropArea` at all, so a dragged file did
nothing.

A drop never sends on its own. Sending is irreversible and outward-facing, and a
stray drag across the window should not put a file in front of a contact, so the
drop opens a confirmation naming the recipient and listing what was dropped. A
caption field appears only for a single file: WhatsApp asks per item when there
are several, and one shared caption would not mean the same thing.

The area stays inert until a conversation is open, since a drop with nowhere to
send is not an error worth reporting. Non-`file://` payloads — a dragged link,
say — are refused at the drop rather than failing later in the daemon.

The first version borrowed `Kirigami.PromptDialog` and looked nothing like the
app: a stock frame, the file's full absolute path as a line of body text, and
mnemonic-underlined Cancel/Send buttons. It is now a built popup on the app's own
palette — a dimmed backdrop, a rounded card, a close button beside the
recipient's name, a preview area showing the image itself or a document glyph
with the file's base name, the caption in a pill like the composer's, and a green
circular send button.

Covered by `desktop-file-drop`, which checks the inert state, that the caption
appears for one file and disappears for two. Mutation-checked: forcing the
caption always visible fails the test. Passing `--screenshot` to that same test
captures the dialog, so its appearance can be reviewed without driving the live
window — which is how the redesign was checked.

### Star app state is verified end to end

Starring from WhatsAppGo now appears in the PWA's `Mensajes destacados` panel,
and unstarring removes it; a star made in the PWA arrives as `*events.Star` on
the next `regular_high` sync and updates the local row. The index WhatsApp's own
client writes uses the chat's **LID** form with an empty sender slot for
own-account messages, which is what `starSenderJID` in
`internal/whatsapp/chatsettings.go` now produces. Inbound app state was briefly
suspected of being dead; that was the cross-account mistake described above, and
it works.

## Test-suite note

The desktop suite reports 26 passing tests after this update. It exercises
rendered QML with Qt wheel events during asynchronous message/chat updates,
checks exact compact filter geometry, and opens a bottom-edge message menu to
verify that both the menu and reaction tray stay inside the window. This proves
the guarded code paths, not complete PWA parity; physical mouse/trackpad input,
long live histories, codec variation, and authenticated receipt states remain
manual release checks.

## 2026-09-04 — Chat-row hover chevron, Block, and the message-scroll regression

### Hover chevron on chat rows

WhatsApp Web reveals a chevron on the hovered chat row that opens the same menu
as right-click, and hides the pin and unread marks while it shows. WhatsAppGo had
the menu but only on right-click, so the affordance was invisible.

- `ChatListDelegate.qml` gained `chatRowMenuButton`, bound to a new
  `showRowMenu` property (`hovered || chatMenu.opened`, off during selection mode)
  so the chevron survives while its own menu is open.
- `pinnedMark` and `unreadBadge` are suppressed while the chevron shows, matching
  the PWA.
- Menu placement moved into `openChatMenuAt(item, x, y)`, shared by the chevron
  and the right-click `TapHandler`, clamped inside the overlay.
- `TintedIcon` gained a `chevron-down` kind.

Verified against the running client at 1600x1000: hovering a row shows the
chevron right-aligned on the preview line, in the same position as the PWA.

### Block / Unblock

The PWA menu has `Bloquear` above the destructive group. The daemon already
exposed `contact.block` and `contacts.blocked`, so the item is now wired:
`chatBlockItem`, hidden for `@g.us` chats, comparing the user part of the JID
because the blocklist comes back in phone form while a chat may be keyed by LID.

Still absent from our menu, unchanged from the earlier audit rounds and blocked
below the UI: **Add to Favorites** (no whatsmeow builder for the whole-list
`favorites` app-state action) and **Clear chat**.

### desktop-message-scroll: root cause

The test had been failing on every run. It was not caused by any of the changes
in this branch — it reproduces on a pristine `HEAD` checkout of `desktop/`.

The test drives a real window-system wheel event at the ListView's scene
position. The conversation split is `visible: backend.loggedIn`, and the test
runs with no daemon, so the split is hidden, the ListView is not hit-testable,
and the wheel lands on nothing: `contentY` never moved. It used to pass only
because a daemon happened to be running on the developer's machine.

Two fixes in `main.cpp`:

- Reveal every ancestor of the list through `QQmlProperty(...).write(true)`.
  A plain `setVisible` leaves the `visible: backend.loggedIn` binding in place,
  and the next failed reconnect re-hides the split mid-test.
- Capture the wheel point *and* the tail `contentY` after that reveal — showing
  the split relays out the list, and a tail captured before it made the
  monotonic check fire on the first tick.

28/28 desktop tests pass, three consecutive runs.

### Follow-ups on the same round

- `desktop-chat-row-menu` (new test) drives a `ChatListDelegate` parented into the
  window: it hovers the row through real window mouse events, asserts the chevron
  appears and the pin/unread marks give way to it, presses the chevron and asserts
  the row menu opens, then swaps the JID to a group and asserts `Block` disappears.
  Waiting on the popup's enter transition matters - `opened` is false until it has
  run.
- `Block` now uses a `block` icon (circle with a diagonal slash) rather than a
  padlock, matching the PWA glyph, and is hidden for `@g.us`, `@newsletter` and
  `@broadcast` chats.
- `refreshBlockedContacts()` and `refreshChatLabels()` only ran when *switching*
  to the chats section, so they never ran on the section the window starts on:
  every chat would have offered `Block` forever, including blocked ones. Both now
  also run when the daemon connects.

29/29 desktop tests pass.

## 2026-09-04 — Kirigami removed from the UI, and a way off the linking screen

### The dialogs are the application's own

Every Kirigami dialog is gone. `WhatsAppDialog.qml` is the replacement: a centred
card on a dimmed page, title, optional subtitle, a body, and right-aligned
actions - a borderless Cancel and a filled pill that turns red when the action is
destructive. Converted: unlink, message search, starred messages, rename account,
pin duration, new list, delete chat (conversation header and chat row), new group,
forward, edit message, delete message.

Supporting controls, so a dialog no longer mixes product styling with the
platform's: `DialogTextField.qml` (rounded, borderless, optional search glyph,
replacing `TextField` and `Kirigami.SearchField`) and `DialogRadioButton.qml`.

`Kirigami.ApplicationWindow`, `Kirigami.Page` and `Kirigami.Separator` went too;
no QML file imports Kirigami any more. The Qt Quick Controls style is still
`org.kde.desktop`, which the remaining stock controls (`TextArea`, `ItemDelegate`
rows, `BusyIndicator`, `ToolTip`) inherit - changing that is a separate visual
pass.

Dialogs can now be photographed without a daemon: `--screenshot <path>
--open-dialog <objectName>` opens one by name before grabbing the window.

### The linking screen had no exit

Reported: *"there is no exit from this screen"*. `PairingPage` is shown whenever
`!backend.loggedIn`, and it carried no controls at all, so an account that was
signed out - or one that had only just been added - trapped the window.

The page now has a header: an account switcher (the same control the sidebar
uses, so any other account is one click away) and a back arrow to the account that
was open before. Profile switches route through `window.switchProfile()`, which
records the outgoing profile. The arrow appears only when that profile still
exists and is not the one being linked.

The page was restyled at the same time - green pill buttons and the app's own
field, in place of the platform button and text field.

29/29 desktop tests pass.

### Found while doing the above

- **The filter bar's `+` was a blank circle.** `ChatFilterBar` asks `TintedIcon`
  for `plus.svg`, and `TintedIcon` had no `plus` case, so the chip drew its
  outline and nothing inside. Auditing every `icons/*.svg` referenced in QML
  against the kinds `TintedIcon` actually draws turned up one more blank,
  `info.svg` on the message hover row. Both are drawn now, and the audit is a
  two-line shell check worth repeating whenever an icon is added.
- **A dropped daemon connection sent a paired account to the QR page.** The
  daemon is right - `LoggedIn` only clears on `events.LoggedOut`, never on
  `Disconnected` - but the client starts with an empty status, so `loggedIn` is
  false until the first `connection.changed` arrives, and `PairingPage` was
  bound to nothing more than `!backend.loggedIn`. Losing the daemon therefore
  looked exactly like never having linked. The QR page now waits for
  `backend.daemonConnected` *and* a non-empty `status.state`; until then the
  window shows a connecting indicator. This is why the reported screen appeared
  at all - the exit added above is the mitigation, this is the cause.
- Text dialogs focus their field on open, as WhatsApp Web does.
- `--smoke-test` now asserts `pairingBackButton` exists and is *not* visible in a
  fresh window. During one live sighting the arrow showed while `previousProfile`
  should have been empty; the daemon reconnected before that could be traced, so
  the cause is not established - `canGoBack` guards it and the assertion will
  catch a regression at startup.
- `find_package(KF6Kirigami)` and the `KF6::Kirigami` link stay for now: no QML
  imports it, but the Qt Quick Controls style is still `org.kde.desktop` and
  unlinking is a separate change with its own visual pass.

Checked in the dark theme as well: dialog card, muted fields, focus ring and the
radio dot all read correctly on `Theme.surface`.

## 2026-09-04 — Media from all chats

Compared side by side with the PWA's own browser. Ours was a text list of grey
squares; the PWA is a grid of thumbnails grouped by day.

**The thumbnails were never going to load.** `MediaLibraryPane` asked for
`"data:image/jpeg;base64," + media_thumbnail`, but `media_thumbnail` is a file
path everywhere else in the client (`MessageDelegate`, `ContactInfoDrawer`).
Every tile fell through to the placeholder. Fixed by using the same `localUrl()`
the contact drawer uses.

**Each item is labelled with the chat it came from**, as in the PWA - not the
sender, which is what we showed. `sharedMessages` now joins `chats` and returns
`chat_title`; the join carries the per-chat browser too, harmlessly.

Rebuilt to match the PWA:

- A grid of square tiles, roughly 300px, cropped, 2px gutters, instead of 64px
  rows.
- Grouped by day, each group headed by the relative day and the full date -
  a `ListView` of days each holding a `Grid`, because a `GridView` carries no
  section headers.
- The conversation's name over a gradient at the bottom-left of every tile.
- Videos carry a play badge and, when the duration is known, its running time.
- The header now reads as the PWA's: title and subtitle on the left, the three
  categories as centred underlined tabs (not chat-list pills), and search, sort
  and close on the right. Search filters the loaded page locally rather than
  making a round trip per keystroke; sort flips newest/oldest; close returns to
  the chats section.
- Documents and links keep their row layout, as they have in the PWA.

New `TintedIcon` kinds: `play`, `sort`. `--section media` is now accepted on the
command line, which is what makes this screen photographable at all.

Still missing against the PWA header: the multi-select checkbox, which has no
bulk action behind it here yet.

29/29 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — The chat menu is complete, and accounts can be removed

### Add to Favorites and Clear chat are no longer blocked

Both were written up in earlier rounds as protocol-blocked because whatsmeow has
no builder for them. It has the pieces, though - `appstate.IndexFavorites`,
`IndexClearChat`, `waSyncAction.FavoritesAction`, `ClearChatAction` - so the
patches are now built by hand:

- `SetChatFavorite` reads the current favourites out of the store, edits the
  list and sends it whole. Favourites are not a per-chat flag in app state: the
  entire list travels as one mutation, so sending only the change would clear
  every other favourite on the phone.
- `ClearChat` sends a `clearChat` mutation keyed on the last message, the way
  `BuildDeleteChat` keys the delete, then empties the local rows while leaving
  the chat in the list. New store method `ClearChatMessages`.

New RPCs `chat.favorite` and `chat.clear`. The row menu and the conversation
menu both carry the entries, `Clear chat` behind a destructive confirmation.
With these the chat row menu matches the PWA's entry for entry:

    Archive · Mute▸ · Pin · Mark as unread · Add to Favorites · Add to list▸
    ─ Block · Clear chat · Delete chat

`Block` now uses the crossed-circle glyph in the conversation menu too.

### Disappearing messages

`SetDisappearingTimer` was there in whatsmeow all along. The conversation menu
now offers Off / 24 hours / 7 days / 90 days, the timer is stored per chat
(`chats.disappearing_seconds`), and the dialog opens on whatever the chat
currently has, so saving without touching anything changes nothing. New RPC
`chat.disappearing`.

### Removing an account

Requested: the account switcher could add and rename accounts but never remove
one. Each row now carries a second, red action beside the pencil.

`RpcClient::removeProfile` leaves the account first if it is the open one, stops
its daemon, drops its monitor, removes its socket, deletes its data and cache
directories, and forgets its settings. Two accounts are never removable: the
first one, which owns the shared data directory that every other profile lives
under, and the last one left. The profile name is validated against the daemon's
own `^[a-z0-9][a-z0-9_-]{0,31}$` before it is ever turned into a path, so a
crafted name cannot reach outside the application's directories.

Because it deletes files, `desktop-profile-removal` covers it: the test makes an
account in its own XDG directories, writes a file into it, checks that removing
the first account is refused, then removes the account it made and asserts both
the profile and its directory are gone.

30/30 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — Export chat, media selection, and what the protocol still refuses

### Export chat

The conversation menu can now write a transcript, the way the PWA's "Export
chat" does. `Store.ExportChatTranscript` walks the conversation oldest-first and
streams one line per message in WhatsApp's own shape -
`[dd/mm/yyyy, hh:mm:ss] Author: text` - rather than collecting it: a long
conversation is tens of thousands of rows, and joining them at the end would
cost more than the file. Attachments are named, not copied.

`Client.ExportChat` requires an absolute path and creates the file `0600`: a
transcript is as private as the conversation it came from. The path comes from
the reader's own save dialog, pre-named after the chat.

Verified end to end against the live account: 373 lines, mode 600.

### Media browser selection

The PWA's fourth header action is a checkbox, and it now does something here:
selecting tiles shows a bar with the count, a forward action and a cancel. The
forward reuses the existing dialog.

Forwarding from that browser needed a new client call: `forwardMessage` assumed
the open conversation as the source, which is wrong for a browser that spans
every chat, so `forwardMessageFrom(fromChatJid, messageId, toChatJid)` carries
the source with the message and `forwardMessage` now delegates to it.

`Add to Favorites` is in the conversation menu too, not only the row menu.

### Verified against the live account

- **Favorites**: set and cleared through `chat.favorite` on a real chat; the
  server acknowledged both patches and the state survived a daemon restart. The
  chat was returned to how it was found.
- **Clear chat**: implemented but *not* verified live. The only disposable chat
  on this account is the reader's own "message yourself" chat, which holds their
  notes; clearing it to prove a patch is not a trade worth making. The patch is
  built the same way as the delete patch that is known to work, but that is an
  argument, not a test.

### Still open, and why

- **Report**: whatsmeow exposes no report/spam call at all - only
  `reportingtoken.go`, which attaches tokens to outgoing messages. There is no
  client method to wrap, so this one is genuinely blocked rather than unbuilt.
- **Voice and video calls**: not carried by this protocol.
- **Meta AI**: server-side product, nothing to talk to.

30/30 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — Settings

The largest remaining gap in the control inventory was section 12: the PWA has a
settings tree, and WhatsAppGo rendered Account, Privacy, Chats and Notifications
as rows with nothing behind them. `SettingsPane.qml` replaces the placeholder,
built the way the PWA builds it - a list of sections that drills into one page at
a time, with a back arrow.

Only settings this client can actually carry out are offered. A row that does
nothing is worse than no row, which is also why two candidates were dropped
after being written: a notifications page (the daemon takes its notification
setting at spawn, so a toggle would not have applied) and an automatic media
download switch (nothing reads it).

**Privacy is real.** whatsmeow exposes `GetPrivacySettings` and
`SetPrivacySetting`, so Last seen and online, Profile photo, Status, Read
receipts and Groups are read from the account and written back to it. Each
setting's accepted values are checked here before the request goes out - WhatsApp
answers a bad value with an opaque error that names nothing - and a rejected
change is followed by a read, rather than leaving a stale answer on screen.
Blocked contacts get their own page off Privacy, with unblock on each row.

Verified against the live account: `privacy.get` returns
`last_seen=contact_blacklist, online=match_last_seen, profile_photo=all,
status=all, read_receipts=all, group_add=all`, and the page shows exactly that
alongside the 26 blocked contacts.

**Profile** shows the account name and number and edits the About text through
`SetStatusMessage` (139 characters, WhatsApp's own limit). The name itself is
labelled as belonging to the phone rather than being presented as editable -
the account-switcher pencil edits a local alias, and the inventory warns against
confusing the two.

**Chats** carries the theme and "Enter is send". The composer honours it: with
the setting off, Enter opens a line and Ctrl+Enter sends, as in the PWA.

**Keyboard shortcuts** opens a dialog. The shortcuts in it are declared in
`Main.qml` and work - Ctrl+F, Ctrl+Shift+F, Ctrl+N, Ctrl+Shift+N, Ctrl+E,
Ctrl+Shift+M, Ctrl+Shift+U - rather than being copied from the PWA's list. The
send/newline rows follow the "Enter is send" setting.

New RPCs: `privacy.get`, `privacy.set`, `profile.set_about`. New command-line
argument `--settings-section`, which is what makes each page photographable
without driving the window.

30/30 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — Status posting and channel actions

### Status can be posted, not only read

Inventory section 8 recorded "Add Status and Status menu are visible but
non-functional". Both work now.

WhatsApp treats a status as a message addressed to the status broadcast, and
whatsmeow works out the recipients from the account's own status privacy, so
nothing here builds a recipient list. `PostTextStatus` sends an
`ExtendedTextMessage` with one of WhatsApp's packed-ARGB backgrounds;
`PostMediaStatus` goes through the ordinary attachment path, addressed to the
broadcast rather than to a conversation. New RPC `status.post` takes either.

The `+` button opens the PWA's own two choices - Photos and videos, Text - and
the status menu offers Status privacy, which jumps to the privacy page rather
than duplicating the control.

The text dialog shows the chosen background behind the text as it will look,
with WhatsApp's five colours. Its hint is drawn as a label rather than through
`placeholderText`: the desktop style renders the placeholder in a colour that
cannot be read on these backgrounds.

**Not verified live.** Posting a status shows it to every contact who can see
the account's status; that is not something to do to prove a code path. Say the
word and it goes out.

### Channels

A followed channel's row now carries mute and Leave, through whatsmeow's
`NewsletterToggleMute` and `UnfollowNewsletter`. New RPCs `channel.mute` and
`channel.follow` (the follow direction is there for a discovery surface that
does not exist yet).

Channel discovery, creating a channel, and community creation remain absent:
they need a browse-and-search surface that has no backing here yet, which is a
build rather than a gap to close in passing.

30/30 desktop tests, `go vet` and Go tests clean.

Found while adding the channel row actions: a channel description carries its own
line breaks, and rich text honours them whatever the label's elide and line-count
say, so the row grew past its 84px and the description ran underneath the new
buttons. The row collapses the description's whitespace before showing it.

## 2026-09-04 — Channels, communities, invite links, own reactions

### Creating and finding

Inventory sections 9 and 10 recorded that WhatsAppGo only displayed channel and
community records: no creation, no navigation. Four operations now exist:

- `channel.create` — `CreateNewsletter`, from the Channels header.
- `channel.follow_link` — the invite is read with `GetNewsletterInfoWithInvite`
  before `FollowNewsletter`, so the confirmation can name what was followed.
- `community.create` — a community is a group with the parent flag set, and
  WhatsApp adds its announcement group itself.
- `group.join_link` — `JoinGroupWithLink`, offered from the sidebar menu, and the
  group opens once joined rather than only appearing in the list.

Both link fields accept the whole link or the bare code: WhatsApp's share sheets
hand out the link, and asking someone to trim it is a way to make them get it
wrong.

**The PWA's channel Discover cannot be reproduced.** It is a server-side
directory with no client API in this protocol, so a link is how a channel is
found from here, and the dialog says so rather than implying a search exists.

Error paths checked against the live account, no side effects: an empty
community name is refused locally (`a community name is required`), a link with
no code is refused locally (`that link carries no invite code`), and a bogus
channel code reaches the server and comes back attributed
(`look up channel: graphql error: 400 Bad Request (CRITICAL)`).

Creating a channel or a community was **not** run live: a channel is public and a
community is not something to conjure on someone's account to prove a call
works.

### Own reactions are marked

Inventory section 6: "WhatsAppGo optimistically writes reactions and aggregates
counts, but does not clearly mark self-selection." The badge now takes the
primary colour and border when one of its reactions is the reader's own,
compared on the JID's user part because the same person arrives as a phone JID
or a LID depending on the chat.

30/30 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — The contact drawer carries its actions

Inventory section 13 listed what the PWA's drawer offers and recorded that "the
remaining structural sections and destructive/moderation branches are absent".
Everything that has a backend now has a row:

- **Add/remove Favorites** - was a disabled row captioned "Managed by WhatsApp",
  which stopped being true once `chat.favorite` existed. It works now.
- **Starred messages**, **Disappearing messages** (showing the chat's current
  timer), **Export chat**, **Clear chat**, **Block/Unblock**, **Delete chat**.

Block is kept out of a group's drawer, since a group has nobody to block. The
closing note no longer claims blocking and disappearing messages are unavailable
- it names only what really is: reporting and calls.

`desktop-contact-info` asserts each row by name, because a row that quietly
stopped being built would otherwise read as a shorter drawer rather than as a
missing feature. It also checks Block appears for a contact and not for a group.

Two things learned writing that test, both worth remembering:

- QML's `visible` reports *effective* visibility, so a drawer built without a
  window reports every row invisible however its own binding decided. The row
  exposes its decision as `blockable` and the test reads that.
- Mutating `info` on a live drawer did not re-evaluate the row. The group case
  builds a second drawer instead, which is both simpler and a truer question:
  it asks what the drawer builds for a group.

30/30 desktop tests, `go vet` and Go tests clean.

## 2026-09-04 — Sidebar search

### The query did not survive a refresh

Search appeared not to work at all. The field passed its text straight into
`RpcClient::refreshChats(query)`, but nineteen other call sites in
`rpcclient.cpp` invoked the same method with no argument — a periodic refresh, a
`chat.updated` event, and the completion callback of nearly every settings
mutation. Each of those sent `chats.list` with an empty query and replaced the
list, so results appeared for a moment and were then wiped by whatever refreshed
next. The backend was never at fault: `chats.list` and `messages.search` both
answered correctly when called directly through `whatsappctl`.

The client now remembers the query. `refreshChats()` takes no argument and
always fetches the unfiltered list — `m_chats` still backs the unread total and
the new-chat picker, so a search must not narrow it — while `searchChats(query)`
stores the query and fetches the three result groups into lists of their own.
A refresh replays the search rather than clearing it. Each reply is dropped when
the field has moved on, so a slow request for an earlier keystroke cannot win
the race.

### The result groups

The PWA replaces the chat list with `Chats`, `Contacts` and `Messages`, each
under a heading, and tints the matched run green inside every row. WhatsAppGo
filtered the chat list in place and put message hits behind a separate dialog in
the app menu.

`SearchResultsPane.qml` now carries all three groups in one list, with a heading
only where a group has hits and a single empty state otherwise. Chat rows reuse
`ChatListDelegate`, so a result looks exactly like the row it opens. Contact
rows carry a name and nothing else, as the PWA shows them. Message rows have no
avatar: the chat name heads the row with its date, and the matched line sits
below with the receipt ticks when the message was outgoing. Clicking a message
result opens the chat and jumps to that message.

Supporting changes: `messages.search` now returns `chat_title` and skips status
and newsletter posts, which the PWA also keeps out of these results; a new
`contacts.list` reads address-book entries that have no conversation yet, so a
person is never listed under both Chats and Contacts.

### Archived chats belong in the results

Checked against WhatsApp Web directly: a query for an archived conversation's
name returns it under `Chats`, marked with an `Archivado` badge, rather than
hiding it until the reader opens the archived list. WhatsAppGo searched only the
unarchived shelf.

`chats.search` now spans both and reports each result's archived state, while
`chats.list` keeps the shelves apart as before. A search result carries an
`Archived` badge, shown only when the row stands for a search hit.

### Latency

Measured against the live daemon, every query is fast: `chats.list` with a
limit of 200 takes 8-10 ms, `chats.search` and `contacts.list` 4 ms, and
`messages.search` 27-42 ms. The wait was not in the database.

Three things caused it. The keystroke debounce was 250 ms and is now 120 ms.
Chat-list refreshes replayed the live query one-for-one, and refreshes arrive in
bursts, so the replay is coalesced through a single-shot timer. Most of all,
`rpc.Server.serveConn` handled requests one at a time per connection, so a
search waited behind whatever a scrolling list had already asked for - avatar
fetches, media, a history page. Requests are now handled off the read loop with
at most eight in flight; the desktop matches replies to requests by id, so the
changed order costs nothing. `Service.Handle` holds no state between calls, and
the store serialises on its single SQLite connection, so nothing there needed a
lock.

The Chats group no longer waits for the daemon at all: `searchChats` matches the
chats already loaded and publishes them on the keystroke, and the daemon's
answer - which also reaches the archived shelf - replaces that a moment later.
Searching before the daemon is up is silent rather than raising three "not
connected" errors per keystroke; the query replays once the connection lands.

### Known difference

The PWA matches names loosely — a query of `hola` surfaced `Gal Golan` and
`Hila Halamit`. Ours matches substrings. Closing that would mean reproducing an
unpublished ranking function, and it is left alone deliberately.

### Still duplicated

`Search messages` remains in the app menu and in the conversation header. In the
PWA the header's search is scoped to the open conversation and opens a panel on
the right, not a global dialog. The sidebar now covers the global case; the
in-conversation panel is still missing.


## 2026-09-05 — Removing an account crashed the process

Deleting an account killed the application. The first symptom recorded was
`malloc_consolidate(): unaligned fastbin chunk detected`; a self-installed
handler later caught the fault as `activateTimers` to `notifyInternal2` to
nothing, which is a timer delivered into freed memory.

Bisecting it took four cuts. Removing an account through `RpcClient` alone was
clean. Removing it after the confirmation dialog had been opened crashed, and
still crashed when the dialog was closed again first, so the dialog was not the
cause - it was the animation clock that opening a popup starts, which is what
makes a dangling timer registration fatal instead of merely wrong. Emptying the
account switcher's `Repeater`, dropping `Overlay.modal` and suppressing the
completion notice each changed nothing. Setting
`WHATSAPPGO_DISABLE_PROFILE_MONITORS=1` made the crash disappear entirely.

`ProfileMonitor` was the cause. `removeProfile` scheduled it with `deleteLater`,
which keeps the object alive until the event loop returns. In that window its
`QLocalSocket` was still connected to the daemon the same function was about to
terminate, and its reconnect and refresh timers were still armed. The socket's
`disconnected` and `errorOccurred` handlers restarted the reconnect timer on an
object already queued for deletion.

`ProfileMonitor::shutdown` now stops both timers, drops the socket's own signal
connections and aborts it, and `removeProfile` calls that before `deleteLater`.
Three other defects found while reading the same function are fixed with it: the
account now leaves the list before anything is torn down, so every handler that
runs during teardown sees it as already gone; `startBackendForProfile` and
`ensureProfileMonitor` refuse an account that is not in the list, which stops a
deleted account's daemon being started again; and the blocking
`waitForFinished` is gone, replaced by the daemon deleting itself when it exits,
with the account's files removed at that point rather than while the daemon
still holds them open.

### Reproducing it

The crash needs a monitor that has actually connected, so it needs a live
daemon:

```
whatsappgo --profile scratch --screenshot /tmp/shot.png --remove-account scratch
```

run with `WHATSAPPGO_DISABLE_PROFILE_MONITORS=0`, a settings file naming two
accounts, and a short `XDG_RUNTIME_DIR` - a long one exceeds the 107-byte limit
on a unix socket path and the daemon never binds.

`desktop-profile-removal-monitored` covers the same teardown with monitors
constructed, but it points `WHATSAPPGO_BACKEND` at a program that exits at once
so the suite leaves no daemon behind. That also means its monitors never reach a
live socket, so the test does not reproduce the crash. It was confirmed to pass
both with and without the fix.

## 2026-09-05 — The account menu opened outside the window

On the pairing page the account menu ran off the right edge of the window.
`AccountSwitcherButton` clamped the menu only when a caller passed
`popupParent`, and `PairingPage` passed none, which left `x: 0` relative to a
44-pixel button for a 244-pixel menu. The clamp now falls back to the window's
own content item, so it applies wherever the button is used, and it clamps the
vertical edge as well.

The same page showed a delete control that did nothing: `PairingPage` never
carried `removeRequested` out to the window. It does now, and reaches the same
confirmation the main window uses.

A layout test asserted `accountMenu.x == 0`, which had recorded the unclamped
behaviour as correct. It now asserts the menu stays within its surface.


## 2026-09-05 — Link previews failed, loudly

A YouTube link in the composer put this in front of the reader, as a toast wide
enough to cover the composer:

```
Get "https://www.youtube.com/oembed?...": net/http: HTTP/1.x transport connection
broken: malformed HTTP response "\x00\x00\x12\x04...http2_handshake_failed"
```

Two separate defects.

`RefreshLinkPreview` returned the resolver's error, which the service turned
into `request_failed` and the desktop into an error toast. A preview that cannot
be fetched is a normal outcome for a link in a message - the site may be down,
may block us, or may not describe itself - and `link.preview` already treated it
that way. The refresh path now returns the message unchanged.

The fetch itself failed for a reason worth recording. The preview client was
pinned to HTTP/1.1 on purpose, because a hostile server can hold a client
goroutine on HTTP/2 control frames while no request is outstanding, which the
request timeout does not cover. On this network that no longer works: hosts
answer an http/1.1-only ALPN offer with an HTTP/2 GOAWAY whose debug data reads
`http2_handshake_failed`. Reproduced directly - the default transport gets
`200 OK proto=HTTP/2.0`, and both the legacy `ForceAttemptHTTP2 = false` form
and the modern `Protocols.SetHTTP1(true)` form fail identically, so this is the
network, not Go's configuration.

HTTP/2 is now negotiated with its framing bounded instead of trusted:
`SendPingTimeout` and `PingTimeout` close a connection that stops answering,
`WriteByteTimeout` closes one that stops reading, and `MaxReadFrameSize` caps
what a single frame can make the client buffer. Every other protection is
unchanged - no proxy, dialling only public addresses, ports 80 and 443 only, a
redirect cap, an overall request timeout and a body size cap.

This relaxes a deliberate security decision, so the test that recorded it was
rewritten rather than deleted: `TestPublicClientKeepsAProxyOutAndBoundsHTTP2`
now asserts the four bounds above.


## 2026-09-05 — Searching inside a conversation

Compared against WhatsApp Web directly. Its `Buscar` in a chat header opens a
panel on the right of the window titled `Buscar mensajes`: a close button, a
search pill, and hits listed newest first as a timestamp above the matched line,
with the match tinted green, the receipt ticks on outgoing messages, and a
document icon and filename for file messages. The conversation narrows to make
room. Nothing covers the chat.

WhatsAppGo instead opened a modal dialog that searched every conversation, which
is neither what the control says nor what the PWA does, and duplicated the
sidebar's own Messages group.

`ChatSearchPanel.qml` now sits beside the conversation and carries that layout.
`messages.search` takes an optional `chat_jid`, so the panel is scoped to the
open chat while the sidebar keeps searching everywhere. A hit opens at its
message through the existing jump.

Two things came out of the comparison. The PWA matches document filenames -
searching `cuenta` returned `Extracto_202512_Cuentas_de_ahorro_3052.pdf` - so
the query now matches `media_name` as well as `body`, which improves the
sidebar's Messages group too. And the modal dialog is gone, along with the
`Search messages` entry that duplicated the sidebar field in the app menu; the
conversation header, the conversation menu, the contact drawer and the keyboard
shortcut all open the panel instead.

## 2026-09-05 — Menu rows were not vertically centred

Reported against the account menu: the labels sat high in their rows and the
rows did not match the other menus. A `RowLayout` stretches its children to the
row's height and a `Label` draws its text at the top of whatever height it is
given, so every label in `WhatsAppMenuItem` was top-aligned. Each child now
states `Layout.alignment: Qt.AlignVCenter` and each label
`verticalAlignment: Text.AlignVCenter`.

The account rows also carry 32-pixel edit and remove buttons inside a 36-pixel
row, which left them pressed against its edges. A row with those buttons is now
44 pixels; a plain row stays at 36. The buttons stay at 32, which a layout test
already required as a pointer target and which the first attempt at this fix
broke by shrinking them to 28.

## 2026-09-05 — Kirigami's build dependency removed, and the port off Linux

Correcting the note above that said `find_package(KF6Kirigami)` and the
`KF6::Kirigami` link would stay: they were dead. Grep found Kirigami only in
comments, so both lines are gone from `desktop/CMakeLists.txt`,
`ldd whatsappgo | grep -i kirigami` is empty, and the UI renders unchanged.
Every install document, packaging manifest and the dependency check script
dropped the Kirigami requirement with it; `qml6-module-org-kde-desktop` stays,
because the Qt Quick Controls style is a separate package.

The application now builds for Windows and macOS as well. See
[CROSS_PLATFORM.md](CROSS_PLATFORM.md) for the whole account. The parts that
touch how the application looks:

- The Qt Quick Controls style is `org.kde.desktop` on Linux and Fusion on
  Windows and macOS. Almost every control is drawn by this project's own QML
  against `Theme.qml`, so a native style would restyle only the few primitives
  underneath — exactly the ones that would then stop matching WhatsApp. Fusion
  keeps all three platforms on one design.
- Notifications are drawn by the freedesktop service on Linux and by the
  application's own tray icon elsewhere, decided by a `handled` flag on the
  `notification.received` event so a message is never shown twice.

One bug found while doing it, unrelated to the port but visible in the same
place: `RpcClient::startBackendForProfile` deleted the daemon socket whenever a
connection was refused, on the assumption that a refusal proves the daemon is
gone. It does not — a full listen backlog refuses too — and unlinking a bound
Unix socket leaves the daemon running on a path nothing can reach. Thirteen
orphaned daemons had accumulated. The client now probes before clearing the
path, and the daemon stops on its own when its socket is taken away.
