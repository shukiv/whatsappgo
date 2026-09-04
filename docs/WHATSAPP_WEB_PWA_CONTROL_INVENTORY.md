# WhatsApp Web PWA control inventory

Reference captured: 2026-09-03  
Reference client: installed, authenticated WhatsApp Web PWA  
Comparison target: WhatsAppGo desktop client in this repository

Comparison status updated: 2026-09-03 after compact geometry, responsive filter,
drawer sizing, scroll preservation, and popup-boundary fixes. The inventory
continues to list reference controls that are not yet implemented.

## Purpose and evidence levels

This is the control-by-control companion to the parity gap audit. It records menus, submenus, drawers, dialogs, hover controls, selection modes, and state-dependent actions instead of treating a screen as covered merely because it was opened.

Evidence labels used below:

- **Opened** — the control was activated and the resulting screen, menu, drawer, or dialog was inspected.
- **Confirmed, then cancelled** — the destructive or external action was followed to its confirmation boundary and cancelled.
- **Observed** — the control and its state were visible, but committing it would alter the account or contact another person.
- **Unavailable** — the account, media sample, or test machine did not provide the state needed to complete that branch.
- **Screenshot-confirmed** — supplied reference evidence shows a state which was not safely reproducible during the automated pass.

No message, reaction, status, channel, community, call, block, report, deletion, logout, follow, or settings mutation was committed during this audit. File pickers, calls, and external-navigation actions were stopped before their irreversible or externally visible step.

## Coverage summary

| Surface | Coverage | Important branches reached |
|---|---|---|
| Navigation rail | Opened | Chats, Calls, Status, Channels, Communities, Meta AI, global Media, Profile/settings |
| Chats sidebar | Opened | filters, archived, new-chat pane, app menu, row menu, multi-select mode |
| Conversation | Opened | header controls, overflow, composer, attachment menu, hover actions, message menus |
| Message states | Opened / screenshot-confirmed | incoming/outgoing, text/image/voice/link/quoted, old versus edit-eligible outgoing text |
| Calls | Opened | dial number, new-call selector, add favorite, call-link modal, call-number panel, permission failure |
| Status | Opened | privacy menu, status creation menu, status list/viewer entry points |
| Channels | Opened | discovery, region/search, channel creation intro and form |
| Communities | Opened | creation intro and editable form |
| Global Media | Opened | Media, Documents, Links, search, sort/select entry points |
| Profile/settings | Opened | every first-level row, privacy/chat/notification/account subpages, logout confirmation, shortcut dialog |
| Contact info | Opened | media/links/docs, favorite/mute, privacy/encryption and destructive rows |
| Media viewers | Opened / sample-limited | image viewer, voice controls, video entry points; some video/viewer menu states lacked a loaded sample |

## 1. Global navigation rail

The PWA rail contains the following controls in order:

1. Chats
2. Calls
3. Status
4. Channels
5. Communities
6. Separator
7. Meta AI
8. Media from all chats
9. You / profile and settings

All eight destinations were opened.

### WhatsAppGo comparison

| PWA control | WhatsAppGo | Gap |
|---|---|---|
| Chats | Present | Core destination exists |
| Calls | Data-only feature page | No favorite, dialing, call-link, or calling workflow |
| Status | Viewer/reply support exists | Add-status and status-menu controls are dead |
| Channels | Data-only feature page | No discovery, follow, open, or creation workflow |
| Communities | Data-only feature page | No open or creation workflow |
| Meta AI | Absent | Optional product decision, not a core parity blocker |
| Global Media | Absent | Missing cross-chat Media/Documents/Links browser |
| Profile/settings | Present as a shell | Most settings rows have no destination or behavior |

WhatsAppGo additionally puts Search, New Chat, and Theme in the rail. In the PWA, new chat is in the Chats header, search belongs to the relevant surface, and theme is under settings.

## 2. Chats sidebar

### 2.1 Header controls

- **App menu — Opened**
- **New chat — Opened**

PWA app menu:

1. New group
2. Starred messages
3. Select chats
4. Mark all as read
5. App lock
6. Log out

Subflows exercised:

- **New group — Opened:** contact search, selectable contacts, and the member-selection step were present.
- **Starred messages — Opened:** global starred-message destination.
- **Select chats — Opened:** checkboxes appear on chat rows. After selecting a row, the bulk menu contains Mark unread, Mute notifications, Archive chat, and Clear selected chats.
- **Mark all as read — Observed:** not committed because it changes all unread state.
- **App lock — Opened:** information page plus Enable app lock switch/button.
- **Log out — Confirmed, then cancelled:** confirmation offers Log out, Lock application, and Cancel.

WhatsAppGo's app menu currently contains Search messages, Add another account, Reconnect, Appearance, and Unlink this account. Those are useful WhatsAppGo extensions, but they do not implement the PWA command set.

### 2.2 Search, filters, and lists

Observed controls:

- chat search / start-new-chat field
- All
- Unread
- Favorites
- Groups when space permits
- list/filter overflow; this account exposed New list in the overflow
- Archived row and count

The filter strip is responsive: WhatsApp moves filters or user-created lists
into an overflow rather than removing the concept. WhatsAppGo now keeps All,
Unread, and Favorites direct, shows Groups directly at `440 px` and wider, and
moves Groups into the chevron overflow below that width. Its Unread label does
not absorb the unread total, and the filter strip precedes the Archived row.
List creation and user-created list filters remain unimplemented.

### 2.3 New-chat pane

Opened controls:

- Back
- international phone-number entry
- New group
- New contact
- New community
- self-chat row
- contact search
- contact results

WhatsAppGo supports contact selection and phone-number start. New group and New community currently report that the feature is unavailable; New contact is not equivalent to the PWA contact workflow.

### 2.4 Chat-row hover/context menu

The PWA shows a small dropdown affordance on hover. Right-click and the dropdown lead to the same action family:

1. Archive chat
2. Mute notifications
3. Pin / Unpin
4. Mark read / unread
5. Add / remove Favorites
6. Add to list
7. Block
8. Clear chat
9. Delete chat

WhatsAppGo has Archive, Mute, Pin, and read-state handling, but lacks the hover
dropdown and the remaining row commands. Its context menu now uses the compact
`238 px` width and `36 px` rows and is clamped inside the application window.

## 3. Conversation header and overflow

Header controls opened or inspected:

- contact/profile information
- video call
- voice call
- search in conversation
- overflow menu

PWA conversation overflow:

1. Contact info
2. Search
3. Select messages
4. Mute notifications
5. Disappearing messages
6. Add / remove Favorites
7. Add to list
8. Export chat
9. Close chat
10. Send call link
11. New group call
12. Report
13. Block
14. Clear chat
15. Delete chat

Destructive/external branches were only followed to their confirmation or selection boundary. WhatsAppGo currently has contact info and search in the header; it has no video-call, voice-call, or conversation-overflow structure.

### 3.1 Search mode

The PWA opens a dedicated in-chat search surface and preserves the conversation. WhatsAppGo opens a search dialog. The underlying search exists, but layout, result navigation, and selection/highlight behavior are not PWA-equivalent.

### 3.2 Select-messages mode

The PWA changes the conversation into an explicit selection state with message checkboxes and state-dependent bulk actions. WhatsAppGo has no equivalent conversation selection mode.

## 4. Composer and attachment branches

### 4.1 Composer controls

- Attach
- Emojis, GIFs, and Stickers
- message editor
- send when text exists
- voice-message recording when empty

The PWA emoji surface has separate emoji, GIF, and sticker modes plus search/category navigation. WhatsAppGo exposes an emoji picker, but GIF and sticker-library parity is incomplete.

### 4.2 Attachment menu

PWA order:

1. Document
2. Photos and videos
3. Camera
4. Audio
5. Contact
6. Poll
7. Event
8. New sticker

The Poll branch was opened. It contains:

- Question
- multiple option fields
- Allow multiple answers
- Hide voter names
- Set end time

File/camera branches were stopped at the picker/permission boundary. Event and sticker creation were identified but not committed.

WhatsAppGo displays all eight labels. Document, Photos/videos, and Audio are functional. Camera, Contact, Poll, Event, and New sticker only return an unavailable notice.

## 5. Message hover controls and action matrix

WhatsApp does not have one universal message menu. The visible action set depends on:

- incoming versus outgoing
- message age and edit window
- message type
- direct chat versus group/channel
- whether the message is revoked, starred, pinned, or already downloaded
- whether reporting is applicable

### 5.1 Hover controls

- message-options chevron: about `24 × 18 px`, transparent visual treatment
- quick React button: about `26 × 26 px`, compact icon with visible padding
- media-only Forward shortcut where applicable

WhatsAppGo now separates visual size from hit size: the options control renders
a `16 px` chevron on a transparent `32 px` target, and React renders a `26 px`
circle inside its `36 px` target. The quick tray is `44 px` high with `32 px`
cells. This preserves the reference visual weight and accessible hit areas.

### 5.2 Observed message-menu states

| State | PWA actions observed |
|---|---|
| Outgoing text, no longer editable | Reply, Copy, React, Forward, Pin, Ask Meta AI, Star, Delete |
| Outgoing text, edit-eligible | Reply, Copy, React, Forward, Pin, Ask Meta AI, Star, **Edit**, Delete |
| Incoming text | Reply, Copy, React, Forward, Pin, Ask Meta AI, Star, Report when applicable, Delete |
| Incoming image | Reply, Copy, React, Download, Forward, Pin, Ask Meta AI, Star, Report, Delete |
| Voice/audio | Reply, React, Forward, Pin, Ask Meta AI, Star, Report when applicable, Delete; playback and speed controls are separate |
| Outgoing message with receipts | Message info is available in the appropriate action/header path |
| Revoked/system message | Reduced action set; actions that require original content disappear |

Ask Meta AI is optional for WhatsAppGo. Forward, Star, download by media state, Delete-for-me, Message info, and correct conditional behavior are normal desktop messaging actions.

### 5.3 Edit behavior

Edit is not a permanent action on every outgoing text. It appears only for an eligible recent outgoing text message. Activating it enters an edit state with the original text prefilled and explicit accept/cancel controls. The keyboard-shortcut dialog also exposes **Edit last message**.

WhatsAppGo does implement Edit, but its menu condition is simply `from_me && kind === text && !revoked`. It does not enforce the PWA edit window or other server eligibility, and it uses a generic modal dialog rather than the PWA's composer-integrated editing treatment. This is **partial parity**, not a missing feature.

### 5.4 WhatsAppGo message-menu comparison

WhatsAppGo currently offers:

- Message info for every outgoing message
- Copy selected text
- Copy body
- Copy image
- Reply
- React
- Pin
- Edit for any outgoing non-revoked text
- Delete for everyone for any outgoing non-revoked message

The popup now uses the measured `196 px` width, compact `36 px` rows, and a
late-layout clamp so state-dependent final actions are not cut off near the
composer or window edge. The action set and predicates below remain incomplete.

Missing or incorrect state logic:

- no Forward
- no Star / Unstar
- no normal Delete-for-me path, including incoming messages
- no Report for eligible incoming content
- download is an inline media action rather than a matching conditional menu action
- Edit and Delete-for-everyone eligibility are too broad
- Message info availability and receipt rows need to follow actual delivery/media state
- menu order differs from the PWA
- current user's selected reaction is not visibly distinguished

## 6. Replies, quotes, reactions, pins, and receipts

- Clicking a quoted preview in the PWA jumps to the original message and
  highlights it. WhatsAppGo has a `quotedMessageRequested` path,
  pagination-aware jump logic, and an anchor-preservation regression test.
  Continue validating it on long live histories because pagination timing can
  still differ from the test model.
- PWA reactions expose the current user's selected reaction and allow replace/remove. WhatsAppGo optimistically writes reactions and aggregates counts, but does not clearly mark self-selection.
- PWA pin/unpin is stateful and may request a duration. WhatsAppGo has pin/unpin backend calls and a pinned-message banner, but menu state and duration/confirmation parity need checking.
- PWA outgoing message information separates Played, Read, and Delivered when applicable. WhatsAppGo has a message-info drawer and played/read/delivered timestamps; validate conditional rows rather than duplicating it.

## 7. Calls

Opened controls and branches:

- Dial number → phone-number/search panel
- New call → contact selector
- Add favorite → contact selector
- Start call → attempted; the test machine returned “No camera or microphone found,” then Got it
- New call link → modal with video toggle, Join call, Copy call link, and Send link via WhatsApp
- Call a number → phone-number panel

The PWA also presents Favorites and the call-history area. WhatsAppGo only displays synced records and explicitly states that placing calls is unsupported.

## 8. Status

Opened controls and branches:

- Status menu → Status privacy
- Add Status → Photos and videos, Text
- My status / status rows → viewer entry point
- individual recent status rows

Status privacy options:

1. My contacts
2. My contacts except…
3. Only share with…

WhatsAppGo can display status groups, view media, advance/back, pause, and reply. Add Status and Status menu are visible but non-functional. Status notifications are intentionally excluded, which matches the requested product behavior.

## 9. Channels

Opened controls and branches:

- owned/followed channel row
- Discover more
- channel search
- Select region
- Follow buttons were observed but not committed
- Create channel

Create-channel flow:

1. Intro explains public discoverability, identity visibility, responsibility, and guidelines.
2. Continue opens New channel.
3. Form contains Add channel icon, Channel name, Description, two emoji entry points, and Create channel.

WhatsAppGo only displays backend channel records. Search currently references an unresolved icon, and rows do not open a functional channel surface.

## 10. Communities

Opened controls and branches:

- Create new community
- Start community
- Back
- Start
- Add community image
- Set random photo
- emoji panel entry
- Community name
- description/rules field
- Create

Creation was stopped before Create. WhatsAppGo only renders associated community data; creation and navigation are absent.

## 11. Global Media

The PWA's rail destination opens cross-chat content with:

- Media tab
- Documents tab
- Links tab
- Search by sender or caption
- Sort by
- Select mode
- Close

WhatsAppGo has per-contact shared Media/Links/Documents, but no global cross-chat browser.

## 12. Profile and settings tree

Every first-level row was opened.

### 12.1 Profile

- profile photo edit
- name edit
- about/status-text edit
- username copy

WhatsAppGo's account-switcher pencil edits a local profile alias. It must not be presented as editing the WhatsApp profile name.

### 12.2 Account

- Security notifications
- Request account info
- How to delete account

### 12.3 Privacy

- Last seen and online
- Profile photo
- About
- Status
- Read receipts
- Default message duration
- Groups
- Blocked contacts and Add blocked contact
- App lock
- Block messages from unknown accounts, with More info
- Protect IP address in calls, with More info
- Disable link previews, with More info

### 12.4 Chats

- Theme
- Wallpaper
- Media upload quality
- Media auto-download
- Spell check
- Replace text with emojis
- Enter is send

### 12.5 Notifications

- Messages
- Groups
- Status
- Calls
- Show preview
- Outgoing message sounds
- Background sync

### 12.6 Keyboard shortcuts

The shortcut dialog was opened and includes chat navigation, unread/mute/archive/pin, search, new chat/group, add-to-list, profile/info, voice speed, settings, emoji/GIF/sticker panels, app lock, chat info/lock, reply/private reply, forward, star, attachment menu, voice recording controls, edit last message, and in-call camera/mute/reaction/hand/screen-share/end-call controls.

### 12.7 Help and feedback

- Help center
- Contact us
- Send feedback
- Terms and Privacy Policy
- Channel reports
- Join beta

### 12.8 Log out

Confirmation was opened and cancelled. It offers Log out, Lock application, and Cancel.

WhatsAppGo currently renders Account, Privacy, Chats, and Notifications as rows without implementations. Keyboard shortcuts, help/feedback, WhatsApp-profile editing, and the nested setting controls are absent.

## 13. Contact information drawer

PWA drawer branches observed across normal/business contacts:

- avatar/profile image viewer
- contact/profile edit where applicable
- about and phone/business information
- Call, Video, Search
- Media, links, and documents with category view
- Starred messages
- Add/remove Favorites
- Add to list
- Mute notifications
- Disappearing messages
- Advanced chat privacy
- Encryption information
- Export chat
- Clear chat
- Block
- Report
- Delete chat
- business-specific information/share actions when applicable

WhatsAppGo implements the avatar viewer, Search, per-contact shared content,
Favorites display, Mute, Archive, and an encryption placeholder. Its drawer now
matches the measured `540 px` width, `64 px` header, and `120 px` avatar.
Call/Video are visible but disabled. The remaining structural sections and
destructive/moderation branches are absent.

## 14. Media and viewer controls

Observed PWA behavior:

- image/video thumbnails open inside the app
- image viewer uses aspect-fit at 100%, with zoom controls and a filmstrip/caption area
- message/media menu provides Download when the original is available
- voice messages provide play/pause, seek/progress, duration/time, avatar, played styling, and playback-speed control
- playing audio/video updates played/read state
- outgoing media Message info can show Played separately from Read and Delivered

WhatsAppGo has an in-app image viewer with zoom, copy/save, and aspect-fit; Qt Multimedia playback; `markMediaPlayed`; and a message-info drawer. These are implemented features requiring format/quality/state regression coverage. Animated WebP stickers are still flattened to a first-frame fallback.

## 15. Geometry snapshot

Measured in the PWA at a `1007 × 686` viewport, DPR `1.25`:

| Element | PWA | WhatsAppGo | Status |
|---|---:|---:|---|
| Rail | `64 px` | `64 px` | Matched |
| Rail action | `40 × 40 px` | `40 × 40 px` | Matched |
| Sidebar/header | `64 px` | `64 px` | Matched |
| Filter strip | `42 px` high | `42 px` high | Matched |
| Filter chip | `32 px` high | `32 px` high | Matched |
| Archived row | `49 px` | `49 px` | Matched |
| Chat row | `72 px` | `72 px` | Matched |
| General menu row | about `36 px` | `36 px` | Matched |
| App/chat menu width | about `229–238 px` | `229 px` / `238 px` | Matched |
| Message menu width | about `195 px` | `196 px` | Within tolerance |
| Attachment menu | about `153 × 298 px` | `154 px`; eight `35 px` rows | Matched density |
| Message options visual | `24 × 18 px`, transparent | `16 px` glyph; transparent `32 px` target | Matched treatment |
| React visual | `26 × 26 px` | `26 px` visual; `36 px` target | Matched treatment |
| Contact drawer | `540 px` | `540 px` | Matched |
| Contact avatar | `120 × 120 px` | `120 × 120 px` | Matched |

Popup geometry is also boundary-aware: menus are clamped after their final
visibility-dependent layout so the last row and paired reaction tray remain
inside an 8 px window margin.

## 16. Environment-limited branches

The following were identified but not fully committed or could not be completed safely:

- real voice/video call connection: no camera/microphone and would contact another person
- Follow channel, create channel, create community, create poll/event/status: account mutation
- Mark all read, favorite/list changes, privacy/settings toggles: account mutation
- block/report/clear/delete/logout: inspected only to confirmation boundary
- file/camera picker completion: would select or transmit local content
- every group-admin-only or channel-admin-only menu: the authenticated account did not expose every possible role/state
- all disappearing-message timer outcomes: changing the timer would affect the conversation
- every video codec/viewer combination: the visible sample set was limited

These exclusions are explicit. They should not be mistaken for verified parity.
