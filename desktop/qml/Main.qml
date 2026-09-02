import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import QtMultimedia
import QtCore
import org.kde.kirigami as Kirigami
import org.whatsappgo

Kirigami.ApplicationWindow {
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
    property string pendingMessageJumpId: ""
    property int pendingMessageJumpAttempts: 0
    property string highlightedMessageId: ""
    property string chatFilter: "all"
    property bool newChatOpen: false
    property bool showArchived: false
    property bool infoDrawerOpen: false
    property string infoDrawerChatJid: ""
    property bool statusViewerRequested: false
    property string activeSection: {
        const allowed = ["chats", "status", "calls", "channels", "communities", "profile"]
        const args = Qt.application.arguments
        for (let i = 0; i < args.length - 1; ++i) {
            if (args[i] === "--section" && allowed.indexOf(args[i + 1]) >= 0)
                return args[i + 1]
        }
        return "chats"
    }
    readonly property int totalUnreadCount: {
        let total = 0
        for (let i = 0; i < backend.chats.length; ++i)
            total += Number(backend.chats[i].unread_count || 0)
        return total
    }
    readonly property var visibleChats: {
        const result = []
        const chats = backend.chats
        for (let i = 0; i < chats.length; ++i) {
            if (chatFilterBar.accepts(chats[i]))
                result.push(chats[i])
        }
        return result
    }

    Settings {
        id: appearanceSettings
        category: "appearance"
        property string themeMode: "system"
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

    function openChatSearch() {
		showSection("chats")
		searchField.forceActiveFocus()
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
        attachmentMenu.opened ? attachmentMenu.close() : attachmentMenu.open()
    }

    function prepareClipboardPaste() {
        const imageUrl = backend.prepareClipboardImage()
        if (!imageUrl)
            return
        if (mediaPreview.previewActive)
            backend.discardClipboardImage(mediaPreview.imageUrl)
        mediaPreview.openImage(imageUrl)
    }

    function localMediaUrl(path) {
        const value = String(path || "")
        if (!value)
            return ""
        return value.indexOf("file:") === 0 || value.indexOf("data:") === 0 || value.indexOf("qrc:") === 0
            ? value : "file://" + value
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
            onFinished: path => backend.sendVoice(path)
            onFailed: message => {
                window.transientError = message
                errorTimer.restart()
            }
        }
    }

    Connections {
        target: backend
        function onErrorOccurred(message) {
            window.transientError = message
            errorTimer.restart()
        }
        function onNoticeOccurred(message) {
            window.transientNotice = message
            noticeTimer.restart()
        }
        function onDaemonConnectedChanged() {
            if (!backend.daemonConnected)
                return
            backend.refreshStatuses()
            if (window.activeSection !== "chats")
                window.showSection(window.activeSection)
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
        id: messageJumpHighlight
        interval: 1600
        onTriggered: window.highlightedMessageId = ""
    }

    PairingPage {
        anchors.fill: parent
        visible: !backend.loggedIn
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0
        visible: backend.loggedIn

        Rectangle {
            id: navigationRail
            objectName: "navigationRail"
            Layout.preferredWidth: 80
            Layout.fillHeight: true
            color: Theme.navigation

            ColumnLayout {
                anchors.fill: parent
                anchors.topMargin: 12
                anchors.bottomMargin: 12
                spacing: 6

                Rectangle {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    radius: 24
                    color: Theme.primary
                    Label {
                        anchors.centerIn: parent
                        text: "W"
                        color: Theme.primaryText
                        font.pixelSize: 17
                        font.weight: Font.Bold
                    }
                    Rectangle {
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        width: 11
                        height: 11
                        radius: 5.5
                        color: backend.status.state === "connected" ? Theme.brand : Theme.textMuted
                        border.color: Theme.navigation
                        border.width: 2
                    }
                }

                Item { Layout.preferredHeight: 4 }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Qt.resolvedUrl("icons/chats.svg")
                    iconTint: window.activeSection === "chats" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Chats")
                    Accessible.checked: window.activeSection === "chats"
                    onClicked: window.showSection("chats")
                    background: Rectangle {
                        objectName: "navigationChatsBackground"
                        radius: 24
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
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "calls.svg"
                    iconTint: window.activeSection === "calls" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Calls")
                    onClicked: window.showSection("calls")
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "calls" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "status.svg"
                    iconTint: window.activeSection === "status" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Status")
                    onClicked: window.showSection("status")
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "status" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "channels.svg"
                    iconTint: window.activeSection === "channels" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Channels")
                    onClicked: window.showSection("channels")
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "channels" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "communities.svg"
                    iconTint: window.activeSection === "communities" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Communities")
                    onClicked: window.showSection("communities")
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "communities" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Qt.resolvedUrl("icons/search.svg")
                    Accessible.name: qsTr("Search chats")
					onClicked: window.openChatSearch()
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Qt.resolvedUrl("icons/new-chat.svg")
                    Accessible.name: qsTr("Start a new chat")
                    onClicked: window.openNewChat()
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                Item { Layout.fillHeight: true }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "profile.svg"
                    iconTint: window.activeSection === "profile" ? Theme.text : Theme.icon
                    Accessible.name: qsTr("Profile")
                    onClicked: window.showSection("profile")
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : window.activeSection === "profile" ? Theme.navigationSelected
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                    }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                ThemedToolButton {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Theme.dark ? Qt.resolvedUrl("icons/sun.svg") : Qt.resolvedUrl("icons/moon.svg")
                    iconSize: 21
                    Accessible.name: Theme.dark ? qsTr("Switch to light mode") : qsTr("Switch to dark mode")
                    onClicked: Theme.preferredMode = Theme.dark ? "light" : "dark"
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.navigationPressed
                            : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
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
            Layout.preferredWidth: Math.max(360, Math.min(520, window.width * 0.30))
            Layout.fillHeight: true
            color: Theme.surface
            border.color: Theme.border
            visible: window.activeSection === "chats"

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 72
                    color: Theme.surface

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 12
                        anchors.rightMargin: 10
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

                        AccountSwitcherButton {
                            id: accountSwitcherButton
                            objectName: "accountSwitcherButton"
                            Layout.preferredWidth: 44
                            Layout.preferredHeight: 44
                            profiles: backend.profiles
                            currentProfile: backend.profile
                            displayNames: backend.profileDisplayNames
                            unreadCounts: backend.profileUnreadCounts
                            onSwitchRequested: profile => backend.switchProfile(profile)
                            onRenameRequested: profile => {
                                renameAccountDialog.profile = profile
                                renameAccountDialog.initialName = String(backend.profileDisplayNames[profile] || profile)
                                renameAccountDialog.open()
                            }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }

                        ThemedToolButton {
                            id: sidebarMenuButton
                            Layout.preferredWidth: 40
                            Layout.preferredHeight: 40
                            iconSource: Qt.resolvedUrl("icons/menu.svg")
                            iconSize: 20
                            Accessible.name: qsTr("Main menu")
                            Accessible.description: sidebarMenu.opened ? qsTr("Menu open") : qsTr("Menu closed")
                            onClicked: sidebarMenu.opened ? sidebarMenu.close() : sidebarMenu.open()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name

                            WhatsAppMenuPopup {
                                id: sidebarMenu
                                objectName: "sidebarMenu"
                                parent: Overlay.overlay
                                x: Math.max(8, Math.min(window.width - width - 8,
                                                       sidebarMenuButton.mapToItem(window.contentItem, 0, 0).x
                                                       + sidebarMenuButton.width - width))
                                y: sidebarMenuButton.mapToItem(window.contentItem, 0, 0).y + sidebarMenuButton.height + 2

                                WhatsAppMenuItem {
                                    text: qsTr("Search messages")
                                    iconSource: Qt.resolvedUrl("icons/search.svg")
                                    onClicked: { sidebarMenu.close(); messageSearchDialog.open() }
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
                                y: Math.min(window.height - height - 8, sidebarMenu.y + 3 * 46)

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
                        anchors.leftMargin: 12
                        anchors.rightMargin: 12
                        anchors.topMargin: 8
                        anchors.bottomMargin: 8
                        radius: height / 2
                        color: Theme.surfaceMuted

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
                            rightPadding: 12
                            placeholderText: qsTr("Search or start a new chat")
                            color: Theme.text
                            font.pixelSize: 14
                            Accessible.name: qsTr("Search chats")
                            onTextEdited: searchTimer.restart()
                            Keys.onEscapePressed: clear()
                            background: Item {}
                        }
                    }
                }

                // Conversations that were put away live behind this row, the
                // way they do on WhatsApp Web.
                ItemDelegate {
                    id: archivedRow
                    objectName: "archivedRow"
                    Layout.fillWidth: true
                    visible: backend.archivedCount > 0 || window.showArchived
                    height: visible ? 54 : 0
                    leftPadding: 12
                    rightPadding: 12
                    Accessible.name: window.showArchived ? qsTr("Back to chats") : qsTr("Archived chats")
                    onClicked: window.showArchived = !window.showArchived
                    background: Rectangle {
                        color: archivedRow.hovered ? Theme.hoverRow : Theme.surface
                    }
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon {
                            Layout.preferredWidth: 22
                            Layout.preferredHeight: 22
                            Layout.leftMargin: 14
                            source: Qt.resolvedUrl(window.showArchived ? "icons/back.svg" : "icons/archive.svg")
                            tint: Theme.icon
                        }
                        Label {
                            Layout.fillWidth: true
                            text: window.showArchived ? qsTr("Back to chats") : qsTr("Archived")
                            color: Theme.text
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

                ChatFilterBar {
                    id: chatFilterBar
                    visible: !window.showArchived
                    Layout.fillWidth: true
                    Layout.preferredHeight: visible ? implicitHeight : 0
                    selectedFilter: window.chatFilter
                    unreadCount: window.totalUnreadCount
                    onFilterSelected: filter => window.chatFilter = filter
                }

                Timer { id: searchTimer; interval: 250; onTriggered: backend.refreshChats(searchField.text) }

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
                        anchors.rightMargin: 8
                        model: window.showArchived ? backend.archivedChats : window.visibleChats
                        spacing: 2
                        clip: true
                        reuseItems: true
                        boundsBehavior: Flickable.StopAtBounds
                        delegate: ChatListDelegate {
                            current: backend.selectedChat.jid === modelData.jid
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
                            visible: chatList.count === 0
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
                                onClicked: backend.refreshChats(searchField.text)
                            }
                        }
                    }

                    OverlayScrollBar {
                        id: chatListScrollBar
                        objectName: "chatListScrollBar"
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
                        text: qsTr("WhatsAppGo for Linux")
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
                    Layout.fillWidth: true
                    Layout.preferredHeight: 72
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
                            Layout.preferredHeight: 60
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
                                    source: backend.selectedChat.avatar_path ? "file://" + backend.selectedChat.avatar_path : ""
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
                            onClicked: messageSearchDialog.open()
                            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
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
                        onTapped: pinnedMessageMenu.open()
                    }

                    WhatsAppMenuPopup {
                        id: pinnedMessageMenu
                        objectName: "pinnedMessageMenu"
                        parent: Overlay.overlay
                        width: 210
                        x: Math.max(8, window.width - width - 18)
                        y: Math.max(8, pinnedMessageBar.mapToItem(window.contentItem, 0, pinnedMessageBar.height).y + 2)

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
                            chatTitle: backend.selectedChat.title || ""
                            chatAvatarSource: backend.selectedChat.avatar_path ? "file://" + backend.selectedChat.avatar_path : ""
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
                            onImagePreviewRequested: message => window.openChatImage(message)
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
                        }
                    }

                    Connections {
                        target: backend
                        function onSelectedChatChanged() {
                            window.pendingMessageJumpId = ""
                            window.pendingMessageJumpAttempts = 0
                            window.highlightedMessageId = ""
                            messageJumpRetry.stop()
                            messageJumpHighlight.stop()
                            messageList.prepareForChat(backend.selectedChat.jid)
                            if (window.infoDrawerOpen && backend.selectedChat.jid !== window.infoDrawerChatJid) {
                                window.infoDrawerOpen = false
                                backend.clearChatInfo()
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
                                    ToolTip.visible: hovered
                                    ToolTip.text: Accessible.name

                                    AttachmentMenu {
                                        id: attachmentMenu
                                        parent: Overlay.overlay
                                        x: Math.max(8, Math.min(window.width - width - 8,
                                                               attachmentButton.mapToItem(window.contentItem, 0, 0).x - 8))
                                        y: Math.max(8, attachmentButton.mapToItem(window.contentItem, 0, 0).y
                                                       - height - 8)
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
                                        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !(event.modifiers & Qt.ShiftModifier)) {
                                            sendButton.clicked()
                                            event.accepted = true
                                        }
                                    }
                                    background: Item {}
                                }
                                }

                                ThemedToolButton {
                                    id: sendButton
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
                                        if (composer.text.trim().length > 0) {
                                            const body = composer.text
                                            composer.clear()
                                            backend.sendMessage(body, window.replyTargetId)
                                            window.replyTargetId = ""
                                            window.replyPreview = ""
                                            backend.setTyping(false)
                                        } else if (window.recordingVoice) {
                                            voiceRecorderLoader.item.stop()
                                        } else {
                                            voiceRecorderLoader.active = true
                                            Qt.callLater(() => voiceRecorderLoader.item.start())
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
                anchors.fill: parent
                anchors.topMargin: 72
                z: 20
                onSendRequested: (imageUrl, caption) => backend.sendClipboardImage(imageUrl, caption)
                onCanceled: imageUrl => backend.discardClipboardImage(imageUrl)
                onAddRequested: window.prepareClipboardPaste()
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
                onSearchRequested: messageSearchDialog.open()
                onMuteChanged: muted => {
                    backend.setChatMuted(backend.selectedChat.jid, muted)
                }
                onArchiveChanged: archived => {
                    backend.setChatArchived(backend.selectedChat.jid, archived)
                    window.infoDrawerOpen = false
                    backend.clearChatInfo()
                }
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
        }

        FeatureSection {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: window.activeSection !== "chats" && window.activeSection !== "status"
            section: window.activeSection
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

            VideoOutput {
                id: videoSurface
                anchors.fill: parent
                anchors.margins: 24
                anchors.bottomMargin: 84
                fillMode: VideoOutput.PreserveAspectFit
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
            anchors.centerIn: parent
            width: parent.width - 24
            text: window.transientError
            color: Theme.dangerText
            wrapMode: Text.Wrap
            horizontalAlignment: Text.AlignHCenter
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
        id: documentFileDialog
        title: qsTr("Choose a document")
        nameFilters: [qsTr("All files (*)")]
        onAccepted: backend.sendFile(selectedFile)
    }
    FileDialog {
        id: photosVideosFileDialog
        title: qsTr("Choose photos or videos")
        nameFilters: [
            qsTr("Photos and videos (*.jpg *.jpeg *.png *.gif *.webp *.bmp *.heic *.heif *.mp4 *.mov *.mkv *.webm *.avi *.3gp)"),
            qsTr("All files (*)")
        ]
        onAccepted: backend.sendFile(selectedFile)
    }
    FileDialog {
        id: audioFileDialog
        title: qsTr("Choose audio")
        nameFilters: [
            qsTr("Audio files (*.mp3 *.m4a *.ogg *.opus *.wav *.aac *.flac)"),
            qsTr("All files (*)")
        ]
        onAccepted: backend.sendFile(selectedFile)
    }
    Kirigami.PromptDialog {
        id: logoutDialog
        title: qsTr("Unlink this computer?")
        subtitle: qsTr("Local message history remains until you remove the application data.")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel
        onAccepted: backend.logout()
    }
    Kirigami.PromptDialog {
        id: messageSearchDialog
        title: qsTr("Search message history")
        preferredWidth: Math.min(window.width - 48, 640)
        preferredHeight: Math.min(window.height - 48, 560)
        standardButtons: Kirigami.Dialog.Close
        contentItem: ColumnLayout {
            TextField {
                id: messageSearchField
                Layout.fillWidth: true
                placeholderText: qsTr("Search message text")
                Accessible.name: qsTr("Search message text")
                onTextEdited: messageSearchTimer.restart()
            }
            Timer { id: messageSearchTimer; interval: 250; onTriggered: backend.searchMessages(messageSearchField.text) }
            ListView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                model: backend.searchResults
                clip: true
                reuseItems: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: OverlayScrollBar {}
                delegate: ItemDelegate {
                    required property var modelData
                    width: ListView.view.width
                    text: modelData.body
                    Accessible.name: modelData.body
                    onClicked: { backend.openChat(modelData.chat_jid, modelData.chat_jid); messageSearchDialog.close() }
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
    Kirigami.PromptDialog {
        id: renameAccountDialog
        objectName: "renameAccountDialog"
        property string profile: ""
        property string initialName: ""
        title: qsTr("Rename account")
        subtitle: qsTr("This changes only the local label. Your WhatsApp name and account data stay unchanged.")
        preferredWidth: Math.min(window.width - 48, 430)
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel
        onOpened: {
            renameAccountName.text = initialName
            renameAccountName.forceActiveFocus()
            renameAccountName.selectAll()
        }
        onAccepted: backend.renameProfile(profile, renameAccountName.text)
        contentItem: TextField {
            id: renameAccountName
            placeholderText: qsTr("Account display name")
            maximumLength: 64
            Accessible.name: qsTr("Account display name")
            onAccepted: if (text.trim()) renameAccountDialog.accept()
        }
    }
    Kirigami.PromptDialog {
        id: pinDialog
        objectName: "pinMessageDialog"
        property string messageId: ""
        property string senderJid: ""
        property string messagePreview: ""
        title: qsTr("Choose how long to pin this message")
        subtitle: qsTr("You can unpin it at any time.")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel
        onAccepted: {
            let duration = 7 * 24 * 60 * 60
            if (pinOneDay.checked)
                duration = 24 * 60 * 60
            else if (pinThirtyDays.checked)
                duration = 30 * 24 * 60 * 60
            backend.pinMessage(messageId, senderJid, duration)
        }
        contentItem: ColumnLayout {
            spacing: 8
            ButtonGroup { id: pinDurationGroup }
            RadioButton {
                id: pinOneDay
                text: qsTr("24 hours")
                ButtonGroup.group: pinDurationGroup
                Accessible.name: text
            }
            RadioButton {
                id: pinSevenDays
                text: qsTr("7 days")
                checked: true
                ButtonGroup.group: pinDurationGroup
                Accessible.name: text
            }
            RadioButton {
                id: pinThirtyDays
                text: qsTr("30 days")
                ButtonGroup.group: pinDurationGroup
                Accessible.name: text
            }
        }
    }
    Kirigami.PromptDialog {
        id: editDialog
        property string messageId: ""
        title: qsTr("Edit message")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel
        onAccepted: backend.editMessage(messageId, editField.text)
        contentItem: TextArea {
            id: editField
            wrapMode: TextEdit.Wrap
            Accessible.name: qsTr("Edited message")
        }
    }
    Kirigami.PromptDialog {
        id: deleteDialog
        property string messageId: ""
        property string senderJid: ""
        title: qsTr("Delete this message for everyone?")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel
        onAccepted: backend.deleteMessage(messageId, senderJid)
    }
}
