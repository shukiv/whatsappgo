import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import QtMultimedia
import QtCore
import org.whatsappgo

ApplicationWindow {
    id: window
    width: 1280
    height: 800
    minimumWidth: 760
    minimumHeight: 540
    visible: true
    title: qsTr("WhatsAppGo")
    color: Theme.surface
    palette.window: Theme.surface
    palette.windowText: Theme.text
    palette.base: Theme.surface
    palette.alternateBase: Theme.surfaceMuted
    palette.text: Theme.text
    palette.button: Theme.surfaceRaised
    palette.buttonText: Theme.text
    palette.highlight: Theme.primary
    palette.highlightedText: Theme.primaryText
    palette.placeholderText: Theme.textMuted
    palette.link: Theme.primary

    property string transientError: ""
    property string transientNotice: ""
    property bool recordingVoice: Boolean(voiceRecorderLoader.item && voiceRecorderLoader.item.recording)
    property string replyTargetId: ""
    property string replyPreview: ""
    // A draft belongs to the conversation it was written in, the way WhatsApp
    // Web keeps one per chat. Without this, switching chats mid-sentence sent
    // one person's words - and the message they were replying to - to another.
    property var chatDrafts: ({})
    property string draftChatJid: ""
    // Which account and conversation the composer is holding words for. It is
    // remembered rather than recomputed because switching accounts changes the
    // account before the conversation is put down: parking the draft under a
    // freshly computed key filed the account being left under the account
    // being opened, which is how a draft crossed accounts.
    property string draftHeldKey: ""
    // The same person can be a conversation in two accounts, and those are two
    // conversations. The account is part of what a draft belongs to.
    function draftKey(jid) {
        return String(backend.profile || "") + "\u0000" + String(jid || "")
    }
    function rememberDraft() {
        if (window.draftChatJid === "")
            return
        const key = window.draftHeldKey || window.draftKey(window.draftChatJid)
        const text = composer.text
        if (text === "" && window.replyTargetId === "")
            delete window.chatDrafts[key]
        else
            window.chatDrafts[key] = {
                text: text, replyTargetId: window.replyTargetId, replyPreview: window.replyPreview
            }
    }
    function restoreDraft(jid) {
        const key = window.draftKey(jid)
        const draft = window.chatDrafts[key] || ({})
        composer.text = String(draft.text || "")
        window.replyTargetId = String(draft.replyTargetId || "")
        window.replyPreview = String(draft.replyPreview || "")
        window.draftChatJid = String(jid || "")
        window.draftHeldKey = key
    }
    function clearDraft() {
        composer.clear()
        window.replyTargetId = ""
        window.replyPreview = ""
        if (window.draftChatJid !== "")
            delete window.chatDrafts[window.draftHeldKey || window.draftKey(window.draftChatJid)]
    }
    // Only an acknowledgement may consume the draft, and only the revision
    // that was actually sent. A late answer must not erase newer typing.
    function finishTextSend(profile, jid, text, replyTo, success) {
        if (!success)
            return
        const key = String(profile) + "\u0000" + String(jid)
        if (key === window.draftHeldKey)
            window.rememberDraft()
        const draft = window.chatDrafts[key]
        if (!draft || draft.text !== text || draft.replyTargetId !== replyTo)
            return
        delete window.chatDrafts[key]
        if (key === window.draftHeldKey) {
            window.clearDraft()
            backend.clearComposerLinkPreview()
        }
    }
    function finishImageReply(key, replyTo) {
        if (key === window.draftHeldKey)
            window.rememberDraft()
        const draft = window.chatDrafts[key]
        if (!draft || draft.replyTargetId !== replyTo)
            return
        draft.replyTargetId = ""
        draft.replyPreview = ""
        if (key === window.draftHeldKey) {
            window.replyTargetId = ""
            window.replyPreview = ""
        }
    }
    property string pendingMessageJumpId: ""
    property int pendingMessageJumpAttempts: 0
    property string highlightedMessageId: ""
    property string chatFilter: "all"
    onChatFilterChanged: backend.setChatListFilter(chatFilter)
    property bool newChatOpen: false
    property bool showArchived: false
    property bool infoDrawerOpen: false
    // WhatsApp Web searches inside the open conversation from a panel beside
    // it, not from a dialog over the whole window.
    property bool chatSearchOpen: false
    function openConversationSearch() {
        if (!backend.selectedChat.jid)
            return
        chatSearchOpen = true
        backend.searchMessages("")
        chatSearchPanel.focusField()
    }
    function closeConversationSearch() {
        chatSearchOpen = false
        backend.searchMessages("")
    }
    property string infoDrawerChatJid: ""
	property bool messageInfoOpen: false
	property var messageInfoMessage: ({})
	property string messageInfoChatJid: ""
    property bool statusViewerRequested: false
    // Remembered so an account that lands on the linking screen can go back to
    // the one that was open before it.
    property string previousProfile: ""

    // The rail carries the update control as well as the settings page does:
    // an update nobody sees is an update nobody installs. What pressing it
    // means depends on how far along the update is - see SettingsPane, which
    // spells the same three steps out in words.
    readonly property bool updateDownloading: backend.updateStatus.downloading === true
    readonly property bool updateDownloaded: String(backend.updateStatus.downloaded || "") !== ""
    readonly property bool updateOffered: backend.updateStatus.available === true
    // What this copy calls itself, in the same words the Help page uses: a
    // build from a working copy carries a commit id, not a version.
    readonly property string updateVersionText: {
        const current = String(backend.updateStatus.current || "")
        if (/^v?\d+\.\d+(\.\d+)?([-+].*)?$/.test(current))
            return qsTr("WhatsAppGo %1").arg(current)
        return current === "" || current === "dev"
            ? qsTr("WhatsAppGo, built from source")
            : qsTr("WhatsAppGo, built from source (%1)").arg(current)
    }
    readonly property string updateActionText: {
        if (backend.checkingForUpdates)
            return qsTr("Checking for updates…")
        if (window.updateDownloading)
            return qsTr("Downloading version %1…").arg(String(backend.updateStatus.latest || ""))
        if (window.updateDownloaded)
            return qsTr("Install version %1 and restart").arg(String(backend.updateStatus.latest || ""))
        if (window.updateOffered)
            return backend.updateInstallable()
                ? qsTr("Download version %1").arg(String(backend.updateStatus.latest || ""))
                : qsTr("Open the release page for version %1").arg(String(backend.updateStatus.latest || ""))
        return qsTr("Update WhatsAppGo")
    }

    // editLastOwnMessage opens the edit dialog on the newest message this
    // account sent, and reports whether there was one to edit.
    function editLastOwnMessage() {
        const message = backend.messages.lastOwnEditableMessage()
        const id = String(message.id || "")
        if (id === "")
            return false
        editDialog.messageId = id
        editField.text = String(message.body || "")
        editDialog.open()
        return true
    }

    // sendAttachment sends a file as a reply when the composer is showing one,
    // which is what WhatsApp Web does with an attachment picked while replying.
    function sendAttachment(fileUrl, caption, document) {
        const replyTo = window.replyTargetId
        window.replyTargetId = ""
        window.replyPreview = ""
        window.rememberDraft()
        backend.sendFile(fileUrl, caption || "", replyTo, document === true)
    }

    property var voiceRecordingContext: null
    function startVoiceRecording() {
        const jid = String(backend.selectedChat.jid || "")
        if (!jid || window.voiceRecordingContext)
            return
        const context = { chatJid: jid, profile: backend.profile, replyTo: window.replyTargetId }
        window.voiceRecordingContext = context
        voiceRecorderLoader.active = true
        Qt.callLater(function() {
            // Navigation may cancel the recording before the loader is ready.
            if (window.voiceRecordingContext === context && voiceRecorderLoader.item)
                voiceRecorderLoader.item.start()
        })
    }

    function cancelVoiceRecording() {
        window.voiceRecordingContext = null
        if (voiceRecorderLoader.item)
            voiceRecorderLoader.item.cancel()
    }

    function sendRecordedVoice(path) {
        const context = window.voiceRecordingContext
        window.voiceRecordingContext = null
        if (!context || context.profile !== backend.profile
                || context.chatJid !== String(backend.selectedChat.jid || ""))
            return
        backend.sendVoice(path, context.chatJid, context.profile, context.replyTo)
        if (window.replyTargetId === context.replyTo) {
            window.replyTargetId = ""
            window.replyPreview = ""
            window.rememberDraft()
        }
    }

    // What has to be put down when the reader moves to another conversation.
    // Everything here belongs to the chat being left, and acting on it against
    // the new one is how a draft, a quotation, or a selection reached the wrong
    // person.
    function handleChatSwitched(jid) {
        if (window.draftKey(jid) === window.draftHeldKey)
            return false
        if (window.voiceRecordingContext) {
            window.cancelVoiceRecording()
            window.transientNotice = qsTr("Voice recording canceled because the conversation changed.")
            noticeTimer.restart()
        }
        // Compared by key, not by conversation: the same person in a second
        // account is a second conversation with a draft of its own.
        if (window.draftKey(jid) !== window.draftHeldKey) {
            window.rememberDraft()
            window.restoreDraft(jid)
            // Results found in another conversation are not results in this
            // one: clicking one used to search this chat for somebody else's
            // message.
            if (window.chatSearchOpen)
                window.closeConversationSearch()
        }
        // A search result opens the chat it was found in; clearing the target
        // here left the jump with nothing to look for by the time the first
        // page arrived.
        if (window.pendingMessageJumpChat !== jid) {
            window.pendingMessageJumpId = ""
            window.pendingMessageJumpAttempts = 0
        }
        // Messages picked in another conversation are not a selection in this
        // one: Star, Delete and Forward used to act on them against the new
        // chat.
        if (window.messageSelectionActive)
            window.endMessageSelection()
        return true
    }

    function applyUpdateAction() {
        if (window.updateDownloading)
            return
        if (window.updateDownloaded) {
            backend.installUpdate()
            return
        }
        if (window.updateOffered) {
            if (backend.updateInstallable())
                backend.downloadUpdate()
            else
                backend.openReleasePage()
            return
        }
        backend.checkForUpdates()
    }

    function switchProfile(profile) {
        if (String(profile) === backend.profile)
            return
        window.previousProfile = backend.profile
        backend.switchProfile(profile)
    }
    property string activeSection: {
        const allowed = ["chats", "status", "calls", "channels", "communities", "media", "profile"]
        const args = Qt.application.arguments
        for (let i = 0; i < args.length - 1; ++i) {
            if (args[i] === "--section" && allowed.indexOf(args[i + 1]) >= 0)
                return args[i + 1]
        }
        return "chats"
    }
    // The sidebar swaps the chat list for grouped results while a query is
    // live, which several rows below need to know about.
    readonly property bool searching: String(backend.chatQuery || "").trim().length > 0
    function clearChatSearch() {
        searchField.clear()
        searchTimer.stop()
        backend.searchChats("")
    }
    readonly property int totalUnreadCount: {
        let total = 0
        for (let i = 0; i < backend.chats.length; ++i)
            total += Number(backend.chats[i].unread_count || 0)
        return total
    }
    Settings {
        id: appearanceSettings
        category: "appearance"
        property string themeMode: "system"
    }
    Settings {
        id: updateSettings
        category: "updates"
        // The version the reader has already been offered. Without this, a
        // release that stays newest for a week asks again every few hours and
        // at every start.
        property string offeredVersion: ""
    }
    Component.onCompleted: {
        Theme.preferredMode = appearanceSettings.themeMode
        if (backend.daemonConnected) {
            backend.refreshStatuses()
            if (activeSection !== "chats")
                Qt.callLater(() => showSection(activeSection))
        }
    }
    Connections {
        target: Theme
        function onPreferredModeChanged() {
            appearanceSettings.themeMode = Theme.preferredMode
        }
    }

    function friendlyTitle(title, jid) {
        const fullJid = String(jid || "")
        const local = fullJid.indexOf("@") >= 0 ? fullJid.slice(0, fullJid.indexOf("@")) : fullJid
        const candidate = String(title || "")
        if (candidate && candidate !== fullJid && candidate !== local)
            return candidate
        if (fullJid.endsWith("@s.whatsapp.net"))
            return "+" + local
        if (fullJid.endsWith("@g.us"))
            return qsTr("Group")
        return qsTr("Contact · %1").arg(local.slice(-4))
    }

    function showSection(section) {
        newChatOpen = false
        activeSection = section
        if (section === "status") backend.refreshStatuses()
        else if (section === "calls") backend.refreshCalls()
        else if (section === "channels") backend.refreshChannels()
        else if (section === "communities") backend.refreshCommunities()
        else if (section === "media") backend.refreshMediaLibrary("media", false)
        else if (section === "chats") {
            backend.refreshBlockedContacts()
            backend.refreshChatLabels()
        }
    }

    function statusGroupIndexForJid(jid) {
        const target = String(jid || "")
        const groups = backend.statusUpdates
        for (let index = 0; index < groups.length; ++index) {
            if (String(groups[index].sender_jid || "") === target)
                return index
        }
        return -1
    }

    function openStatusAt(index) {
        if (index < 0)
            return
        statusViewerRequested = true
        Qt.callLater(function() {
            if (statusViewerLoader.item)
                statusViewerLoader.item.openAt(index)
        })
    }

    // The rail no longer offers search — WhatsApp Web has no rail entry for it
    // either — but focusing the sidebar field is still the app's one way in,
    // and the desktop suite drives it through here.
    function openChatSearch() {
        showSection("chats")
        searchField.forceActiveFocus()
    }

    // WhatsApp Web's "Select messages" turns the conversation into a picker and
    // replaces the header with a bar of bulk actions. Whole message records are
    // kept rather than ids, because starring and deleting both need the sender
    // and direction, which are not derivable from an id alone.
    property bool messageSelectionActive: false
    property var selectedMessages: []

    function beginMessageSelection() {
        selectedMessages = []
        messageSelectionActive = true
    }

    function endMessageSelection() {
        messageSelectionActive = false
        selectedMessages = []
    }

    function isMessageSelected(messageId) {
        const id = String(messageId || "")
        for (let i = 0; i < selectedMessages.length; ++i) {
            if (String(selectedMessages[i].id) === id)
                return true
        }
        return false
    }

    function toggleMessageSelection(message) {
        const id = String(message.id || "")
        if (id === "")
            return
        const next = []
        let found = false
        for (let i = 0; i < selectedMessages.length; ++i) {
            if (String(selectedMessages[i].id) === id)
                found = true
            else
                next.push(selectedMessages[i])
        }
        if (!found)
            next.push({ id: id, sender_jid: String(message.sender_jid || ""),
                        from_me: Boolean(message.from_me), starred: Boolean(message.starred) })
        selectedMessages = next
    }

    function starSelectedMessages(starred) {
        const chosen = selectedMessages
        for (let i = 0; i < chosen.length; ++i)
            backend.starMessage(chosen[i].id, chosen[i].sender_jid, chosen[i].from_me, starred)
        endMessageSelection()
    }

    function deleteSelectedMessages() {
        const chosen = selectedMessages
        for (let i = 0; i < chosen.length; ++i)
            backend.deleteMessage(chosen[i].id, chosen[i].sender_jid)
        endMessageSelection()
    }

    // The PWA's "Select chats" turns the sidebar into a picker with the row
    // menu's own actions applied in bulk.
    property bool chatSelectionActive: false
    property var selectedChatJids: []

    function beginChatSelection() {
        selectedChatJids = []
        chatSelectionActive = true
    }

    function endChatSelection() {
        chatSelectionActive = false
        selectedChatJids = []
    }

    function toggleChatSelection(jid) {
        const value = String(jid || "")
        if (value === "")
            return
        const next = selectedChatJids.slice()
        const at = next.indexOf(value)
        if (at >= 0)
            next.splice(at, 1)
        else
            next.push(value)
        selectedChatJids = next
    }

    function archiveSelectedChats() {
        const chosen = selectedChatJids
        for (let i = 0; i < chosen.length; ++i)
            backend.setChatArchived(chosen[i], !window.showArchived)
        endChatSelection()
    }

    function readSelectedChats() {
        const chosen = selectedChatJids
        for (let i = 0; i < chosen.length; ++i)
            backend.setChatRead(chosen[i], true)
        endChatSelection()
    }

    function openNewChat() {
        activeSection = "chats"
        newChatOpen = true
    }

    function openContactInfo() {
        if (!backend.selectedChat.jid)
            return
        infoDrawerChatJid = backend.selectedChat.jid
        infoDrawerOpen = true
		messageInfoOpen = false
		messageInfoMessage = ({})
		messageInfoChatJid = ""
        backend.refreshChatInfo()
        contactInfoDrawer.forceActiveFocus()
    }

    function insertComposerEmoji(emoji) {
        const position = Math.max(0, composer.cursorPosition)
        composer.insert(position, emoji)
        composer.cursorPosition = position + emoji.length
        composer.forceActiveFocus()
    }

    function toggleAttachmentMenu() {
        emojiPicker.close()
        attachmentMenu.toggleUnder(attachmentButton)
    }

    function prepareClipboardPaste() {
        if (mediaPreview.sending)
            return
        const imageUrl = backend.prepareClipboardImage()
        if (!imageUrl)
            return
        if (mediaPreview.previewActive)
            backend.discardClipboardImage(mediaPreview.imageUrl)
        mediaPreview.ownerKey = window.draftHeldKey
        mediaPreview.openImage(imageUrl)
    }

    function localMediaUrl(path) {
        return Theme.fileUrl(path)
    }

    function formatMessageDate(value) {
        let timestamp = Number(value || 0)
        if (!timestamp)
            return ""
        if (timestamp < 1000000000000)
            timestamp *= 1000
        return Qt.formatDateTime(new Date(timestamp), Qt.DefaultLocaleShortDate)
    }

    function presenceText(presence, cachedLastSeen) {
        if (String(presence.chat_state || "") === "composing")
            return String(presence.media || "") === "audio" ? qsTr("Recording audio…") : qsTr("Typing…")
        const state = String(presence.state || "")
        if (state === "online")
            return qsTr("Online")
        const timestamp = Number(presence.last_seen || cachedLastSeen || 0)
        if (timestamp <= 0)
            return ""
        if (state !== "" && state !== "offline")
            return ""
        const seen = new Date(timestamp)
        const today = new Date()
        const sameDay = seen.getFullYear() === today.getFullYear()
            && seen.getMonth() === today.getMonth() && seen.getDate() === today.getDate()
        if (sameDay)
            return qsTr("Last seen today at %1").arg(Qt.formatTime(seen, "HH:mm"))
        const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1)
        const wasYesterday = seen.getFullYear() === yesterday.getFullYear()
            && seen.getMonth() === yesterday.getMonth() && seen.getDate() === yesterday.getDate()
        if (wasYesterday)
            return qsTr("Last seen yesterday at %1").arg(Qt.formatTime(seen, "HH:mm"))
        return qsTr("Last seen %1 at %2").arg(Qt.formatDate(seen, Qt.DefaultLocaleShortDate))
            .arg(Qt.formatTime(seen, "HH:mm"))
    }

    function selectedPresenceText() {
        const info = backend.chatInfo || ({})
        return presenceText(backend.selectedPresence || ({}), Number(info.last_seen || 0))
    }

    function openChatImage(message) {
        if (!message)
            return
        const path = String(message.media_path || message.media_thumbnail || "")
        if (!path) {
            if (message.id)
                backend.downloadMedia(message.id)
            return
        }
        const fromMe = Boolean(message.from_me)
        const title = fromMe
            ? (backend.status.user_name || backend.profile || qsTr("You"))
            : (message.sender_name || friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid))
        const avatar = fromMe || !backend.selectedChat.avatar_path
            ? "" : localMediaUrl(backend.selectedChat.avatar_path)
        chatMediaViewer.openImage(localMediaUrl(path), title, avatar,
                                  formatMessageDate(message.timestamp), String(message.body || ""),
                                  String(message.id || ""))
    }

    function completeChatImageDownload(messageId, path) {
        Playback.fileReady(messageId, path)
        if (chatMediaViewer.previewActive && chatMediaViewer.messageId === messageId)
            chatMediaViewer.replaceImage(window.localMediaUrl(path))
        if (window.infoDrawerOpen) {
            backend.refreshChatInfo()
            if (contactInfoDrawer.sharedView)
                backend.refreshSharedContent(contactInfoDrawer.activeCategory)
        }
    }

    function finishMessageJump(messageId, index) {
        pendingMessageJumpId = ""
        pendingMessageJumpAttempts = 0
        messageJumpRetry.stop()
        messageList.followTail = false
        highlightedMessageId = messageId
        Qt.callLater(function() {
            messageList.positionViewAtIndex(index, ListView.Center)
        })
        messageJumpHighlight.restart()
    }

    function resolvePendingMessageJump() {
        const messageId = pendingMessageJumpId
        if (!messageId)
            return
        const index = backend.messageIndex(messageId)
        if (index >= 0) {
            finishMessageJump(messageId, index)
            return
        }

        // A quoted message may sit outside the current 50-message page. Load
        // history page by page until it becomes part of the model, while
        // keeping the reader's viewport away from the conversation tail.
        if (pendingMessageJumpAttempts > 0 && !backend.canLoadOlderMessages()) {
            pendingMessageJumpId = ""
            pendingMessageJumpAttempts = 0
            messageJumpRetry.stop()
            transientNotice = qsTr("The quoted message is not available in this chat.")
            noticeTimer.restart()
            return
        }
        if (pendingMessageJumpAttempts >= 80) {
            pendingMessageJumpId = ""
            pendingMessageJumpAttempts = 0
            messageJumpRetry.stop()
            transientNotice = qsTr("The quoted message could not be loaded.")
            noticeTimer.restart()
            return
        }
        messageList.followTail = false
        pendingMessageJumpAttempts += 1
        backend.loadOlderMessages()
        if (!messageJumpRetry.running)
            messageJumpRetry.start()
    }

    // A search result names a message in a chat that is not open yet. The
    // existing jump pages history back until it finds the message, which would
    // page the conversation being left if it started before the new one had
    // loaded, so this waits for the first page to arrive.
    property string pendingMessageJumpChat: ""
    function jumpToMessageInChat(chatJid, chatTitle, messageId) {
        const jid = String(chatJid || "")
        const target = String(messageId || "")
        if (!jid || !target)
            return
        pendingMessageJumpChat = jid
        pendingMessageJumpId = target
        pendingMessageJumpAttempts = 0
        backend.openChat(jid, chatTitle)
        messageJumpArrival.ticks = 0
        messageJumpArrival.restart()
    }

    function jumpToMessage(messageId) {
        const target = String(messageId || "")
        if (!target)
            return
        pendingMessageJumpId = target
        pendingMessageJumpAttempts = 0
        resolvePendingMessageJump()
    }

    Loader {
        id: voiceRecorderLoader
        active: false
        sourceComponent: VoiceRecorder {
            onFinished: path => window.sendRecordedVoice(path)
            onFailed: message => {
                window.cancelVoiceRecording()
                window.transientError = message
                errorTimer.restart()
            }
        }
    }

    Connections {
        target: backend
        function onProfileChanged() {
            starredMessagesDialog.close()
            starredMessagesDialog.chatJid = ""
        }
        function onErrorOccurred(message) {
            window.transientError = message
            errorTimer.restart()
        }
        function onNoticeOccurred(message) {
            window.transientNotice = message
            noticeTimer.restart()
        }
        function onUpdateAvailable(version) {
            if (!version || version === updateSettings.offeredVersion)
                return
            updateSettings.offeredVersion = version
            updatePromptDialog.open()
        }
        function onUpdateReady(path, version) {
            updateReadyDialog.open()
        }
        function onUpdateCheckFinished(available, version, error) {
            // A check that says nothing looks like a button that does nothing.
            if (error) {
                window.transientNotice = qsTr("Could not check for updates: %1").arg(error)
            } else if (available) {
                window.transientNotice = qsTr("Version %1 is available.").arg(version)
            } else {
                window.transientNotice = qsTr("WhatsAppGo is up to date.")
            }
            noticeTimer.restart()
        }
        function onUpdateFailed(message) {
            window.transientError = message
            errorTimer.restart()
        }
        function onBugReportFinished(success, message, url) {
            bugReportDialog.finish(success, message)
            if (!success)
                return
            // The URL is the only receipt the reader gets, so it goes in the
            // notice rather than only into the dialog that just closed.
            window.transientNotice = url ? qsTr("Report sent: %1").arg(url) : message
            noticeTimer.restart()
        }
        function onDaemonConnectedChanged() {
            if (!backend.daemonConnected)
                return
            backend.refreshStatuses()
            // showSection() is what normally loads a section's side data, and it
            // never runs for the section the window already starts on.
            if (window.activeSection === "chats") {
                backend.refreshBlockedContacts()
                backend.refreshChatLabels()
            } else {
                window.showSection(window.activeSection)
            }
        }
        function onMediaReady(messageId, path) {
            window.completeChatImageDownload(messageId, path)
        }
    }
    Timer { id: errorTimer; interval: 5000; onTriggered: window.transientError = "" }
    Timer { id: noticeTimer; interval: 3000; onTriggered: window.transientNotice = "" }
    Timer {
        id: messageJumpRetry
        interval: 250
        repeat: true
        onTriggered: window.resolvePendingMessageJump()
    }
    Timer {
        id: messageJumpArrival
        interval: 120
        repeat: true
        property int ticks: 0
        onTriggered: {
            if (window.pendingMessageJumpChat === "") {
                stop()
                return
            }
            ticks += 1
            if (backend.selectedChat.jid === window.pendingMessageJumpChat && messageList.count > 0) {
                stop()
                window.pendingMessageJumpChat = ""
                window.resolvePendingMessageJump()
                return
            }
            // The chat opened but never produced a page; give up rather than
            // leaving a jump armed for whatever the reader opens next.
            if (ticks > 60) {
                stop()
                window.pendingMessageJumpChat = ""
                window.pendingMessageJumpId = ""
                window.pendingMessageJumpAttempts = 0
            }
        }
    }
    Timer {
        id: messageJumpHighlight
        interval: 1600
        onTriggered: window.highlightedMessageId = ""
    }

    // Not being connected to the daemon is not the same as not being linked.
    // WhatsApp Web never answers a dropped connection with its QR page, and a
    // paired account that briefly loses the daemon must not be sent back to
    // linking, so the QR page waits until the daemon has actually reported the
    // account's state.
    readonly property bool accountStateKnown: backend.daemonConnected
        && String(backend.status.state || "") !== ""

    PairingPage {
        anchors.fill: parent
        visible: window.accountStateKnown && !backend.loggedIn
        previousProfile: window.previousProfile
        onSwitchRequested: profile => window.switchProfile(profile)
        onRenameRequested: profile => {
            renameAccountDialog.profile = profile
            renameAccountDialog.initialName = String(backend.profileDisplayNames[profile] || profile)
            renameAccountDialog.open()
        }
        // The switcher offers removal wherever it appears, so the pairing page
        // needs the same confirmation the main window uses; without it the
        // delete control on this page did nothing at all.
        onRemoveRequested: profile => {
            removeAccountDialog.profile = profile
            removeAccountDialog.open()
        }
    }

    // The connecting state keeps the account's own screen rather than showing
    // either the QR page or a blank window.
    ColumnLayout {
        anchors.centerIn: parent
        spacing: 12
        visible: !backend.loggedIn && !window.accountStateKnown
        BusyIndicator {
            Layout.alignment: Qt.AlignHCenter
            running: parent.visible
        }
        Label {
            objectName: "connectingLabel"
            Layout.alignment: Qt.AlignHCenter
            text: qsTr("Connecting to the WhatsAppGo service\u2026")
            color: Theme.textMuted
            font.pixelSize: 14
        }
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0
        visible: backend.loggedIn

        Rectangle {
            id: navigationRail
            objectName: "navigationRail"
            Layout.preferredWidth: 64
            Layout.fillHeight: true
            color: Theme.navigation

            ColumnLayout {
                objectName: "navigationRailColumn"
                anchors.fill: parent
                anchors.topMargin: 10
                anchors.bottomMargin: 10
                spacing: 4

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: Qt.resolvedUrl("icons/chats.svg")
                    iconTint: window.activeSection === "chats" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Chats")
                    Accessible.checked: window.activeSection === "chats"
                    onClicked: window.showSection("chats")
                    background: Rectangle {
                        objectName: "navigationChatsBackground"
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "chats" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    Rectangle {
                        visible: window.totalUnreadCount > 0
                        anchors.right: parent.right
                        anchors.rightMargin: 2
                        anchors.top: parent.top
                        anchors.topMargin: 2
                        implicitWidth: Math.max(18, railUnread.implicitWidth + 8)
                        implicitHeight: 18
                        radius: 9
                        color: Theme.primary
                        Label {
                            id: railUnread
                            anchors.centerIn: parent
                            text: window.totalUnreadCount > 99 ? "99+" : window.totalUnreadCount
                            color: Theme.primaryText
                            font.pixelSize: 10
                            font.weight: Font.DemiBold
                        }
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: "calls.svg"
                    iconTint: window.activeSection === "calls" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Calls")
                    onClicked: window.showSection("calls")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "calls" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: "status.svg"
                    iconTint: window.activeSection === "status" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Status")
                    onClicked: window.showSection("status")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "status" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: "channels.svg"
                    iconTint: window.activeSection === "channels" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Channels")
                    onClicked: window.showSection("channels")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "channels" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: "communities.svg"
                    iconTint: window.activeSection === "communities" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Communities")
                    onClicked: window.showSection("communities")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "communities" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                Item { Layout.fillHeight: true }

                ThemedToolButton {
                    objectName: "navigationUpdateButton"
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: Qt.resolvedUrl("icons/reconnect.svg")
                    iconTint: window.updateOffered || window.updateDownloaded ? Theme.brand : Theme.icon
                    iconSpinning: backend.checkingForUpdates || window.updateDownloading
                    enabled: !window.updateDownloading && !backend.checkingForUpdates
                    Accessible.name: window.updateVersionText + ". " + window.updateActionText
                    onClicked: window.applyUpdateAction()
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    // A release waiting to be installed is worth noticing from
                    // the rail, the way an unread conversation is.
                    Rectangle {
                        objectName: "navigationUpdateDot"
                        visible: window.updateOffered || window.updateDownloaded
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.rightMargin: 4
                        anchors.topMargin: 4
                        width: 9
                        height: 9
                        radius: 4.5
                        color: Theme.brand
                        border.color: Theme.navigation
                        border.width: 2
                    }
                    ToolTip.visible: hovered
                    // The version this copy is on belongs where somebody looks
                    // before deciding whether to update it.
                    ToolTip.text: window.updateVersionText + "\n" + window.updateActionText
                }

                ThemedToolButton {
                    objectName: "navigationMediaButton"
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: Qt.resolvedUrl("icons/gallery.svg")
                    iconTint: window.activeSection === "media" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Media from all chats")
                    onClicked: window.showSection("media")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "media" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    objectName: "navigationProfileButton"
                    iconSource: "profile.svg"
                    iconTint: window.activeSection === "profile" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Profile")
                    onClicked: window.showSection("profile")
                    background: Rectangle {
                        radius: 12
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "profile" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    Rectangle {
                        objectName: "navigationConnectionDot"
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        width: 11
                        height: 11
                        radius: 5.5
                        color: backend.status.state === "connected" ? Theme.brand : Theme.textMuted
                        border.color: Theme.navigation
                        border.width: 2
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }
            }

            Rectangle {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1
                color: Theme.border
            }
        }

        Rectangle {
            id: sidebar
            Layout.preferredWidth: Math.max(360, Math.min(560, window.width * 0.40))
            Layout.fillHeight: true
            color: Theme.surface
            border.color: Theme.border
            visible: window.activeSection === "chats"

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Rectangle {
                    objectName: "chatSelectionBar"
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? 64 : 0
                    visible: window.chatSelectionActive
                    color: Theme.surface

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 14
                        anchors.rightMargin: 14
                        spacing: 8

                        ThemedToolButton {
                            objectName: "chatSelectionCancelButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/close.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Cancel selection")
                            onClicked: window.endChatSelection()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        Label {
                            objectName: "chatSelectionCountLabel"
                            Layout.fillWidth: true
                            text: qsTr("%1 selected").arg(window.selectedChatJids.length)
                            color: Theme.text
                            font.pixelSize: 16
                            font.weight: Font.Medium
                        }
                        ThemedToolButton {
                            objectName: "chatSelectionReadButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            enabled: window.selectedChatJids.length > 0
                            iconSource: Qt.resolvedUrl("icons/chats.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Mark selected chats as read")
                            onClicked: window.readSelectedChats()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        ThemedToolButton {
                            objectName: "chatSelectionArchiveButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            enabled: window.selectedChatJids.length > 0
                            iconSource: Qt.resolvedUrl("icons/archive.svg")
                            iconSize: 20
                            Accessible.name: window.showArchived ? qsTr("Restore selected chats") : qsTr("Archive selected chats")
                            onClicked: window.archiveSelectedChats()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? 64 : 0
                    visible: !window.chatSelectionActive
                    color: Theme.surface

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 20
                        anchors.rightMargin: 12
                        spacing: 8

                        Label {
                            text: qsTr("WhatsAppGo")
                            color: Theme.primary
                            font.pixelSize: 18
                            font.weight: Font.Bold
                        }

                        Item {
                            Layout.fillWidth: true
                            Layout.minimumWidth: 8
                        }

                        ThemedToolButton {
                            id: bugReportButton
                            objectName: "bugReportButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/bug.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Report a problem")
                            onClicked: {
                                backend.refreshBugReportEnvironment()
                                bugReportDialog.reset()
                                bugReportDialog.open()
                            }
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }

                        AccountSwitcherButton {
                            id: accountSwitcherButton
                            objectName: "accountSwitcherButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            profiles: backend.profiles
                            currentProfile: backend.profile
                            displayNames: backend.profileDisplayNames
                            unreadCounts: backend.profileUnreadCounts
                            onSwitchRequested: profile => window.switchProfile(profile)
                            onRenameRequested: profile => {
                                renameAccountDialog.profile = profile
                                renameAccountDialog.initialName = String(backend.profileDisplayNames[profile] || profile)
                                renameAccountDialog.open()
                            }
                            onRemoveRequested: profile => {
                                removeAccountDialog.profile = profile
                                removeAccountDialog.open()
                            }
                            ToolTip.visible: hovered && !accountSwitcherButton.menuOpen
                            ToolTip.text: Accessible.name
                        }

                        ThemedToolButton {
                            id: sidebarMenuButton
                            objectName: "sidebarMenuButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/menu.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Main menu")
                            Accessible.description: sidebarMenu.opened ? qsTr("Menu open") : qsTr("Menu closed")
                            onClicked: sidebarMenu.toggleUnder(sidebarMenuButton)
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            // A tooltip that stays up once the menu is open
                            // covers the menu it belongs to.
                            ToolTip.visible: hovered && !sidebarMenu.visible
                            ToolTip.text: Accessible.name

                            WhatsAppMenuPopup {
                                id: sidebarMenu
                                objectName: "sidebarMenu"
                                parent: Overlay.overlay
                                anchorItem: sidebarMenuButton

                                WhatsAppMenuItem {
                                    objectName: "newGroupMenuItem"
                                    text: qsTr("New group")
                                    iconSource: Qt.resolvedUrl("icons/communities.svg")
                                    onClicked: { sidebarMenu.close(); newGroupDialog.open() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "joinGroupMenuItem"
                                    text: qsTr("Join group with link")
                                    iconSource: Qt.resolvedUrl("icons/link.svg")
                                    onClicked: {
                                        sidebarMenu.close()
                                        joinGroupDialog.open()
                                    }
                                }
                                WhatsAppMenuItem {
                                    objectName: "selectChatsMenuItem"
                                    text: qsTr("Select chats")
                                    iconSource: Qt.resolvedUrl("icons/copy.svg")
                                    onClicked: { sidebarMenu.close(); window.beginChatSelection() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "markAllReadMenuItem"
                                    text: qsTr("Mark all as read")
                                    iconSource: Qt.resolvedUrl("icons/chats.svg")
                                    onClicked: { sidebarMenu.close(); backend.markAllChatsRead() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "starredMessagesMenuItem"
                                    text: qsTr("Starred messages")
                                    iconSource: Qt.resolvedUrl("icons/star.svg")
                                    onClicked: { sidebarMenu.close(); starredMessagesDialog.openForChat("") }
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Add another account")
                                    iconSource: Qt.resolvedUrl("icons/new-chat.svg")
                                    onClicked: { sidebarMenu.close(); addAccountDialog.open() }
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Reconnect")
                                    iconSource: Qt.resolvedUrl("icons/reconnect.svg")
                                    onClicked: { sidebarMenu.close(); backend.reconnect() }
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Appearance")
                                    iconSource: Theme.dark ? Qt.resolvedUrl("icons/moon.svg") : Qt.resolvedUrl("icons/sun.svg")
                                    onClicked: { sidebarMenu.close(); appearanceMenu.open() }
                                }
                                Rectangle {
                                    width: parent.width
                                    height: 1
                                    color: Theme.border
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Unlink this account")
                                    iconSource: Qt.resolvedUrl("icons/logout.svg")
                                    destructive: true
                                    onClicked: { sidebarMenu.close(); logoutDialog.open() }
                                }
                            }

                            WhatsAppMenuPopup {
                                id: appearanceMenu
                                objectName: "appearanceMenu"
                                parent: Overlay.overlay
                                width: 226
                                x: Math.max(8, sidebarMenu.x - width - 8)
                                y: Math.min(window.height - height - 8, sidebarMenu.y + 3 * 36)

                                WhatsAppMenuItem {
                                    text: qsTr("System default")
                                    iconSource: Qt.resolvedUrl("icons/sun.svg")
                                    checkable: true
                                    checked: Theme.preferredMode === "system"
                                    onClicked: { Theme.preferredMode = "system"; appearanceMenu.close() }
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Light")
                                    iconSource: Qt.resolvedUrl("icons/sun.svg")
                                    checkable: true
                                    checked: Theme.preferredMode === "light"
                                    onClicked: { Theme.preferredMode = "light"; appearanceMenu.close() }
                                }
                                WhatsAppMenuItem {
                                    text: qsTr("Dark")
                                    iconSource: Qt.resolvedUrl("icons/moon.svg")
                                    checkable: true
                                    checked: Theme.preferredMode === "dark"
                                    onClicked: { Theme.preferredMode = "dark"; appearanceMenu.close() }
                                }
                            }
                        }

                        ThemedToolButton {
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/new-chat.svg")
                            iconTint: Theme.primaryText
                            iconSize: 20
                            Accessible.name: qsTr("Start a new chat")
                            onClicked: window.openNewChat()
                            background: Rectangle {
                                radius: 20
                                color: parent.down ? Qt.darker(Theme.primary, 1.12) : Theme.primary
                            }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 64
                    color: Theme.surface

                    Rectangle {
                        anchors.fill: parent
                        anchors.leftMargin: 22
                        anchors.rightMargin: 22
                        anchors.topMargin: 12
                        anchors.bottomMargin: 12
                        radius: height / 2
                        color: Theme.surfaceMuted
                        // WhatsApp Web rings the pill in green while it has the
                        // caret, which is the only cue that typing searches.
                        border.width: searchField.activeFocus ? 2 : 0
                        border.color: Theme.primary

                        TintedIcon {
                            anchors.left: parent.left
                            anchors.leftMargin: 13
                            anchors.verticalCenter: parent.verticalCenter
                            width: 18
                            height: 18
                            source: Qt.resolvedUrl("icons/search.svg")
                            tint: Theme.icon
                        }
                        TextField {
                            id: searchField
                            objectName: "chatSearchField"
                            anchors.fill: parent
                            leftPadding: 42
                            rightPadding: 40
                            placeholderText: qsTr("Search or start a new chat")
                            color: Theme.text
                            font.pixelSize: 14
                            Accessible.name: qsTr("Search chats")
                            // Programmatic clears do not raise onTextEdited, so
                            // the query follows the text itself.
                            onTextChanged: searchTimer.restart()
                            Keys.onEscapePressed: window.clearChatSearch()
                            background: Item {}
                        }
                        ThemedToolButton {
                            objectName: "chatSearchClear"
                            anchors.right: parent.right
                            anchors.rightMargin: 6
                            anchors.verticalCenter: parent.verticalCenter
                            visible: searchField.text.length > 0
                            width: 28
                            height: 28
                            iconSource: Qt.resolvedUrl("icons/close.svg")
                            iconTint: Theme.icon
                            iconSize: 16
                            Accessible.name: qsTr("Clear search")
                            onClicked: window.clearChatSearch()
                            background: Item {}
                        }
                    }
                }

                ChatFilterBar {
                    id: chatFilterBar
                    objectName: "chatFilterBar"
                    visible: !window.showArchived
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? implicitHeight : 0
                    selectedFilter: window.chatFilter
                    unreadCount: window.totalUnreadCount
                    onFilterSelected: filter => window.chatFilter = filter
                    onNewListRequested: newListDialog.open()
                }

                // WhatsApp places filters first, then the archived-chat
                // destination before the conversation rows.
                ItemDelegate {
                    id: archivedRow
                    objectName: "archivedRow"
                    Layout.fillWidth: true
                    visible: !window.searching && (backend.archivedCount > 0 || window.showArchived)
                    height: visible ? 49 : 0
                    leftPadding: 7
                    rightPadding: 15
                    Accessible.name: window.showArchived ? qsTr("Back to chats") : qsTr("Archived chats")
                    onClicked: window.showArchived = !window.showArchived
                    background: Rectangle {
                        color: archivedRow.hovered ? Theme.hoverRow : Theme.surface
                    }
                    contentItem: RowLayout {
                        spacing: 13
                        // The glyph sits in the same column the chat avatars
                        // use, so the row's label starts on the chat titles
                        // rather than 20 px to their left.
                        Item {
                            Layout.preferredWidth: 56
                            Layout.preferredHeight: 40
                            Layout.leftMargin: 11
                            TintedIcon {
                                anchors.centerIn: parent
                                width: 20
                                height: 20
                                source: Qt.resolvedUrl(window.showArchived ? "icons/back.svg" : "icons/archive.svg")
                                tint: Theme.icon
                            }
                        }
                        Label {
                            Layout.fillWidth: true
                            text: window.showArchived ? qsTr("Back to chats") : qsTr("Archived")
                            color: Theme.textMuted
                            font.pixelSize: 15
                        }
                        Label {
                            visible: !window.showArchived && backend.archivedCount > 0
                            text: backend.archivedCount
                            color: Theme.textMuted
                            font.pixelSize: 12
                        }
                    }
                }

                Timer { id: searchTimer; interval: 120; onTriggered: backend.searchChats(searchField.text) }

                Item {
                    id: chatListViewport
                    objectName: "chatListViewport"
                    Layout.fillWidth: true
                    Layout.fillHeight: true

                    ListView {
                        id: chatList
                        objectName: "chatList"
                        anchors.left: parent.left
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        anchors.right: parent.right
                        anchors.leftMargin: 7
                        anchors.rightMargin: 8
                        visible: !window.searching
                        model: window.showArchived ? backend.archivedChatListModel : backend.chatListModel
                        spacing: 2
                        clip: true
                        reuseItems: true
                        boundsBehavior: Flickable.StopAtBounds
                        // The list arrives a page at a time. Nearing the end of
                        // what is loaded asks for the next page, so an account
                        // with more conversations than one page can be scrolled
                        // through rather than stopping dead.
                        onContentYChanged: {
                            if (!window.showArchived && contentHeight > 0
                                    && contentY + height > contentHeight - 600)
                                backend.loadMoreChats()
                        }
                        delegate: ChatListDelegate {
                            current: backend.selectedChat.jid === modelData.jid
                            selectionActive: window.chatSelectionActive
                            selected: window.chatSelectionActive
                                && window.selectedChatJids.indexOf(String(modelData.jid || "")) >= 0
                            onSelectionToggled: jid => window.toggleChatSelection(jid)
                            statusGroupIndex: window.statusGroupIndexForJid(modelData.jid)
                            statusItemCount: statusGroupIndex >= 0
                                ? (backend.statusUpdates[statusGroupIndex].items || []).length : 0
                            onChosen: (jid, title) => backend.openChat(jid, title)
                            onStatusRequested: jid => window.openStatusAt(window.statusGroupIndexForJid(jid))
                            onAvatarRequested: jid => backend.refreshChatAvatar(jid)
                        }

                        Column {
                            anchors.centerIn: parent
                            spacing: 10
                            visible: chatList.count === 0 && !window.searching
                            Label {
                                anchors.horizontalCenter: parent.horizontalCenter
                                text: {
                                    if (searchField.text)
                                        return qsTr("No matching chats")
                                    if (window.chatFilter === "unread")
                                        return qsTr("No unread chats")
                                    if (window.chatFilter === "favorites")
                                        return qsTr("No favorite chats")
                                    if (window.chatFilter === "groups")
                                        return qsTr("No groups")
                                    return qsTr("No chats synced yet")
                                }
                                color: Theme.textMuted
                            }
                            Button {
                                anchors.horizontalCenter: parent.horizontalCenter
                                text: qsTr("Refresh")
                                flat: true
                                onClicked: backend.refreshChats()
                            }
                        }
                    }

                    OverlayScrollBar {
                        id: chatListScrollBar
                        objectName: "chatListScrollBar"
                        visible: !window.searching
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        anchors.right: parent.right
                        size: chatList.visibleArea.heightRatio
                        position: chatList.visibleArea.yPosition
                        active: chatList.moving
                        onPositionChanged: {
                            if (pressed)
                                chatList.contentY = position * chatList.contentHeight
                        }
                    }

                    SearchResultsPane {
                        id: searchResultsPane
                        objectName: "searchResultsPane"
                        anchors.fill: parent
                        visible: window.searching
                        query: backend.chatQuery
                        chatHits: backend.chatSearchHits
                        contactHits: backend.contactSearchHits
                        messageHits: backend.messageSearchHits
                        onChatChosen: (jid, title) => {
                            backend.openChat(jid, title)
                            window.clearChatSearch()
                        }
                        // The conversation has to be open before the row can be
                        // found, and jumpToMessage pages back until it is.
                        onMessageChosen: (chatJid, chatTitle, messageId) => {
                            window.clearChatSearch()
                            window.jumpToMessageInChat(chatJid, chatTitle, messageId)
                        }
                    }
                }
            }

            NewChatPane {
                anchors.fill: parent
                visible: window.newChatOpen
                z: 10
                chats: backend.chats
                onCloseRequested: window.newChatOpen = false
                onChatSelected: (jid, title) => {
                    backend.openChat(jid, title)
                    window.newChatOpen = false
                }
                onPhoneRequested: phone => {
                    backend.startChat(phone)
                    window.newChatOpen = false
                }
                onUnavailableRequested: feature => {
                    window.transientNotice = qsTr("%1 is not supported yet").arg(feature)
                    noticeTimer.restart()
                }
            }
        }

        Rectangle {
            id: conversationPane
            Layout.fillWidth: true
            Layout.fillHeight: true
            color: backend.selectedChat.jid ? Theme.chatBackground : Theme.emptyBackground
            visible: window.activeSection === "chats"

            Image {
                objectName: "chatBackgroundPattern"
                anchors.fill: parent
                source: Qt.resolvedUrl("assets/chat-background.png")
                fillMode: Image.Tile
                horizontalAlignment: Image.AlignLeft
                verticalAlignment: Image.AlignTop
                opacity: Theme.patternOpacity
                visible: Boolean(backend.selectedChat.jid)
                smooth: false
                cache: true
                Accessible.ignored: true
            }

            Rectangle {
                anchors.centerIn: parent
                width: Math.min(460, parent.width - 80)
                height: emptyStateColumn.implicitHeight + 64
                radius: 24
                color: Theme.surface
                visible: !backend.selectedChat.jid
                border.width: Theme.dark ? 1 : 0
                border.color: Theme.border

                Column {
                    id: emptyStateColumn
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - 64
                    spacing: 14

                    TintedIcon {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 76
                        height: 76
                        source: Qt.resolvedUrl("icons/new-chat.svg")
                        tint: Theme.primary
                    }
                    Label {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: qsTr("WhatsAppGo for %1").arg(Theme.platformName)
                        color: Theme.text
                        font.pixelSize: 28
                        font.weight: Font.Normal
                    }
                    Label {
                        width: parent.width
                        text: qsTr("Send and receive messages without keeping your phone online. Your history is stored locally on this computer.")
                        color: Theme.textMuted
                        font.pixelSize: 14
                        wrapMode: Text.Wrap
                        horizontalAlignment: Text.AlignHCenter
                        lineHeight: 1.35
                    }
                    Rectangle {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 52
                        height: 1
                        color: Theme.border
                    }
                    Label {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: qsTr("End-to-end encrypted")
                        color: Theme.textMuted
                        font.pixelSize: 12
                    }
                }
            }

            ColumnLayout {
                anchors.fill: parent
                visible: Boolean(backend.selectedChat.jid)
                spacing: 0

                Rectangle {
                    objectName: "messageSelectionBar"
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? 64 : 0
                    visible: window.messageSelectionActive
                    color: Theme.surface

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 14
                        anchors.rightMargin: 14
                        spacing: 10

                        ThemedToolButton {
                            objectName: "messageSelectionCancelButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/close.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Cancel selection")
                            onClicked: window.endMessageSelection()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        Label {
                            objectName: "messageSelectionCountLabel"
                            Layout.fillWidth: true
                            text: qsTr("%1 selected").arg(window.selectedMessages.length)
                            color: Theme.text
                            font.pixelSize: 16
                            font.weight: Font.Medium
                        }
                        ThemedToolButton {
                            objectName: "messageSelectionStarButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            enabled: window.selectedMessages.length > 0
                            iconSource: Qt.resolvedUrl("icons/star.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Star selected messages")
                            onClicked: window.starSelectedMessages(true)
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        ThemedToolButton {
                            objectName: "messageSelectionForwardButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            enabled: window.selectedMessages.length > 0
                            iconSource: Qt.resolvedUrl("icons/forward.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Forward selected messages")
                            onClicked: {
                                forwardDialog.messageIds = window.selectedMessages.map(m => m.id)
                                forwardDialog.messageId = ""
                                window.endMessageSelection()
                                forwardDialog.open()
                            }
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        ThemedToolButton {
                            objectName: "messageSelectionDeleteButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            enabled: window.selectedMessages.length > 0
                            iconSource: Qt.resolvedUrl("icons/delete.svg")
                            iconSize: 20
                            iconTint: Theme.danger
                            Accessible.name: qsTr("Delete selected messages")
                            onClicked: window.deleteSelectedMessages()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? 64 : 0
                    visible: !window.messageSelectionActive
                    color: Theme.surface

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 14
                        anchors.rightMargin: 10
                        spacing: 10

                        Button {
                            id: contactHeaderButton
                            objectName: "contactHeaderButton"
                            Layout.fillWidth: true
                            Layout.preferredHeight: 56
                            flat: true
                            leftPadding: 0
                            rightPadding: 8
                            Accessible.name: qsTr("Open contact information for %1").arg(window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid))
                            onClicked: window.openContactInfo()
                            background: Rectangle {
                                radius: 10
                                color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                            }
                            contentItem: RowLayout {
                                spacing: 10
                                Avatar {
                                    Layout.preferredWidth: 48
                                    Layout.preferredHeight: 48
                                    diameter: 48
                                    title: window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid)
                                    fallbackIdentity: title.startsWith("+") || title.startsWith(qsTr("Contact ·"))
                                    source: Theme.fileUrl(backend.selectedChat.avatar_path)
                                    Accessible.ignored: true
                                }
                                ColumnLayout {
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 0
                                    spacing: 1
                                    Label {
                                        Layout.fillWidth: true
                                        Layout.minimumWidth: 0
                                        text: window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid)
                                        color: Theme.text
                                        font.pixelSize: 17
                                        font.weight: Font.Medium
                                        elide: Text.ElideRight
                                    }
                                    Label {
                                        objectName: "contactPresenceLabel"
                                        text: window.selectedPresenceText()
                                        visible: text !== ""
                                        color: Theme.textMuted
                                        font.pixelSize: 12
                                    }
                                }
                            }
                        }

                        ThemedToolButton {
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/search.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Search message history")
                            onClicked: window.openConversationSearch()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }

                        ThemedToolButton {
                            id: conversationMenuButton
                            objectName: "conversationMenuButton"
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/menu.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Conversation menu")
                            Accessible.description: conversationMenu.opened ? qsTr("Menu open") : qsTr("Menu closed")
                            onClicked: conversationMenu.toggleUnder(conversationMenuButton)
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered && !conversationMenu.visible
                            ToolTip.text: Accessible.name

                            WhatsAppMenuPopup {
                                id: conversationMenu
                                objectName: "conversationMenu"
                                parent: Overlay.overlay
                                width: 238
                                anchorItem: conversationMenuButton

                                WhatsAppMenuItem {
                                    objectName: "conversationContactInfoItem"
                                    text: backend.selectedChat.is_group ? qsTr("Group info") : qsTr("Contact info")
                                    iconSource: Qt.resolvedUrl("icons/profile.svg")
                                    onClicked: { conversationMenu.close(); window.openContactInfo() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationSearchItem"
                                    text: qsTr("Search")
                                    iconSource: Qt.resolvedUrl("icons/search.svg")
                                    onClicked: { conversationMenu.close(); window.openConversationSearch() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationSelectItem"
                                    text: qsTr("Select messages")
                                    iconSource: Qt.resolvedUrl("icons/copy.svg")
                                    onClicked: { conversationMenu.close(); window.beginMessageSelection() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationMuteItem"
                                    readonly property bool muted: Number(backend.selectedChat.muted_until || 0) > Date.now()
                                    text: muted ? qsTr("Unmute notifications") : qsTr("Mute notifications")
                                    iconSource: Qt.resolvedUrl("icons/mute.svg")
                                    onClicked: {
                                        conversationMenu.close()
                                        backend.setChatMuted(backend.selectedChat.jid, !muted)
                                    }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationCloseItem"
                                    text: qsTr("Close chat")
                                    iconSource: Qt.resolvedUrl("icons/close.svg")
                                    onClicked: { conversationMenu.close(); backend.closeChat() }
                                }
                                Rectangle {
                                    Layout.fillWidth: true
                                    Layout.preferredHeight: 1
                                    Layout.topMargin: 4
                                    Layout.bottomMargin: 4
                                    color: Theme.border
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationBlockItem"
                                    readonly property bool blocked:
                                        backend.blockedContacts.indexOf(String(backend.selectedChat.jid || "")) >= 0
                                    // Groups have no block, so the entry stays
                                    // out of a group's menu rather than failing
                                    // when pressed.
                                    visible: !backend.selectedChat.is_group
                                    text: blocked ? qsTr("Unblock") : qsTr("Block")
                                    destructive: !blocked
                                    iconSource: Qt.resolvedUrl("icons/block.svg")
                                    onClicked: {
                                        conversationMenu.close()
                                        backend.setContactBlocked(backend.selectedChat.jid, !blocked)
                                    }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationFavoriteItem"
                                    readonly property bool favorite: Boolean(backend.selectedChat.favorite)
                                    text: favorite ? qsTr("Remove from Favorites") : qsTr("Add to Favorites")
                                    iconSource: Qt.resolvedUrl("icons/heart.svg")
                                    onClicked: {
                                        conversationMenu.close()
                                        backend.setChatFavorite(backend.selectedChat.jid, !favorite)
                                    }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationExportItem"
                                    text: qsTr("Export chat")
                                    iconSource: Qt.resolvedUrl("icons/document.svg")
                                    onClicked: { conversationMenu.close(); exportChatDialog.open() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationDisappearingItem"
                                    text: qsTr("Disappearing messages")
                                    iconSource: Qt.resolvedUrl("icons/mute.svg")
                                    onClicked: { conversationMenu.close(); disappearingDialog.open() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationClearItem"
                                    text: qsTr("Clear chat")
                                    destructive: true
                                    iconSource: Qt.resolvedUrl("icons/block.svg")
                                    onClicked: { conversationMenu.close(); conversationClearDialog.open() }
                                }
                                WhatsAppMenuItem {
                                    objectName: "conversationDeleteItem"
                                    text: qsTr("Delete chat")
                                    destructive: true
                                    iconSource: Qt.resolvedUrl("icons/delete.svg")
                                    onClicked: { conversationMenu.close(); conversationDeleteDialog.open() }
                                }
                            }
                        }
                    }

                    Rectangle {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        height: 1
                        color: Theme.border
                    }
                }

                Rectangle {
                    id: pinnedMessageBar
                    objectName: "pinnedMessageBar"
                    readonly property var pinned: backend.chatInfo.pinned_message || ({})
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? 46 : 0
                    visible: Boolean(pinned.id)
                    color: Theme.surfaceRaised
                    border.color: Theme.border

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 16
                        anchors.rightMargin: 10
                        spacing: 12

                        TintedIcon {
                            Layout.preferredWidth: 20
                            Layout.preferredHeight: 20
                            source: Qt.resolvedUrl("icons/pin.svg")
                            tint: Theme.textMuted
                        }
                        Label {
                            Layout.fillWidth: true
                            text: pinnedMessageBar.pinned.body
                                  || pinnedMessageBar.pinned.media_name
                                  || qsTr("Pinned message")
                            color: Theme.textMuted
                            font.pixelSize: 13
                            elide: Text.ElideRight
                            maximumLineCount: 1
                            Accessible.name: qsTr("Pinned message: %1").arg(text)
                        }
                        ThemedToolButton {
                            id: pinnedMessageMenuButton
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
                            iconSize: 18
                            rotation: 90
                            Accessible.name: qsTr("Pinned message actions")
                            onClicked: pinnedMessageMenu.open()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                        }
                    }

                    TapHandler {
                        onTapped: pinnedMessageMenu.openUnder(pinnedMessageBar)
                    }

                    WhatsAppMenuPopup {
                        id: pinnedMessageMenu
                        objectName: "pinnedMessageMenu"
                        parent: Overlay.overlay
                        width: 210
                        anchorItem: pinnedMessageBar
                        anchorOffsetX: -10

                        WhatsAppMenuItem {
                            text: qsTr("Unpin")
                            iconSource: Qt.resolvedUrl("icons/pin.svg")
                            onClicked: {
                                pinnedMessageMenu.close()
                                backend.unpinMessage(pinnedMessageBar.pinned.id,
                                                     pinnedMessageBar.pinned.sender_jid || "")
                            }
                        }
                        WhatsAppMenuItem {
                            text: qsTr("Go to message")
                            iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
                            onClicked: {
                                pinnedMessageMenu.close()
                                window.jumpToMessage(pinnedMessageBar.pinned.id)
                            }
                        }
                    }
                }

                Item {
                    Layout.fillWidth: true
                    Layout.fillHeight: true

                    ListView {
                        id: messageList
                        objectName: "messageList"
                        property bool initialPositionPending: false
                        property string positionedChatJid: ""
                        anchors.fill: parent
                        model: backend.messages
                        clip: true
                        reuseItems: true
                        verticalLayoutDirection: ListView.BottomToTop
                        // Qt estimates a variable-height ListView from its
                        // instantiated delegates. Keep several screens around
                        // the viewport so alternating text and tall media do
                        // not make that estimate lurch during each wheel tick.
                        // Media dimensions are known before delegate creation,
                        // so this cache no longer triggers decode-time relayouts.
                        cacheBuffer: height * 3
                        spacing: 1
                        // Short conversations grow upward from the composer,
                        // just like long conversations whose tail is visible.
                        // ListView's contentHeight excludes its margins.
                        topMargin: count > 0 ? Math.max(10, height - contentHeight - bottomMargin) : 10
                        bottomMargin: 10
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: OverlayScrollBar {}
                        // In a bottom-to-top chat, Qt's default wheel mapping
                        // points toward the composer. Consume the wheel once
                        // and move toward older rows explicitly; this also
                        // avoids two competing kinetic-scroll animations.
                        WheelHandler {
                            target: null
                            blocking: true
                            onWheel: event => {
                                messageList.releaseTail()
                                messageList.cancelFlick()
                                const pixels = event.pixelDelta.y !== 0
                                    ? event.pixelDelta.y
                                    : event.angleDelta.y * 0.5
                                const historyEdge = messageList.originY
                                const composerEdge = messageList.originY
                                    + messageList.contentHeight - messageList.height
                                const minimum = Math.min(historyEdge, composerEdge)
                                const maximum = Math.max(historyEdge, composerEdge)
                                messageList.contentY = Math.max(minimum,
                                    Math.min(maximum, messageList.contentY - pixels))
                                if (messageList.nearHistoryStart())
                                    olderMessagesTimer.restart()
                            }
                        }
                        delegate: MessageDelegate {
                            navigationHighlighted: String(modelData.id || "") === window.highlightedMessageId
                            selectionActive: window.messageSelectionActive
                            selected: window.messageSelectionActive && window.isMessageSelected(modelData.id)
                            onSelectionToggled: message => window.toggleMessageSelection(message)
                            chatTitle: backend.selectedChat.title || ""
                            chatAvatarSource: Theme.fileUrl(backend.selectedChat.avatar_path)
                            ownTitle: backend.status.user_name || ""
                            onEditRequested: (messageId, body) => {
                                editDialog.messageId = messageId
                                editField.text = body
                                editDialog.open()
                            }
                            onDeleteRequested: (messageId, senderJid) => {
                                deleteDialog.messageId = messageId
                                deleteDialog.senderJid = senderJid
                                deleteDialog.open()
                            }
                            onReplyRequested: (messageId, body) => {
                                window.replyTargetId = messageId
                                window.replyPreview = body
                                composer.forceActiveFocus()
                            }
                            onQuotedMessageRequested: messageId => window.jumpToMessage(messageId)
                            onPinRequested: (messageId, senderJid, body) => {
                                pinDialog.messageId = messageId
                                pinDialog.senderJid = senderJid
                                pinDialog.messagePreview = body
                                pinSevenDays.checked = true
                                pinDialog.open()
                            }
                            onStarRequested: (messageId, senderJid, fromMe, starred) =>
                                backend.starMessage(messageId, senderJid, fromMe, starred)
                            onForwardRequested: messageId => {
                                forwardDialog.messageId = messageId
                                forwardDialog.open()
                            }
                            onImagePreviewRequested: message => window.openChatImage(message)
							onInfoRequested: message => {
								window.infoDrawerOpen = false
								window.messageInfoMessage = message
								window.messageInfoChatJid = String(backend.selectedChat.jid || "")
								window.messageInfoOpen = true
							}
                        }
                        property bool followTail: true
                        property bool positioningTail: false

                        function nearTail() {
                            return contentY >= originY + contentHeight - height - 48
                        }

                        function nearHistoryStart() {
                            return contentY <= originY + 80
                        }

                        function scheduleTailPosition() {
                            if (followTail)
                                tailPositionTimer.restart()
                        }

                        function releaseTail() {
                            tailPositionTimer.stop()
                            // A physical wheel event can arrive in the single
                            // frame between tail positioning and callLater.
                            // It still owns the scroll position: never let the
                            // programmatic-positioning guard ignore it.
                            followTail = false
                        }

                        function prepareForChat(chatJid) {
                            const jid = String(chatJid || "")
                            // selectedChatChanged also reports refreshed titles
                            // and avatars. Only a real conversation change may
                            // request the initial jump to the newest message.
                            if (jid === positionedChatJid)
                                return
                            positionedChatJid = jid
                            tailPositionTimer.stop()
                            followTail = true
                            initialPositionPending = jid !== "" && count === 0
                            if (jid !== "" && count > 0)
                                scheduleTailPosition()
                        }

                        // Geometry can change several times while a batch of
                        // delegates and image previews settles. One position
                        // per event-loop turn avoids visible stop-start jumps.
                        Timer {
                            id: tailPositionTimer
                            interval: 16
                            repeat: false
                            onTriggered: {
                                if (!messageList.followTail)
                                    return
                                messageList.positioningTail = true
                                messageList.positionViewAtBeginning()
                                Qt.callLater(() => {
                                    messageList.positioningTail = false
                                })
                            }
                        }

                        Timer {
                            id: olderMessagesTimer
                            objectName: "olderMessagesTimer"
                            interval: 120
                            repeat: false
                            onTriggered: backend.loadOlderMessages()
                        }

                        // Only physical movement by the reader changes whether
                        // the tail is followed. Layout-driven contentY changes
                        // must not turn following off halfway through a batch.
                        onMovementStarted: {
                            // A wheel gesture may begin while a media resize or
                            // appended row still has a tail update queued. The
                            // reader's gesture always takes precedence.
                            releaseTail()
                        }
                        onContentYChanged: {
                            if (!positioningTail && nearHistoryStart())
                                olderMessagesTimer.restart()
                        }
                        onMovementEnded: {
                            if (!positioningTail)
                                followTail = nearTail()
                            if (followTail)
                                scheduleTailPosition()
                            if (nearHistoryStart())
                                olderMessagesTimer.restart()
                        }
                        onHeightChanged: scheduleTailPosition()
                        onCountChanged: {
                            if (window.pendingMessageJumpId)
                                Qt.callLater(() => window.resolvePendingMessageJump())
                            if (count > 0 && initialPositionPending) {
                                initialPositionPending = false
                                followTail = true
                                scheduleTailPosition()
                            }
                        }

                        Connections {
                            target: backend.messages
                            function onAppended() {
                                messageList.scheduleTailPosition()
                                if (window.infoDrawerOpen) {
                                    backend.refreshChatInfo()
                                    if (contactInfoDrawer.sharedView)
                                        backend.refreshSharedContent(contactInfoDrawer.activeCategory)
                                }
                            }
							function onDataChanged() {
								if (!window.messageInfoOpen || !window.messageInfoMessage.id)
									return
								const refreshed = backend.messageById(String(window.messageInfoMessage.id))
								if (refreshed && refreshed.id)
									window.messageInfoMessage = refreshed
							}
                        }
                    }

                    Connections {
                        target: backend
                        function onTextSendFinished(profile, chatJid, text, replyTo, success) {
                            window.finishTextSend(profile, chatJid, text, replyTo, success)
                        }
                        function onClipboardSendFinished(profile, chatJid, localUrl, replyTo, success) {
                            const key = String(profile) + "\u0000" + String(chatJid)
                            if (key !== mediaPreview.ownerKey || String(localUrl) !== mediaPreview.pendingUrl)
                                return
                            // The original stays available for retry, including
                            // the chosen rotation; temporary rotated copies do not.
                            if (String(localUrl) !== String(mediaPreview.imageUrl))
                                backend.discardClipboardImage(localUrl)
                            mediaPreview.pendingUrl = ""
                            mediaPreview.sending = false
                            if (success) {
                                window.finishImageReply(key, replyTo)
                                mediaPreview.closePreview()
                            }
                        }
                        function onSelectedChatChanged() {
                            if (!window.handleChatSwitched(String(backend.selectedChat.jid || "")))
                                return
                            window.highlightedMessageId = ""
                            messageJumpRetry.stop()
                            messageJumpHighlight.stop()
                            messageList.prepareForChat(backend.selectedChat.jid)
                            if (window.infoDrawerOpen && backend.selectedChat.jid !== window.infoDrawerChatJid) {
                                window.infoDrawerOpen = false
                                backend.clearChatInfo()
                            }
							if (window.messageInfoOpen
									&& backend.selectedChat.jid !== window.messageInfoChatJid) {
								window.messageInfoOpen = false
								window.messageInfoMessage = ({})
								window.messageInfoChatJid = ""
							}
                        }
                    }

                    Column {
                        anchors.centerIn: parent
                        width: Math.min(parent.width - 48, 420)
                        spacing: 6
                        visible: messageList.count === 0

                        Label {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            text: qsTr("No synced messages in this conversation")
                            color: Theme.text
                            font.pixelSize: 16
                            font.weight: Font.Medium
                        }

                        Label {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            text: qsTr("New messages will appear here.")
                            color: Theme.textMuted
                            font.pixelSize: 13
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                    implicitHeight: composerColumn.implicitHeight + 20
                    color: "transparent"

                    ColumnLayout {
                        id: composerColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 12
                        anchors.rightMargin: 12
                        spacing: 6

                        Rectangle {
                            Layout.fillWidth: true
                            visible: Boolean(window.replyTargetId)
                            implicitHeight: replyRow.implicitHeight + 10
                            radius: 10
                            color: Theme.composer
                            border.color: Theme.border
                            RowLayout {
                                id: replyRow
                                anchors.fill: parent
                                anchors.leftMargin: 12
                                anchors.rightMargin: 4
                                Label {
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 0
                                    text: qsTr("Replying to: %1").arg(window.replyPreview)
                                    color: Theme.textMuted
                                    font.pixelSize: 12
                                    elide: Text.ElideRight
                                }
                                ThemedToolButton {
                                    Layout.preferredWidth: 36
                                    Layout.preferredHeight: 36
                                    iconSource: Qt.resolvedUrl("icons/close.svg")
                                    iconSize: 18
                                    Accessible.name: qsTr("Cancel reply")
                                    onClicked: { window.replyTargetId = ""; window.replyPreview = "" }
                                }
                            }
                        }

                        ComposerLinkPreview {
                            Layout.fillWidth: true
                            preview: backend.composerLinkPreview
                            onDismissed: backend.clearComposerLinkPreview()
                        }

                        Rectangle {
                            Layout.fillWidth: true
                            // The box grows with the message and then stops and
                            // scrolls. Sizing it to the text's unbounded height
                            // let a long message inflate it to fill the window,
                            // and a radius of half that turned it into a blob.
                            implicitHeight: Math.max(52, Math.min(composer.implicitHeight, 148) + 8)
                            radius: Math.min(height / 2, 22)
                            color: Theme.composer
                            border.color: Theme.dark ? "transparent" : Theme.border

                            RowLayout {
                                anchors.fill: parent
                                anchors.leftMargin: 6
                                anchors.rightMargin: 6
                                spacing: 4

                                ThemedToolButton {
                                    id: attachmentButton
                                    objectName: "attachmentButton"
                                    Layout.preferredWidth: 44
                                    Layout.preferredHeight: 44
                                    iconSource: Qt.resolvedUrl("icons/attach.svg")
                                    Accessible.name: qsTr("Attach")
                                    Accessible.description: attachmentMenu.opened
                                        ? qsTr("Attachment options open") : qsTr("Attachment options closed")
                                    onClicked: window.toggleAttachmentMenu()
                                    background: Rectangle {
                                        radius: 22
                                        color: parent.hovered || attachmentMenu.opened ? Theme.hoverRow : "transparent"
                                    }
                                    ToolTip.visible: hovered && !attachmentMenu.visible
                                    ToolTip.text: Accessible.name

                                    AttachmentMenu {
                                        id: attachmentMenu
                                        parent: Overlay.overlay
                                        anchorItem: attachmentButton
                                        anchorAlignLeft: true
                                        anchorAbove: true
                                        anchorOffsetX: -8
                                        onDocumentRequested: documentFileDialog.open()
                                        onPhotosVideosRequested: photosVideosFileDialog.open()
                                        onAudioRequested: audioFileDialog.open()
                                        onUnavailableRequested: feature => {
                                            window.transientNotice = qsTr("%1 is not supported yet").arg(feature)
                                            noticeTimer.restart()
                                        }
                                    }
                                }

                                ThemedToolButton {
                                    id: emojiButton
                                    objectName: "emojiButton"
                                    Layout.preferredWidth: 44
                                    Layout.preferredHeight: 44
                                    iconSource: Qt.resolvedUrl("icons/smile.svg")
                                    Accessible.name: qsTr("Choose an emoji")
                                    Accessible.description: emojiPicker.opened ? qsTr("Emoji picker open") : qsTr("Emoji picker closed")
                                    onClicked: {
                                        attachmentMenu.close()
                                        emojiPicker.opened ? emojiPicker.close() : emojiPicker.open()
                                    }
                                    background: Rectangle { radius: 22; color: parent.hovered || emojiPicker.opened ? Theme.hoverRow : "transparent" }
                                    ToolTip.visible: hovered
                                    ToolTip.text: Accessible.name
                                }

                                ScrollView {
                                    objectName: "composerScroll"
                                    Layout.fillWidth: true
                                    Layout.minimumHeight: 44
                                    Layout.maximumHeight: 140
                                    clip: true
                                    ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
                                    ScrollBar.vertical: OverlayScrollBar {}

                                TextArea {
                                    id: composer
                                    objectName: "messageComposer"
                                    leftPadding: 10
                                    rightPadding: 10
                                    topPadding: 11
                                    bottomPadding: 8
                                    placeholderText: qsTr("Type a message")
                                    color: Theme.text
                                    font.family: Theme.isEmojiOnly(text) ? Theme.emojiFontFamily : Application.font.family
                                    font.pixelSize: 14
                                    wrapMode: TextEdit.Wrap
                                    Accessible.name: qsTr("Message")
                                    onTextChanged: {
                                        typingTimer.restart()
                                        linkPreviewTimer.restart()
                                    }
                                    Keys.onPressed: event => {
                                        if (event.matches(StandardKey.Paste) && backend.clipboardHasImage) {
                                            window.prepareClipboardPaste()
                                            event.accepted = true
                                            return
                                        }
                                        // What WhatsApp Web does with Up on an
                                        // empty composer: the last thing this
                                        // account said opens for editing. A
                                        // composer with something in it keeps
                                        // the arrow for moving the cursor.
                                        if (event.key === Qt.Key_Up && event.modifiers === Qt.NoModifier
                                                && composer.text === "") {
                                            if (window.editLastOwnMessage())
                                                event.accepted = true
                                            return
                                        }
                                        // With "Enter is send" off, the roles swap:
                                        // Enter opens a line and Ctrl+Enter sends,
                                        // which is what the PWA does.
                                        const enterSends = settingsPane.enterIsSend
                                        const plainReturn = (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
                                            && !(event.modifiers & Qt.ShiftModifier)
                                            && !(event.modifiers & Qt.ControlModifier)
                                        const controlReturn = (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
                                            && (event.modifiers & Qt.ControlModifier)
                                        if ((enterSends && plainReturn) || (!enterSends && controlReturn)) {
                                            sendButton.clicked()
                                            event.accepted = true
                                        }
                                    }
                                    background: Item {}
                                }
                                }

                                ThemedToolButton {
                                    id: sendButton
                                    objectName: "messageSendButton"
                                    Layout.preferredWidth: 44
                                    Layout.preferredHeight: 44
                                    iconSource: composer.text.trim().length > 0 ? Qt.resolvedUrl("icons/send.svg") : (window.recordingVoice ? Qt.resolvedUrl("icons/stop.svg") : Qt.resolvedUrl("icons/mic.svg"))
                                    iconTint: composer.text.trim().length > 0 ? Theme.primaryText : Theme.icon
                                    Accessible.name: composer.text.trim().length > 0 ? qsTr("Send message") : (window.recordingVoice ? qsTr("Stop and send voice message") : qsTr("Record voice message"))
                                    enabled: !backend.busy
                                    background: Rectangle {
                                        radius: 22
                                        color: composer.text.trim().length > 0 ? Theme.primary : parent.hovered ? Theme.hoverRow : "transparent"
                                    }
                                    onClicked: {
                                        if (!enabled)
                                            return
                                        if (composer.text.trim().length > 0) {
                                            const body = composer.text
                                            const replyTo = window.replyTargetId
                                            window.rememberDraft()
                                            backend.sendMessage(body, replyTo)
                                            backend.setTyping(false)
                                        } else if (window.recordingVoice) {
                                            voiceRecorderLoader.item.stop()
                                        } else {
                                            window.startVoiceRecording()
                                        }
                                    }
                                    ToolTip.visible: hovered
                                    ToolTip.text: Accessible.name
                                }
                            }
                        }
                    }
                    Timer { id: typingTimer; interval: 700; onTriggered: backend.setTyping(composer.text.length > 0) }
                    Timer { id: linkPreviewTimer; interval: 450; onTriggered: backend.requestLinkPreview(composer.text) }
                }
            }

            MediaPreview {
                id: mediaPreview
                property string ownerKey: ""
                property string pendingUrl: ""
                // A preview belongs to its original account/chat. Hide it on
                // navigation; returning there restores it, even after failure.
                visible: previewActive && ownerKey === window.draftHeldKey
                sendAllowed: !backend.busy
                anchors.fill: parent
                anchors.topMargin: 64
                z: 20
                onSendRequested: (imageUrl, caption, rotation) => {
                    const replyTo = window.replyTargetId
                    window.rememberDraft()
                    pendingUrl = backend.rotatedImage(imageUrl, rotation)
                    backend.sendClipboardImage(pendingUrl, caption, replyTo)
                }
                onCanceled: imageUrl => backend.discardClipboardImage(imageUrl)
                onAddRequested: window.prepareClipboardPaste()
            }

            ChatSearchPanel {
                id: chatSearchPanel
                objectName: "chatSearchPanel"
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                width: Math.min(460, parent ? parent.width : 460)
                visible: window.chatSearchOpen
                query: backend.conversationQuery
                results: backend.searchResults
                onCloseRequested: window.closeConversationSearch()
                onQueryEdited: text => backend.searchMessages(text)
                onMessageChosen: messageId => window.jumpToMessage(messageId)
            }

            ContactInfoDrawer {
                id: contactInfoDrawer
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                opened: window.infoDrawerOpen
                selectedChat: backend.selectedChat
                info: backend.chatInfo
                sharedContent: backend.sharedContent
                sharedContentHasMore: backend.sharedContentHasMore
                sharedContentLoading: backend.sharedContentLoading
                onCloseRequested: {
                    window.infoDrawerOpen = false
                    backend.clearChatInfo()
                }
                onSharedRequested: category => backend.refreshSharedContent(category)
                onLoadMoreRequested: category => backend.refreshSharedContent(category, true)
                onSearchRequested: window.openConversationSearch()
                onMuteChanged: muted => {
                    backend.setChatMuted(backend.selectedChat.jid, muted)
                }
                onArchiveChanged: archived => {
                    backend.setChatArchived(backend.selectedChat.jid, archived)
                    window.infoDrawerOpen = false
                    backend.clearChatInfo()
                }
                onFavoriteChanged: favorite => backend.setChatFavorite(backend.selectedChat.jid, favorite)
                onBlockChanged: blocked => backend.setContactBlocked(backend.selectedChat.jid, blocked)
                onDisappearingRequested: disappearingDialog.open()
                onStarredRequested: starredMessagesDialog.openForChat(backend.selectedChat.jid)
                onExportRequested: exportChatDialog.open()
                onClearRequested: conversationClearDialog.open()
                onDeleteRequested: conversationDeleteDialog.open()
                onOpenFileRequested: path => backend.openFile(path)
                onImagePreviewRequested: message => window.openChatImage(message)
                onAvatarPreviewRequested: (path, title) => {
                    chatMediaViewer.openImage(window.localMediaUrl(path), title,
                                              window.localMediaUrl(path), "", "", "")
                }
                onDownloadRequested: messageId => backend.downloadMedia(messageId)
                onOpenLinkRequested: url => {
                    if (url)
                        Qt.openUrlExternally(url)
                }
            }

			MessageInfoDrawer {
				id: messageInfoDrawer
				anchors.top: parent.top
				anchors.right: parent.right
				anchors.bottom: parent.bottom
				opened: window.messageInfoOpen
				message: window.messageInfoMessage
				onCloseRequested: {
					window.messageInfoOpen = false
					window.messageInfoMessage = ({})
					window.messageInfoChatJid = ""
				}
			}
        }

        FeatureSection {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: window.activeSection !== "chats" && window.activeSection !== "status"
                && window.activeSection !== "media" && window.activeSection !== "profile"
            section: window.activeSection
            onCreateChannelRequested: newChannelDialog.open()
            onFollowChannelRequested: followChannelDialog.open()
            onCreateCommunityRequested: newCommunityDialog.open()
        }

        SettingsPane {
            id: settingsPane
            objectName: "settingsPane"
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: window.activeSection === "profile"
            onLogoutRequested: logoutDialog.open()
            onShortcutsRequested: shortcutsDialog.open()
            onAppearanceRequested: Theme.preferredMode = Theme.dark ? "light" : "dark"
            onBugReportRequested: {
                backend.refreshBugReportEnvironment()
                bugReportDialog.reset()
                bugReportDialog.open()
            }
        }

        MediaLibraryPane {
            objectName: "mediaLibraryPane"
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: window.activeSection === "media"
            onMessageRequested: (chatJid, messageId) => {
                if (chatJid === "")
                    return
                window.showSection("chats")
                window.jumpToMessageInChat(chatJid, "", messageId)
            }
            onCloseRequested: window.showSection("chats")
            onForwardRequested: items => {
                // Items from the media browser carry their own chat, so they are
                // forwarded from where they were found rather than from whatever
                // conversation happens to be open.
                forwardDialog.messagePairs = items
                forwardDialog.messageIds = []
                forwardDialog.open()
            }
        }

        StatusPage {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: window.activeSection === "status"
            groups: backend.statusUpdates
            ownName: backend.status.user_name || backend.profile
            onGroupRequested: index => {
                window.openStatusAt(index)
            }
            onAvatarRequested: jid => backend.fetchStatusAvatar(jid)
            onTextStatusRequested: textStatusDialog.open()
            onPhotoStatusRequested: statusMediaDialog.open()
            onStatusPrivacyRequested: {
                window.showSection("profile")
                settingsPane.openSection = "privacy"
            }
        }
    }

    ChatMediaViewer {
        id: chatMediaViewer
        anchors.fill: parent
        z: 120
    }

    Loader {
        id: statusViewerLoader
        anchors.fill: parent
        z: 100
        active: window.activeSection === "status" || window.statusViewerRequested
        sourceComponent: StatusViewer {
            groups: backend.statusUpdates
            onCloseRequested: window.statusViewerRequested = false
            onMediaRequested: messageId => backend.ensureStatusMedia(messageId)
            onReplyRequested: (recipientJid, statusMessageId, text) =>
                backend.sendStatusReply(recipientJid, statusMessageId, text)
        }
    }

    Connections {
        target: backend
        function onStatusReplyFinished(recipientJid, statusMessageId, success, message) {
            if (statusViewerLoader.item)
                statusViewerLoader.item.finishReply(recipientJid, statusMessageId, success, message)
        }
    }

    // Videos and voice notes play inside the window. Handing them to the
    // desktop opened a web browser on systems with no registered player.
    //
    // The overlay is loaded only while a video plays: declaring a VideoOutput
    // up front starts the FFmpeg backend during application startup, which
    // prints hardware-decoder probing warnings before anything is played.
    Loader {
        id: videoOverlay
        objectName: "videoOverlay"
        anchors.fill: parent
        z: 60
        active: Playback.videoActive
        visible: active
        sourceComponent: Rectangle {
            color: "#F2000000"

            Component.onCompleted: Playback.videoSurface = videoSurface
            Component.onDestruction: Playback.videoSurface = null

            MouseArea {
                anchors.fill: parent
                onClicked: Playback.toggle()
            }

            // VideoSurface rather than VideoOutput: the software scene graph
            // this application runs on cannot draw a VideoOutput, so every
            // video played to a black screen. See src/videosurface.h.
            VideoSurface {
                id: videoSurface
                anchors.fill: parent
                anchors.margins: 24
                anchors.bottomMargin: 84
            }

            ThemedToolButton {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: 12
                width: 44
                height: 44
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconTint: "#FFFFFF"
                Accessible.name: qsTr("Close the video")
                background: Rectangle { radius: 22; color: parent.hovered ? "#33FFFFFF" : "transparent" }
                onClicked: Playback.stop()
            }

            RowLayout {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: 20
                spacing: 12

                ThemedToolButton {
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    Accessible.name: Playback.playing ? qsTr("Pause") : qsTr("Play")
                    contentItem: Label {
                        text: Playback.playing ? "❚❚" : "▶"
                        color: "#FFFFFF"
                        font.pixelSize: 17
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle { radius: 22; color: parent.hovered ? "#33FFFFFF" : "transparent" }
                    onClicked: Playback.toggle()
                }

                Item {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 16
                    Rectangle {
                        id: videoTrack
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        height: 4
                        radius: 2
                        color: "#55FFFFFF"
                        Rectangle {
                            width: parent.width * (Playback.duration > 0 ? Playback.position / Playback.duration : 0)
                            height: parent.height
                            radius: parent.radius
                            color: Theme.primary
                        }
                    }
                    MouseArea {
                        anchors.fill: parent
                        enabled: Playback.duration > 0
                        onClicked: mouse => Playback.seek(Playback.duration * (mouse.x / Math.max(1, width)))
                    }
                }

                Label {
                    text: {
                        const clock = value => {
                            const total = Math.max(0, Math.round(value / 1000))
                            const minutes = Math.floor(total / 60)
                            const seconds = total % 60
                            return minutes + ":" + (seconds < 10 ? "0" : "") + seconds
                        }
                        return clock(Playback.position) + " / " + clock(Playback.duration)
                    }
                    color: "#FFFFFF"
                    font.pixelSize: 12
                }
            }

            Keys.onEscapePressed: Playback.stop()
        }
    }

    Connections {
        target: Playback
        function onDownloadRequested(messageId) {
            backend.downloadMedia(messageId)
        }
        function onFailed(message) {
            window.transientError = message
            errorTimer.restart()
        }
        function onFinished(messageId) {
            const next = backend.nextAudioAfter(messageId)
            if (next && next.id)
                Playback.start(next.id, next.media_path || "", false)
        }
		function onStarted(messageId) {
			backend.markMediaPlayed(messageId)
		}
    }

    Rectangle {
        visible: Boolean(window.transientError)
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 24
        width: Math.min(errorLabel.implicitWidth + 32, parent.width - 32)
        height: errorLabel.implicitHeight + 20
        radius: Theme.radiusMedium
        color: Theme.danger
        z: 100
        Label {
            id: errorLabel
            objectName: "errorBannerLabel"
            anchors.centerIn: parent
            width: parent.width - 24
            text: window.transientError
            color: Theme.dangerText
            wrapMode: Text.Wrap
            // A failure from the daemon can carry a whole download address.
            // Three lines is enough to say what went wrong; the rest would
            // paint a wall across the conversation.
            maximumLineCount: 3
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignHCenter
            ToolTip.visible: bannerHover.hovered && truncated
            ToolTip.text: window.transientError
            HoverHandler { id: bannerHover }
        }
    }

    Rectangle {
        visible: Boolean(window.transientNotice) && !Boolean(window.transientError)
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 24
        width: Math.min(noticeLabel.implicitWidth + 32, parent.width - 32)
        height: noticeLabel.implicitHeight + 20
        radius: Theme.radiusMedium
        color: Theme.primary
        z: 100
        Label {
            id: noticeLabel
            anchors.centerIn: parent
            width: parent.width - 24
            text: window.transientNotice
            color: Theme.primaryText
            wrapMode: Text.Wrap
            horizontalAlignment: Text.AlignHCenter
        }
    }

    EmojiPicker {
        id: emojiPicker
        parent: emojiButton
        x: emojiButton.width - width
        y: -height - 10
        onEmojiChosen: emoji => window.insertComposerEmoji(emoji)
    }

    FileDialog {
        id: exportChatDialog
        objectName: "exportChatDialog"
        title: qsTr("Export chat")
        fileMode: FileDialog.SaveFile
        nameFilters: [qsTr("Text files (*.txt)"), qsTr("All files (*)")]
        // Named after the conversation, the way WhatsApp names its own exports.
        currentFile: StandardPaths.writableLocation(StandardPaths.DocumentsLocation)
            + "/WhatsApp Chat - " + String(backend.selectedChat.title || backend.selectedChat.jid || "chat").replace(/[\/]/g, " ") + ".txt"
        onAccepted: backend.exportChat(backend.selectedChat.jid, selectedFile)
    }

    FileDialog {
        id: documentFileDialog
        title: qsTr("Choose a document")
        nameFilters: [qsTr("All files (*)")]
        onAccepted: window.sendAttachment(selectedFile, "", true)
    }
    FileDialog {
        id: photosVideosFileDialog
        title: qsTr("Choose photos or videos")
        nameFilters: [
            qsTr("Photos and videos (*.jpg *.jpeg *.png *.gif *.webp *.bmp *.heic *.heif *.mp4 *.mov *.mkv *.webm *.avi *.3gp)"),
            qsTr("All files (*)")
        ]
        onAccepted: window.sendAttachment(selectedFile)
    }
    FileDialog {
        id: audioFileDialog
        title: qsTr("Choose audio")
        nameFilters: [
            qsTr("Audio files (*.mp3 *.m4a *.ogg *.opus *.wav *.aac *.flac)"),
            qsTr("All files (*)")
        ]
        onAccepted: window.sendAttachment(selectedFile)
    }
    // The keyboard shortcuts the settings dialog lists. They are declared here so
    // that list describes something real rather than copying the PWA's.
    Shortcut {
        sequences: [StandardKey.Find]
        onActivated: {
            if (!backend.selectedChat.jid)
                return
            window.showSection("chats")
            window.openChatSearch()
        }
    }
    Shortcut {
        sequence: "Ctrl+Shift+F"
        onActivated: window.openConversationSearch()
    }
    Shortcut {
        sequence: "Ctrl+N"
        onActivated: { window.showSection("chats"); window.newChatOpen = true }
    }
    Shortcut {
        sequence: "Ctrl+Shift+N"
        onActivated: newGroupDialog.open()
    }
    Shortcut {
        sequence: "Ctrl+E"
        onActivated: {
            if (backend.selectedChat.jid)
                backend.setChatArchived(backend.selectedChat.jid, !backend.selectedChat.archived)
        }
    }
    Shortcut {
        sequence: "Ctrl+Shift+M"
        onActivated: {
            if (!backend.selectedChat.jid)
                return
            const muted = Number(backend.selectedChat.muted_until || 0) > Date.now()
            backend.setChatMuted(backend.selectedChat.jid, !muted, 0)
        }
    }
    Shortcut {
        sequence: "Ctrl+Shift+U"
        onActivated: {
            if (backend.selectedChat.jid)
                backend.setChatRead(backend.selectedChat.jid, false)
        }
    }

    WhatsAppDialog {
        id: newChannelDialog
        objectName: "newChannelDialog"
        title: qsTr("New channel")
        subtitle: qsTr("A channel is public: anyone with its link can follow it and see what you post.")
        acceptText: qsTr("Create")
        preferredWidth: 440
        acceptEnabled: newChannelName.text.trim() !== ""
        onOpened: {
            newChannelName.text = ""
            newChannelDescription.text = ""
            newChannelName.forceActiveFocus()
        }
        onAccepted: backend.createChannel(newChannelName.text, newChannelDescription.text)
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            DialogTextField {
                id: newChannelName
                objectName: "newChannelNameField"
                Layout.fillWidth: true
                maximumLength: 75
                placeholderText: qsTr("Channel name")
            }
            DialogTextField {
                id: newChannelDescription
                objectName: "newChannelDescriptionField"
                Layout.fillWidth: true
                maximumLength: 500
                placeholderText: qsTr("Description (optional)")
            }
        }
    }

    // A newer release exists. Nothing has been downloaded yet: this asks first,
    // because an update is a hundred megabytes on somebody else's connection.
    WhatsAppDialog {
        id: updatePromptDialog
        objectName: "updateAvailableDialog"
        title: qsTr("Update WhatsAppGo")
        subtitle: {
            const status = backend.updateStatus
            const latest = String(status.latest || "")
            const current = String(status.current || "")
            return backend.updateInstallable()
                ? qsTr("Version %1 is out. This copy is %2.").arg(latest).arg(current)
                : qsTr("Version %1 is out. This copy is %2, and it was not installed in a way that can replace itself, so the download is on the release page.").arg(latest).arg(current)
        }
        acceptText: backend.updateInstallable() ? qsTr("Download it") : qsTr("Open the release page")
        cancelText: qsTr("Not now")
        preferredWidth: 460
        onAccepted: {
            if (backend.updateInstallable())
                backend.downloadUpdate()
            else
                backend.openReleasePage()
        }
    }

    // The file is downloaded and checked. Installing it closes this window, so
    // it happens when the reader says so and not in the middle of a message.
    WhatsAppDialog {
        id: updateReadyDialog
        objectName: "updateReadyDialog"
        title: qsTr("The update is ready")
        subtitle: qsTr("Version %1 has been downloaded. Installing it closes WhatsAppGo and opens the new version.")
            .arg(String(backend.updateStatus.latest || ""))
        acceptText: qsTr("Install and restart")
        cancelText: qsTr("Later")
        preferredWidth: 460
        onAccepted: backend.installUpdate()
    }

    WhatsAppDialog {
        id: followChannelDialog
        objectName: "followChannelDialog"
        title: qsTr("Follow a channel")
        subtitle: qsTr("Paste a channel link. There is no channel directory to browse from here, so a link is how a channel is found.")
        acceptText: qsTr("Follow")
        preferredWidth: 440
        acceptEnabled: followChannelLink.text.trim() !== ""
        onOpened: {
            followChannelLink.text = ""
            followChannelLink.forceActiveFocus()
        }
        onAccepted: backend.followChannelLink(followChannelLink.text)
        DialogTextField {
            id: followChannelLink
            objectName: "followChannelLinkField"
            Layout.fillWidth: true
            placeholderText: qsTr("https://whatsapp.com/channel/…")
            onAccepted: if (followChannelDialog.acceptEnabled) followChannelDialog.accept()
        }
    }

    WhatsAppDialog {
        id: newCommunityDialog
        objectName: "newCommunityDialog"
        title: qsTr("New community")
        subtitle: qsTr("A community groups related chats together. WhatsApp adds its announcement group for you.")
        acceptText: qsTr("Create")
        preferredWidth: 440
        acceptEnabled: newCommunityName.text.trim() !== ""
        onOpened: {
            newCommunityName.text = ""
            newCommunityName.forceActiveFocus()
        }
        onAccepted: backend.createCommunity(newCommunityName.text)
        DialogTextField {
            id: newCommunityName
            objectName: "newCommunityNameField"
            Layout.fillWidth: true
            maximumLength: 100
            placeholderText: qsTr("Community name")
            onAccepted: if (newCommunityDialog.acceptEnabled) newCommunityDialog.accept()
        }
    }

    WhatsAppDialog {
        id: joinGroupDialog
        objectName: "joinGroupDialog"
        title: qsTr("Join a group with a link")
        subtitle: qsTr("Paste an invite link. You will join the group and it opens here.")
        acceptText: qsTr("Join")
        preferredWidth: 440
        acceptEnabled: joinGroupLink.text.trim() !== ""
        onOpened: {
            joinGroupLink.text = ""
            joinGroupLink.forceActiveFocus()
        }
        onAccepted: backend.joinGroupLink(joinGroupLink.text)
        DialogTextField {
            id: joinGroupLink
            objectName: "joinGroupLinkField"
            Layout.fillWidth: true
            placeholderText: qsTr("https://chat.whatsapp.com/…")
            onAccepted: if (joinGroupDialog.acceptEnabled) joinGroupDialog.accept()
        }
    }

    WhatsAppDialog {
        id: textStatusDialog
        objectName: "textStatusDialog"
        title: qsTr("Text status")
        subtitle: qsTr("Everyone who can see your status will see this for 24 hours.")
        acceptText: qsTr("Post")
        preferredWidth: 460
        acceptEnabled: statusTextField.text.trim() !== ""
        property int backgroundChoice: 0
        onOpened: {
            statusTextField.text = ""
            backgroundChoice = 0
            statusTextField.forceActiveFocus()
        }
        onAccepted: backend.postTextStatus(statusTextField.text, backgroundChoice)
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 12
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 120
                radius: 10
                // The chosen colour is shown behind the text, the way the status
                // itself will look.
                color: textStatusDialog.backgroundChoice === 0 ? "#128C7E"
                    : textStatusDialog.backgroundChoice === 1 ? "#25D366"
                    : textStatusDialog.backgroundChoice === 2 ? "#E91E63"
                    : textStatusDialog.backgroundChoice === 3 ? "#0B84CC" : "#C0C0C0"
                TextArea {
                    id: statusTextField
                    objectName: "statusTextField"
                    anchors.fill: parent
                    anchors.margins: 12
                    background: null
                    color: "#FFFFFF"
                    font.pixelSize: 18
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                    wrapMode: TextEdit.Wrap
                    selectByMouse: true
                    Accessible.name: qsTr("Status text")
                }
                // The style's own placeholder is drawn in a colour that cannot be
                // read on these backgrounds, so the hint is drawn here instead.
                Label {
                    anchors.centerIn: parent
                    visible: statusTextField.text === ""
                    text: qsTr("Type a status update")
                    color: "#CCFFFFFF"
                    font.pixelSize: 18
                }
            }
            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                Repeater {
                    model: ["#128C7E", "#25D366", "#E91E63", "#0B84CC", "#C0C0C0"]
                    AbstractButton {
                        required property string modelData
                        required property int index
                        objectName: "statusBackgroundSwatch"
                        implicitWidth: 32
                        implicitHeight: 32
                        Accessible.name: qsTr("Background %1").arg(index + 1)
                        onClicked: textStatusDialog.backgroundChoice = index
                        background: Rectangle {
                            radius: 16
                            color: modelData
                            border.width: textStatusDialog.backgroundChoice === index ? 3 : 0
                            border.color: Theme.text
                        }
                    }
                }
                Item { Layout.fillWidth: true }
            }
        }
    }

    FileDialog {
        id: statusMediaDialog
        objectName: "statusMediaDialog"
        title: qsTr("Choose a photo or video for your status")
        nameFilters: [
            qsTr("Photos and videos (*.jpg *.jpeg *.png *.webp *.mp4 *.mov *.mkv *.webm)"),
            qsTr("All files (*)")
        ]
        onAccepted: backend.postMediaStatus(selectedFile, "")
    }

    WhatsAppDialog {
        id: shortcutsDialog
        objectName: "keyboardShortcutsDialog"
        title: qsTr("Keyboard shortcuts")
        preferredWidth: 460
        preferredHeight: 520
        showAccept: false
        cancelText: qsTr("Close")
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0
            Repeater {
                // Only shortcuts this client actually binds. A list copied from
                // the PWA would promise keys that do nothing here.
                model: [
                    { keys: "Ctrl+F", label: qsTr("Search this conversation") },
                    { keys: "Ctrl+Shift+F", label: qsTr("Search message history") },
                    { keys: "Ctrl+N", label: qsTr("New chat") },
                    { keys: "Ctrl+Shift+N", label: qsTr("New group") },
                    { keys: "Ctrl+E", label: qsTr("Archive chat") },
                    { keys: "Ctrl+Shift+M", label: qsTr("Mute chat") },
                    { keys: "Ctrl+Shift+U", label: qsTr("Mark as unread") },
                    { keys: "Esc", label: qsTr("Close the open panel") },
                    { keys: settingsPane.enterIsSend ? "Enter" : "Ctrl+Enter", label: qsTr("Send the message") },
                    { keys: settingsPane.enterIsSend ? "Shift+Enter" : "Enter", label: qsTr("New line in the message") }
                ]
                RowLayout {
                    required property var modelData
                    Layout.fillWidth: true
                    Layout.preferredHeight: 38
                    spacing: 12
                    Label {
                        Layout.fillWidth: true
                        text: modelData.label
                        color: Theme.text
                        font.pixelSize: 14
                        elide: Text.ElideRight
                    }
                    Rectangle {
                        Layout.preferredWidth: shortcutKeys.implicitWidth + 16
                        Layout.preferredHeight: 24
                        radius: 6
                        color: Theme.surfaceMuted
                        Label {
                            id: shortcutKeys
                            anchors.centerIn: parent
                            text: modelData.keys
                            color: Theme.textMuted
                            font.pixelSize: 12
                        }
                    }
                }
            }
            Item { Layout.fillHeight: true }
        }
    }

    WhatsAppDialog {
        id: logoutDialog
        objectName: "logoutDialog"
        title: qsTr("Unlink this computer?")
        subtitle: qsTr("Local message history remains until you remove the application data.")
        acceptText: qsTr("Unlink")
        destructive: true
        onAccepted: backend.logout()
    }
    WhatsAppDialog {
        id: starredMessagesDialog
        objectName: "starredMessagesDialog"
        title: qsTr("Starred messages")
        preferredWidth: 640
        preferredHeight: 560
        showAccept: false
        cancelText: qsTr("Close")
        // The list is read on open rather than kept in sync: stars change
        // rarely, and a stale list would be worse than a short wait.
        property string chatJid: ""
        readonly property bool loading: backend.starredMessagesLoading
        function openForChat(jid) {
            chatJid = String(jid || "")
            open()
        }
        onAboutToShow: backend.loadStarredMessages(chatJid)
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Label {
                objectName: "starredMessagesEmpty"
                Layout.fillWidth: true
                Layout.margins: 12
                // Only say the list is empty once it has actually been read;
                // the round trip would otherwise flash the wrong answer.
                visible: !starredMessagesDialog.loading && backend.starredMessages.length === 0
                wrapMode: Text.Wrap
                color: Theme.textMuted
                text: qsTr("Nothing is starred yet. Star a message to keep it here.")
                Accessible.name: text
            }
            ListView {
                objectName: "starredMessagesList"
                Layout.fillWidth: true
                Layout.fillHeight: true
                visible: backend.starredMessages.length > 0
                model: backend.starredMessages
                clip: true
                reuseItems: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: OverlayScrollBar {}
                delegate: ItemDelegate {
                    required property var modelData
                    width: ListView.view.width
                    height: 56
                    // A starred message is only meaningful with the chat it
                    // came from: the same words appear in many conversations.
                    contentItem: ColumnLayout {
                        spacing: 1
                        Label {
                            Layout.fillWidth: true
                            text: modelData.chat_title || modelData.chat_jid || ""
                            color: Theme.text
                            font.pixelSize: 13
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            text: modelData.body || String(modelData.media_name || modelData.kind || "")
                            color: Theme.textMuted
                            font.pixelSize: 12
                            elide: Text.ElideRight
                        }
                    }
                    Accessible.name: qsTr("%1: %2").arg(modelData.chat_title || modelData.chat_jid || "")
                        .arg(modelData.body || "")
                    onClicked: {
                        backend.openChat(modelData.chat_jid, modelData.chat_title || modelData.chat_jid)
                        starredMessagesDialog.close()
                    }
                }
            }
        }
    }
    Popup {
        id: addAccountDialog
        parent: Overlay.overlay
        anchors.centerIn: parent
        width: Math.min(430, window.width - 48)
        height: accountDialogContent.implicitHeight + 48
        modal: true
        focus: true
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
        onOpened: {
            accountName.clear()
            accountName.forceActiveFocus()
        }
        background: Item {
            Rectangle {
                anchors.fill: parent
                anchors.leftMargin: 3
                anchors.topMargin: 6
                radius: 16
                color: Theme.dark ? "#66000000" : "#26000000"
            }
            Rectangle {
                anchors.fill: parent
                anchors.rightMargin: 3
                anchors.bottomMargin: 6
                radius: 16
                color: Theme.surfaceRaised
                border.color: Theme.border
            }
        }
        contentItem: ColumnLayout {
            id: accountDialogContent
            spacing: 16

            Label {
                Layout.fillWidth: true
                text: qsTr("Add another account")
                color: Theme.text
                font.pixelSize: 22
                font.weight: Font.DemiBold
            }
            Label {
                Layout.fillWidth: true
                text: qsTr("Give this account a short name. You will link its WhatsApp number on the next screen.")
                color: Theme.textMuted
                font.pixelSize: 14
                wrapMode: Text.Wrap
                lineHeight: 1.3
            }
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 50
                radius: 10
                color: Theme.surfaceMuted
                border.width: accountName.activeFocus ? 2 : 1
                border.color: accountName.activeFocus ? Theme.primary : Theme.border
                TextField {
                    id: accountName
                    anchors.fill: parent
                    leftPadding: 14
                    rightPadding: 14
                    placeholderText: qsTr("Account name, e.g. Work")
                    color: Theme.text
                    font.pixelSize: 15
                    Accessible.name: qsTr("New account profile name")
                    background: Item {}
                    onAccepted: if (text.trim()) {
                        backend.addProfile(text.trim())
                        addAccountDialog.close()
                    }
                }
            }
            RowLayout {
                Layout.alignment: Qt.AlignRight
                spacing: 8
                Button {
                    Layout.preferredWidth: 96
                    Layout.preferredHeight: 44
                    text: qsTr("Cancel")
                    flat: true
                    onClicked: addAccountDialog.close()
                }
                Button {
                    Layout.preferredWidth: 108
                    Layout.preferredHeight: 44
                    text: qsTr("Continue")
                    enabled: Boolean(accountName.text.trim())
                    contentItem: Label {
                        text: parent.text
                        color: parent.enabled ? Theme.primaryText : Theme.textMuted
                        font.pixelSize: 14
                        font.weight: Font.DemiBold
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle {
                        radius: 22
                        color: parent.enabled ? Theme.primary : Theme.surfaceMuted
                    }
                    onClicked: {
                        backend.addProfile(accountName.text.trim())
                        addAccountDialog.close()
                    }
                }
            }
        }
    }
    WhatsAppDialog {
        id: removeAccountDialog
        objectName: "removeAccountDialog"
        property string profile: ""
        readonly property string profileName: String(backend.profileDisplayNames[profile] || profile)
        title: qsTr("Remove %1?").arg(profileName)
        subtitle: qsTr("The account is unlinked from this computer and its messages, media and login are deleted from it. Your WhatsApp account itself is not affected.")
        acceptText: qsTr("Remove")
        destructive: true
        onAccepted: backend.removeProfile(profile)
    }

    BugReportDialog {
        id: bugReportDialog
        onSubmitRequested: (subject, details) => backend.submitBugReport(subject, details)
    }

    WhatsAppDialog {
        id: renameAccountDialog
        objectName: "renameAccountDialog"
        property string profile: ""
        property string initialName: ""
        title: qsTr("Rename account")
        subtitle: qsTr("This changes only the local label. Your WhatsApp name and account data stay unchanged.")
        preferredWidth: 430
        acceptText: qsTr("Save")
        acceptEnabled: renameAccountName.text.trim() !== ""
        onOpened: {
            renameAccountName.text = initialName
            renameAccountName.forceActiveFocus()
            renameAccountName.selectAll()
        }
        onAccepted: backend.renameProfile(profile, renameAccountName.text)
        DialogTextField {
            id: renameAccountName
            Layout.fillWidth: true
            placeholderText: qsTr("Account display name")
            maximumLength: 64
            onAccepted: if (text.trim()) renameAccountDialog.accept()
        }
    }
    WhatsAppDialog {
        id: pinDialog
        objectName: "pinMessageDialog"
        property string messageId: ""
        property string senderJid: ""
        property string messagePreview: ""
        title: qsTr("Choose how long to pin this message")
        subtitle: qsTr("You can unpin it at any time.")
        acceptText: qsTr("Pin")
        onAccepted: {
            let duration = 7 * 24 * 60 * 60
            if (pinOneDay.checked)
                duration = 24 * 60 * 60
            else if (pinThirtyDays.checked)
                duration = 30 * 24 * 60 * 60
            backend.pinMessage(messageId, senderJid, duration)
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            ButtonGroup { id: pinDurationGroup }
            DialogRadioButton {
                id: pinOneDay
                Layout.fillWidth: true
                text: qsTr("24 hours")
                ButtonGroup.group: pinDurationGroup
            }
            DialogRadioButton {
                id: pinSevenDays
                Layout.fillWidth: true
                text: qsTr("7 days")
                checked: true
                ButtonGroup.group: pinDurationGroup
            }
            DialogRadioButton {
                id: pinThirtyDays
                Layout.fillWidth: true
                text: qsTr("30 days")
                ButtonGroup.group: pinDurationGroup
            }
        }
    }
    WhatsAppDialog {
        id: newListDialog
        objectName: "newListDialog"
        acceptName: "newListCreateAction"
        title: qsTr("New list")
        subtitle: qsTr("Lists group chats together and sync to your other devices.")
        acceptText: qsTr("Create")
        acceptEnabled: newListName.text.trim() !== ""
        onOpened: {
            newListName.text = ""
            newListName.forceActiveFocus()
        }
        onAccepted: backend.createChatLabel(newListName.text.trim())
        DialogTextField {
            id: newListName
            objectName: "newListNameField"
            Layout.fillWidth: true
            placeholderText: qsTr("List name")
            onAccepted: if (newListDialog.acceptEnabled) newListDialog.accept()
        }
    }
    WhatsAppDialog {
        id: disappearingDialog
        objectName: "disappearingMessagesDialog"
        title: qsTr("Disappearing messages")
        subtitle: qsTr("New messages in this chat disappear after the chosen time. Messages already sent are not affected.")
        acceptText: qsTr("Save")
        // Whatever WhatsApp currently keeps for the chat is what the dialog
        // opens on, so saving without touching anything changes nothing.
        onOpened: {
            const current = Number(backend.selectedChat.disappearing_seconds || 0)
            disappearingOff.checked = current === 0
            disappearingDay.checked = current === 86400
            disappearingWeek.checked = current === 604800
            disappearingQuarter.checked = current === 7776000
            if (!disappearingOff.checked && !disappearingDay.checked
                    && !disappearingWeek.checked && !disappearingQuarter.checked)
                disappearingOff.checked = true
        }
        onAccepted: {
            let seconds = 0
            if (disappearingDay.checked)
                seconds = 86400
            else if (disappearingWeek.checked)
                seconds = 604800
            else if (disappearingQuarter.checked)
                seconds = 7776000
            backend.setChatDisappearing(backend.selectedChat.jid, seconds)
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            ButtonGroup { id: disappearingGroup }
            DialogRadioButton {
                id: disappearingQuarter
                Layout.fillWidth: true
                text: qsTr("90 days")
                ButtonGroup.group: disappearingGroup
            }
            DialogRadioButton {
                id: disappearingWeek
                Layout.fillWidth: true
                text: qsTr("7 days")
                ButtonGroup.group: disappearingGroup
            }
            DialogRadioButton {
                id: disappearingDay
                Layout.fillWidth: true
                text: qsTr("24 hours")
                ButtonGroup.group: disappearingGroup
            }
            DialogRadioButton {
                id: disappearingOff
                Layout.fillWidth: true
                checked: true
                text: qsTr("Off")
                ButtonGroup.group: disappearingGroup
            }
        }
    }

    WhatsAppDialog {
        id: conversationClearDialog
        objectName: "conversationClearDialog"
        title: qsTr("Clear this chat?")
        subtitle: qsTr("The messages are removed from this computer and from your other devices. The chat itself stays in the list.")
        acceptText: qsTr("Clear chat")
        destructive: true
        onAccepted: backend.clearChat(backend.selectedChat.jid)
    }

    WhatsAppDialog {
        id: conversationDeleteDialog
        objectName: "conversationDeleteDialog"
        title: qsTr("Delete this chat?")
        subtitle: qsTr("The conversation is removed from this computer and from your other devices. This cannot be undone.")
        acceptText: qsTr("Delete")
        destructive: true
        onAccepted: backend.deleteChat(backend.selectedChat.jid)
    }
    WhatsAppDialog {
        id: newGroupDialog
        objectName: "newGroupDialog"
        acceptName: "newGroupCreateAction"
        // The members come from the chat list rather than a free-text field:
        // a group is made from people already known here, and a mistyped JID
        // would silently invite a stranger.
        property var selectedJids: []
        title: qsTr("New group")
        preferredWidth: 440
        acceptText: qsTr("Create")
        acceptEnabled: newGroupName.text.trim() !== "" && newGroupDialog.selectedJids.length > 0
        onAccepted: backend.createGroup(newGroupName.text.trim(), newGroupDialog.selectedJids)
        onOpened: {
            newGroupName.text = ""
            newGroupFilter.text = ""
            selectedJids = []
            newGroupName.forceActiveFocus()
        }
        function toggleMember(jid) {
            const next = newGroupDialog.selectedJids.slice()
            const at = next.indexOf(jid)
            if (at >= 0)
                next.splice(at, 1)
            else
                next.push(jid)
            newGroupDialog.selectedJids = next
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            DialogTextField {
                id: newGroupName
                objectName: "newGroupNameField"
                Layout.fillWidth: true
                // WhatsApp rejects longer names with a 406, so the field stops
                // before the request does.
                maximumLength: 25
                placeholderText: qsTr("Group name")
            }
            DialogTextField {
                id: newGroupFilter
                objectName: "newGroupFilter"
                Layout.fillWidth: true
                search: true
                placeholderText: qsTr("Search chats")
            }
            ListView {
                objectName: "newGroupMemberList"
                Layout.fillWidth: true
                Layout.preferredWidth: 360
                Layout.preferredHeight: 300
                clip: true
                model: backend.chatListModel
                delegate: ItemDelegate {
                    required property var modelData
                    width: ListView.view ? ListView.view.width : 0
                    readonly property string jid: String(modelData.jid || "")
                    readonly property bool matches: newGroupFilter.text.trim() === ""
                        || String(modelData.title || "").toLowerCase()
                            .indexOf(newGroupFilter.text.trim().toLowerCase()) >= 0
                    // Only people can join a group; another group cannot.
                    visible: matches && !modelData.is_group
                    height: visible ? 48 : 0
                    text: modelData.title || jid
                    Accessible.name: text
                    Accessible.checked: newGroupDialog.selectedJids.indexOf(jid) >= 0
                    highlighted: newGroupDialog.selectedJids.indexOf(jid) >= 0
                    onClicked: newGroupDialog.toggleMember(jid)
                }
            }
            Label {
                objectName: "newGroupSummaryLabel"
                Layout.fillWidth: true
                elide: Text.ElideRight
                color: Theme.textMuted
                text: newGroupDialog.selectedJids.length === 0
                    ? qsTr("Name the group and pick who is in it.")
                    : qsTr("%1 selected").arg(newGroupDialog.selectedJids.length)
                Accessible.name: text
            }
        }
    }
    WhatsAppDialog {
        id: forwardDialog
        objectName: "forwardMessageDialog"
        acceptName: "forwardSendAction"
        property string messageId: ""
        // A batch from the selection bar; the single-message path leaves it
        // empty and uses messageId.
        property var messageIds: []
        // Pairs of {id, chat_jid} from the media browser, where the source chat
        // is not the open one.
        property var messagePairs: []
        property string targetJid: ""
        property string targetTitle: ""
        title: qsTr("Forward to")
        // Forwarding needs a destination, so the dialog offers the chat list
        // rather than a free-text field: a mistyped JID would send a private
        // message to the wrong person.
        //
        // Picking a chat only selects it. Sending is a second, explicit press,
        // because a stray click in a filtered list would otherwise put someone
        // else's message in front of the wrong contact with no way back.
        preferredWidth: 440
        acceptText: qsTr("Send")
        acceptEnabled: forwardDialog.targetJid !== ""
        onAccepted: {
            if (forwardDialog.messagePairs.length > 0) {
                for (let i = 0; i < forwardDialog.messagePairs.length; ++i) {
                    const pair = forwardDialog.messagePairs[i]
                    backend.forwardMessageFrom(String(pair.chat_jid), String(pair.id), forwardDialog.targetJid)
                }
            } else if (forwardDialog.messageIds.length > 0) {
                for (let i = 0; i < forwardDialog.messageIds.length; ++i)
                    backend.forwardMessage(forwardDialog.messageIds[i], forwardDialog.targetJid)
            } else {
                backend.forwardMessage(forwardDialog.messageId, forwardDialog.targetJid)
            }
        }
        onClosed: {
            messageIds = []
            messagePairs = []
        }
        onOpened: {
            forwardFilter.text = ""
            targetJid = ""
            targetTitle = ""
            forwardFilter.forceActiveFocus()
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            DialogTextField {
                id: forwardFilter
                objectName: "forwardChatFilter"
                Layout.fillWidth: true
                search: true
                placeholderText: qsTr("Search chats")
            }
            ListView {
                objectName: "forwardChatList"
                Layout.fillWidth: true
                Layout.preferredWidth: 360
                Layout.preferredHeight: 320
                clip: true
                model: backend.chatListModel
                delegate: ItemDelegate {
                    required property var modelData
                    width: ListView.view ? ListView.view.width : 0
                    // Filtering happens here rather than in a proxy model so the
                    // shared chat model keeps its identity and ordering.
                    readonly property bool matches: forwardFilter.text.trim() === ""
                        || String(modelData.title || "").toLowerCase()
                            .indexOf(forwardFilter.text.trim().toLowerCase()) >= 0
                    visible: matches
                    height: visible ? 48 : 0
                    text: modelData.title || modelData.jid
                    Accessible.name: text
                    highlighted: forwardDialog.targetJid === String(modelData.jid)
                    onClicked: {
                        forwardDialog.targetJid = String(modelData.jid)
                        forwardDialog.targetTitle = text
                    }
                }
            }
            Label {
                objectName: "forwardTargetLabel"
                Layout.fillWidth: true
                elide: Text.ElideRight
                color: Theme.textMuted
                text: forwardDialog.targetJid === ""
                    ? qsTr("Pick a chat, then press Send.")
                    : qsTr("Send to %1").arg(forwardDialog.targetTitle)
                Accessible.name: text
            }
        }
    }
    WhatsAppDialog {
        id: editDialog
        objectName: "editMessageDialog"
        property string messageId: ""
        title: qsTr("Edit message")
        acceptText: qsTr("Save")
        acceptEnabled: editField.text.trim() !== ""
        onOpened: editField.forceActiveFocus()
        onAccepted: backend.editMessage(messageId, editField.text)
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 96
            radius: 8
            color: Theme.surfaceMuted
            TextArea {
                id: editField
                anchors.fill: parent
                anchors.margins: 10
                background: null
                color: Theme.text
                font.pixelSize: 14
                wrapMode: TextEdit.Wrap
                selectByMouse: true
                Accessible.name: qsTr("Edited message")
            }
        }
    }
    WhatsAppDialog {
        id: deleteDialog
        objectName: "deleteMessageDialog"
        property string messageId: ""
        property string senderJid: ""
        title: qsTr("Delete this message for everyone?")
        acceptText: qsTr("Delete")
        destructive: true
        onAccepted: backend.deleteMessage(messageId, senderJid)
    }

    // Dropping a file onto the conversation attaches it, as WhatsApp Web does.
    // A drop never sends on its own: sending is irreversible and a stray drag
    // over the window should not put a file in front of a contact, so the drop
    // opens a confirmation with the file list and a caption.
    DropArea {
        id: fileDropArea
        objectName: "fileDropArea"
        anchors.fill: parent
        enabled: Boolean(backend.selectedChat.jid)

        onEntered: drag => {
            if (!drag.hasUrls) {
                drag.accepted = false
                return
            }
            drag.accepted = true
        }

        onDropped: drop => {
            const accepted = []
            for (let i = 0; i < drop.urls.length; ++i) {
                const url = String(drop.urls[i])
                // Only real files: a dragged link or a directory has nothing to
                // upload, and the daemon would reject it later and less clearly.
                if (url.startsWith("file://"))
                    accepted.push(url)
            }
            if (accepted.length === 0) {
                drop.accepted = false
                return
            }
            drop.acceptProposedAction()
            dropSendDialog.files = accepted
            dropSendDialog.open()
        }
    }

    Rectangle {
        objectName: "fileDropOverlay"
        anchors.fill: parent
        z: 100
        visible: fileDropArea.containsDrag
        color: Theme.surface
        opacity: 0.92

        ColumnLayout {
            anchors.centerIn: parent
            spacing: 10
            TintedIcon {
                Layout.alignment: Qt.AlignHCenter
                width: 48
                height: 48
                source: Qt.resolvedUrl("icons/attach.svg")
                tint: Theme.primary
            }
            Label {
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Drop to attach")
                color: Theme.text
                font.pixelSize: 19
                font.weight: Font.Medium
            }
            Label {
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Sending to %1").arg(window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid))
                color: Theme.textMuted
                font.pixelSize: 13
            }
        }
    }

    // The send preview is built rather than borrowed from Kirigami: a stock
    // dialog puts a raw path and mnemonic-underlined buttons in front of the
    // user, which reads nothing like the rest of the app.
    Popup {
        id: dropSendDialog
        objectName: "dropSendDialog"
        property var files: []
        readonly property string firstFile: files.length > 0 ? String(files[0]) : ""
        readonly property string firstPath: Theme.localPath(firstFile)
        readonly property bool firstIsImage: /\.(png|jpe?g|gif|webp|bmp)$/i.test(firstPath)

        function fileName(url) {
            const path = Theme.localPath(url)
            const at = path.lastIndexOf("/")
            return at >= 0 ? path.substring(at + 1) : path
        }

        parent: Overlay.overlay
        anchors.centerIn: Overlay.overlay
        modal: true
        focus: true
        padding: 0
        width: Math.min(window.width - 80, 560)
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
        onOpened: dropCaption.text = ""
        onClosed: files = []

        Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }

        background: Rectangle {
            radius: 12
            color: Theme.surface
            border.color: Theme.border
            border.width: 1
        }

        contentItem: ColumnLayout {
            spacing: 0

            RowLayout {
                Layout.fillWidth: true
                Layout.margins: 12
                spacing: 10
                ThemedToolButton {
                    objectName: "dropCancelButton"
                    Layout.preferredWidth: 36
                    Layout.preferredHeight: 36
                    iconSource: Qt.resolvedUrl("icons/close.svg")
                    iconSize: 18
                    Accessible.name: qsTr("Cancel")
                    onClicked: dropSendDialog.close()
                    background: Rectangle { radius: 18; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }
                Label {
                    Layout.fillWidth: true
                    text: dropSendDialog.files.length === 1
                        ? qsTr("Send to %1").arg(window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid))
                        : qsTr("Send %1 files to %2").arg(dropSendDialog.files.length)
                            .arg(window.friendlyTitle(backend.selectedChat.title, backend.selectedChat.jid))
                    color: Theme.text
                    font.pixelSize: 16
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }
            }

            // One file gets a preview the way WhatsApp Web shows it; several are
            // listed by name, because a wall of thumbnails would not fit.
            Rectangle {
                Layout.fillWidth: true
                Layout.leftMargin: 16
                Layout.rightMargin: 16
                Layout.preferredHeight: dropSendDialog.files.length === 1 ? 240 : Math.min(240, 28 + dropSendDialog.files.length * 26)
                radius: 10
                color: Theme.surfaceMuted

                Image {
                    anchors.fill: parent
                    anchors.margins: 8
                    visible: dropSendDialog.files.length === 1 && dropSendDialog.firstIsImage
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    source: visible ? dropSendDialog.firstFile : ""
                }

                ColumnLayout {
                    anchors.centerIn: parent
                    spacing: 8
                    visible: dropSendDialog.files.length === 1 && !dropSendDialog.firstIsImage
                    TintedIcon {
                        Layout.alignment: Qt.AlignHCenter
                        width: 44
                        height: 44
                        source: Qt.resolvedUrl("icons/document.svg")
                        tint: Theme.icon
                    }
                    Label {
                        Layout.alignment: Qt.AlignHCenter
                        Layout.maximumWidth: dropSendDialog.width - 80
                        text: dropSendDialog.fileName(dropSendDialog.firstFile)
                        color: Theme.text
                        font.pixelSize: 15
                        elide: Text.ElideMiddle
                    }
                }

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 4
                    visible: dropSendDialog.files.length > 1
                    Repeater {
                        model: dropSendDialog.files
                        RowLayout {
                            required property var modelData
                            spacing: 8
                            TintedIcon {
                                Layout.preferredWidth: 16
                                Layout.preferredHeight: 16
                                source: Qt.resolvedUrl("icons/document.svg")
                                tint: Theme.icon
                            }
                            Label {
                                Layout.fillWidth: true
                                text: dropSendDialog.fileName(modelData)
                                color: Theme.text
                                font.pixelSize: 13
                                elide: Text.ElideMiddle
                            }
                        }
                    }
                    Item { Layout.fillHeight: true }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                Layout.margins: 16
                spacing: 12

                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 44
                    // A caption belongs to one file. WhatsApp asks per item when
                    // there are several, and this dialog does not, so it offers
                    // none rather than one that would apply to the wrong thing.
                    visible: dropSendDialog.files.length === 1
                    radius: 22
                    color: Theme.surfaceMuted
                    TextField {
                        id: dropCaption
                        objectName: "dropCaptionField"
                        anchors.fill: parent
                        anchors.leftMargin: 16
                        anchors.rightMargin: 16
                        background: null
                        color: Theme.text
                        placeholderText: qsTr("Add a caption")
                        Accessible.name: placeholderText
                        onAccepted: dropSendDialog.sendNow()
                    }
                }
                Item { Layout.fillWidth: dropSendDialog.files.length !== 1 }

                ThemedToolButton {
                    objectName: "dropSendAction"
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    enabled: dropSendDialog.files.length > 0
                    iconSource: Qt.resolvedUrl("icons/send.svg")
                    iconSize: 20
                    iconTint: Theme.primaryText
                    Accessible.name: qsTr("Send")
                    onClicked: dropSendDialog.sendNow()
                    background: Rectangle { radius: 24; color: parent.enabled ? Theme.primary : Theme.textMuted }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }
            }
        }

        function sendNow() {
            const caption = dropSendDialog.files.length === 1 ? dropCaption.text : ""
            for (let i = 0; i < dropSendDialog.files.length; ++i)
                window.sendAttachment(dropSendDialog.files[i], i === 0 ? caption : "")
            dropSendDialog.close()
        }
    }
}
