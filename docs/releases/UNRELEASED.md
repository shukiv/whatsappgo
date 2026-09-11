# Unreleased changes

## Desktop reliability fixes — 2026-09-11

These changes are implemented in the local development worktree, after the
`v0.1.8` source snapshot. They are not included in the existing v0.1.8 draft
artifacts. No new version has been assigned or published for this batch.
Build from the reviewed source containing these fixes and restart that build
to use them; rebuilding does not replace an already running process.

### Ten confirmed bugs fixed

- **Group mentions:** converting emoticons around a tag preserves its recipient
  identity. One Undo restores the conversion without removing the tag.
- **Message editing:** a failed save keeps the correction and an inline error
  in the editor. Save is disabled while pending; success closes the dialog.
  A late response cannot close a newer editing session.
- **Settings:** older preference/notification responses no longer overwrite
  newer update events. Settings remain usable after a save completes.
- **Clipboard:** a newer image or text copy supersedes an older pending image
  download, including its late error or media-update event.
- **Media recovery:** failed automatic chat/status downloads can be requested
  again after a two-second cooldown. This is retry eligibility, not an unlimited
  automatic retry loop or a guarantee that expired media can be recovered.
- **Starred results:** remote unstar events remove the item from loaded results;
  an older in-flight snapshot cannot restore it.
- **Confirmation safety:** chat-scoped clear/delete, message delete, pin,
  disappearing-message, forward and edit dialogs dismiss on chat/account changes
  and reject stale acceptance. Already submitted requests are not cancelled.
- **Status replies:** refreshing the same status, including its media metadata,
  preserves the reply draft and pending state. Moving to another status/account
  resets that state; drafts are not a persistent outbox.
- **Media library:** a reload invalidates pending older pages, preventing stale
  append responses from duplicating reloaded results. Account changes clear the
  library; overlapping same-category append requests are ignored.
- **Read receipts:** initial-page and incoming-message receipts are deferred
  while the desktop conversation is not active and visible. Returning to it
  acknowledges the loaded conversation. This is not per-bubble visibility
  tracking and does not override reads made on another linked device.

Earlier audit fixes pending alongside this batch retain failed poll choices
for retry, close stale video viewers when their source changes or disappears,
and stop active or pending playback when switching accounts.

### Navigation icon requests

- **Rail navigation:** opening the media library, or any destination the feature
  panel does not draw, no longer asks the icon provider for artwork that is not
  bundled. The panel now maps its own sections to their bundled icon names
  instead of deriving a filename from the selected section. Status, Calls,
  Channels, Communities and Profile keep their existing empty-state artwork.
  This removes a transient console warning; no visible icon changed.

### Audit findings — 2026-09-11

Six defects found while reviewing the application, with their regressions:

- **Sending:** `message.send`, `message.send_media`, `message.send_contact`,
  `message.forward` and `sticker.send` refuse the status broadcast address and
  other addresses that are not conversations. A status update reaches a client
  as an ordinary message whose chat is that address, so a program answering the
  chat a message came from could publish a status update to its contacts.
  Publishing a status remains `status.post`, which asks who may see it.
  Replying to a status is unchanged: the reply is addressed to the person.
  Channel addresses are refused too; this client has no method for posting to a
  channel, only for following, muting and creating one.
- **Notifications:** a message the desktop notification service refuses is now
  handed back to the window, which presents it itself. A server whose queue is
  full refuses every client, and the daemon had already reported the
  notification as presented, so nothing appeared at all. This needs a system
  tray to present the replacement, and on Linux the tray balloon is drawn by the
  same notification service, so a saturated server may still show nothing. The
  message is always readable in the window.
- **Notifications:** message text and sender names are escaped for a
  notification server that advertises `body-markup`. Text from other people was
  being parsed as markup, so a body containing `<` or `&` could be mangled, and
  tags could become formatting, links, or an image the computer fetches.
- **Alerts:** the alert-deduplication table now drops its oldest entry instead
  of refusing new alerts once it is full. A run of status updates could
  otherwise silence incoming-call and security-code alerts for five minutes.
- **Starred messages:** starring several messages reads the list at most once a
  second instead of once per message, and a refresh no longer empties the list
  on screen first.
- **Navigation:** the feature panel no longer asks the icon provider for
  artwork that is not bundled while the media library is selected.

### Validation and limits

All ten original reproductions failed before the fixes. Afterward, 33 targeted
native checks passed in each of light and dark mode, all 44 desktop CTests
passed, and `go test ./...` / `go vet ./...` passed. Earlier poll/playback and
video fixtures also passed. Light/dark failed-edit screenshots were inspected. The
navigation-icon repair was reproduced separately, then verified by 10 checks in
each mode with the icon provider's warnings captured, and by a mutation that
restored the old filename and failed those checks again. The six audit findings
add Go regressions for the refused destinations, the export guard, the alert
table, the notification escaping and the delivery fallback, and a native
fixture for the starred-list bursts; each was checked with a mutation that
restores the old behaviour and fails its test.

These regression checks used the actual Qt UI with an isolated synthetic RPC
backend, not a fresh live WhatsApp Web comparison. They made no live account
changes and added no repository test infrastructure. See the
[audit record](../WHATSAPP_WEB_PWA_GAP_AUDIT.md#desktop-reliability-regressions--2026-09-11-unreleased)
for issue IDs and local evidence locations. This batch does not add new calling,
history retrieval, library-wide sorting or bulk deletion capabilities.
