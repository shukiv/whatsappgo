# Terminal client parity

`cmd/whatsapptui` is a second front end on the same daemon as the desktop
client: it speaks to `whatsappd` over the same Unix socket through
`internal/rpc.Client`, and follows the same event broker for live updates. It
owns no WhatsApp logic of its own, so a fix in the daemon reaches both clients
at once.

Built on Bubble Tea with Bubbles and Lipgloss. Nothing is released until every
method below is reached from the terminal, which is what parity with the
desktop client means here.

Media is never drawn in the terminal. A picture, a video, a sticker or a voice
note is shown as a labelled placeholder carrying its kind, name, size and
caption, and opening it hands the file to an external program - `xdg-open` for
pictures, video and documents, the system player for audio.

One box per method below: ticked when the terminal can reach it and a test
covers it.


## account (1)

- [ ] `account.logout` — Unlink this profile and delete its device session

## bugreport (2)

- [ ] `bugreport.environment` — Describe what a bug report would disclose
- [ ] `bugreport.submit` — Send a report to the whatsappgo project in Jabali Bugs Intake

## calls (1)

- [ ] `calls.list` — List synchronized call records

## channel (5)

- [ ] `channel.create` — Create a channel
- [ ] `channel.follow` — Follow or leave a channel
- [ ] `channel.follow_link` — Follow a channel from its invite link
- [ ] `channel.messages` — Read and store a channel's recent posts
- [ ] `channel.mute` — Mute or unmute a channel

## channels (1)

- [ ] `channels.list` — List followed channels

## chat (16)

- [ ] `chat.archive` — Archive or restore a chat
- [ ] `chat.avatar` — Fetch and cache a chat avatar
- [ ] `chat.clear` — Empty a conversation on every device without removing it
- [ ] `chat.delete` — Delete a chat on every linked device and locally
- [ ] `chat.disappearing` — Set the disappearing-message timer for a chat, in seconds (0 turns it off)
- [ ] `chat.export` — Write a conversation to a text file
- [ ] `chat.favorite` — Add or remove a chat from Favorites
- [ ] `chat.info` — Get contact metadata and shared-content counts
- [ ] `chat.label` — Add a chat to a list or remove it
- [ ] `chat.labels` — List the lists a chat belongs to
- [ ] `chat.mute` — Mute or unmute a chat; duration_seconds 0 mutes until undone
- [ ] `chat.pin` — Pin or unpin a chat
- [x] `chat.read` — Send receipts and clear the local unread count
- [ ] `chat.set_read` — Mark a chat read or unread across linked devices
- [ ] `chat.shared` — Page through a chat's media, documents, or links
- [x] `chat.typing` — Set composing or paused presence

## chats (5)

- [ ] `chats.archived_count` — Count archived chats
- [x] `chats.list` — List or search chats
- [ ] `chats.mark_all_read` — Clear the unread state of every conversation
- [ ] `chats.search` — Search conversations by title (served, not advertised by rpc.discover)
- [ ] `chats.unread_count` — Count unread messages in visible chats

## communities (1)

- [ ] `communities.list` — List joined communities

## community (4)

- [ ] `community.create` — Create a community
- [ ] `community.description` — Change description after fresh admin and previous-value checks
- [ ] `community.info` — Read description, linked groups and groups available to link
- [ ] `community.link` — Link or unlink a group after checking current permissions

## connection (2)

- [ ] `connection.connect` — Connect the linked device
- [ ] `connection.disconnect` — Disconnect without unlinking

## contact (4)

- [ ] `contact.block` — Block or unblock a contact
- [ ] `contact.presence.subscribe` — Subscribe to a contact's online and last-seen updates
- [ ] `contact.resolve` — Resolve a phone number to a WhatsApp chat
- [ ] `contact.save` — Resolve a phone number and save a local WhatsAppGo label

## contacts (3)

- [ ] `contacts.blocked` — List blocked contacts
- [ ] `contacts.list` — List known contacts (served, not advertised by rpc.discover)
- [ ] `contacts.shareable` — Search up to 100 local contacts with known phone numbers, including archived chats

## event (1)

- [ ] `event.info` — Read locally received event details; RSVP is unavailable

## group (14)

- [ ] `group.create` — Create a group with the given name and participants
- [ ] `group.info` — Fetch live group membership, roles, permissions and creation metadata
- [ ] `group.invite_link` — Get the group invite link; reset revokes the previous link (admins only)
- [ ] `group.invite_preview` — Read group identity before joining; does not join
- [ ] `group.invite_qr` — Return the current invite link and PNG data URI
- [ ] `group.join_link` — Join a group from its invite link
- [ ] `group.join_previewed` — Join only after rechecking the previewed identity; joined=false means membership is not confirmed
- [ ] `group.leave` — Leave a group without deleting local history
- [ ] `group.members` — Add, remove, promote or demote group members with permission checks
- [ ] `group.requests.list` — List pending group join requests (current admins only)
- [ ] `group.requests.review` — Approve or reject one still-pending group request after fresh admin validation
- [ ] `group.set_info` — Update group name or description after checking permissions and the previous value
- [ ] `group.set_permission` — Update one group permission after checking current admin rights; boolean value and previous are required
- [ ] `group.set_photo` — Set or remove a group photo after fresh edit-permission validation

## history (2)

- [ ] `history.refresh` — Request a recent history page
- [x] `history.request` — Request an older history page

## label (1)

- [ ] `label.create` — Create a chat list

## labels (1)

- [ ] `labels.list` — List the account's chat lists

## link (2)

- [ ] `link.preview` — Resolve Open Graph metadata for composer text
- [ ] `link.preview.refresh` — Replace a small cached link image with its full preview

## media (1)

- [ ] `media.shared` — Page media, documents or links across every chat

## message (19)

- [ ] `message.answer_prompt` — Press one of the answers a received question offered; the choice must be one that question carried
- [x] `message.delete` — Delete a message for everyone
- [ ] `message.download` — Download and cache message media
- [x] `message.edit` — Edit a sent text message
- [x] `message.forward` — Forward a message into another conversation
- [x] `message.get` — One stored message with its reactions and quoted line
- [x] `message.pin` — Pin a message for 24 hours, 7 days, or 30 days
- [ ] `message.played` — Mark an incoming voice or video message as played
- [x] `message.react` — Add or remove a reaction
- [x] `message.revisions` — List the earlier versions of a message, including one that was deleted
- [ ] `message.schedule` — Write a message now and send it at a given time, while this app is running
- [x] `message.send` — Send a text message
- [ ] `message.send_buttons` — Ask a question with up to 3 tappable answers; taps arrive as message.tapped
- [ ] `message.send_contact` — Send one reviewed name and international phone number as a contact card
- [ ] `message.send_list` — Ask a question with up to 10 tappable choices behind one button; taps arrive as message.tapped
- [ ] `message.send_media` — Send media; document=true preserves file semantics regardless of MIME type
- [x] `message.star` — Star or unstar a message for the account
- [x] `message.unpin` — Remove the pinned message from a chat
- [ ] `message.unschedule` — Drop a message that has not been sent yet

## messages (5)

- [x] `messages.list` — Page through one chat's messages
- [ ] `messages.on_date` — Find the first locally stored message within a calendar day
- [ ] `messages.scheduled` — List messages waiting to be sent; an empty chat_jid lists every conversation
- [ ] `messages.search` — Search local message text
- [x] `messages.starred` — List starred messages, optionally scoped to chat_jid

## notifications (3)

- [ ] `notifications.get` — Read local per-profile notification preferences
- [ ] `notifications.set` — Change a local notification preference
- [ ] `notifications.test_sound` — Preview the incoming or outgoing desktop sound (Linux)

## pairing (2)

- [ ] `pairing.phone` — Request a phone pairing code
- [ ] `pairing.start` — Start QR-code pairing

## poll (3)

- [ ] `poll.create` — Create a poll with 2–12 distinct options
- [ ] `poll.info` — Read locally received poll options and vote totals
- [ ] `poll.vote` — Replace your poll selections; an empty list retracts your vote

## preferences (2)

- [ ] `preferences.get` — Read local media-download and link-preview preferences
- [ ] `preferences.set` — Change one local network preference

## privacy (3)

- [ ] `privacy.default_timer.set` — Set the default timer for new chats; current value cannot be read
- [ ] `privacy.get` — Read the account's privacy settings
- [ ] `privacy.set` — Change one privacy setting

## profile (5)

- [ ] `profile.export` — Write this account to a portable archive; it carries the device credentials, so move it like a password and run the profile on one machine only. deactivate retires this copy once the archive exists, which is what keeps one account from running in two places
- [ ] `profile.get` — Read your WhatsApp name, About text and profile photo
- [ ] `profile.set_about` — Change the account's about text
- [ ] `profile.set_name` — Change the WhatsApp profile name (1–25 characters)
- [ ] `profile.set_photo` — Set your account photo from a 640×640 JPEG, or explicitly remove it

## rpc (1)

- [ ] `rpc.discover` — List protocol methods and event names

## status (4)

- [ ] `status.audience` — Read the default status broadcast audience and exception JIDs
- [ ] `status.audience_contacts` — Resolve included/excluded status audience contacts; read-only
- [x] `status.get` — Get connection and login state
- [ ] `status.post` — Publish a status update, as text or as a photo or video

## statuses (1)

- [ ] `statuses.list` — List active status stories grouped by sender

## sticker (1)

- [ ] `sticker.send` — Reuse an original sticker from this account's history

## update (3)

- [ ] `update.check` — Ask GitHub now whether a newer release exists
- [ ] `update.download` — Download and check this platform's artifact for the newer release
- [ ] `update.status` — Report the installed version and any newer release

**119 methods in all.**

