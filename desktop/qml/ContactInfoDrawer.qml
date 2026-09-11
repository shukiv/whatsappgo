import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Rectangle {
    id: root
    objectName: "contactInfoDrawer"

    property bool opened: false
    property bool sharedView: false
    property string activeCategory: "media"
    property var selectedChat: ({})
    property var info: ({})
    property var sharedContent: []
    property bool sharedContentHasMore: false
    property bool sharedContentLoading: false
    property string profile: ""
    property var documentDownloads: []
    property bool selectionActive: false
    property var selectedSharedIds: []
    readonly property string sharedChatJid: String(selectedChat.jid || "")
    readonly property var sharedItems: (sharedContent || []).filter(item => item && item.id
        && !item.revoked && item.kind !== "view_once"
        && (!item.chat_jid || String(item.chat_jid) === sharedChatJid))
    // Resolve identities against the current payload before acting; never keep
    // a parked message object alive across a chat, account or category change.
    readonly property var selectedSharedItems: sharedItems.filter(item => selectedSharedIds.indexOf(String(item.id)) >= 0)
    readonly property var mediaMonths: {
        const groups = []
        const rows = sharedItems.slice().sort((a, b) => Number(b.timestamp || 0) - Number(a.timestamp || 0))
        for (let item of rows) {
            const date = new Date(Number(item.timestamp || 0))
            const valid = Number(item.timestamp) > 0 && isFinite(date.getTime())
            const key = valid ? Qt.formatDate(date, "yyyy-MM") : "undated"
            if (!groups.length || groups[groups.length - 1].key !== key)
                groups.push({ key: key, title: valid ? Qt.formatDate(date, "MMMM yyyy") : qsTr("Undated"), items: [] })
            groups[groups.length - 1].items.push(item)
        }
        return groups
    }
    onSharedChatJidChanged: endSharedSelection()
    onProfileChanged: endSharedSelection()
    onActiveCategoryChanged: endSharedSelection()
    onOpenedChanged: if (!opened) endSharedSelection()
    onSharedViewChanged: if (!sharedView) endSharedSelection()
    property var groupInfo: ({})
    property bool groupLoading: false
    property bool groupBusy: false
    property string groupError: ""
    signal groupActionRequested(string action, var member)
    signal memberAvatarRequested(string jid)
    signal copyRequested(string value)
    readonly property bool isGroup: Boolean(chat && chat.is_group) || String(selectedChat.jid || "").endsWith("@g.us")

    signal closeRequested()
    signal sharedRequested(string category)
    signal loadMoreRequested(string category)
    signal searchRequested()
    signal muteChanged(bool muted)
    signal archiveChanged(bool archived)
    signal favoriteChanged(bool favorite)
    signal blockChanged(bool blocked)
    signal disappearingRequested()
    signal starredRequested()
    signal exportRequested()
    signal clearRequested()
    signal deleteRequested()
    signal openFileRequested(string path)
    signal imagePreviewRequested(var message)
    signal videoPreviewRequested(var message)
    signal avatarPreviewRequested(string path, string title)
    signal downloadRequested(string messageId)
    signal openLinkRequested(string url)
    signal forwardSharedRequested(var items)
    signal saveDocumentRequested(var message)
    signal allMediaRequested(string category)

    width: Math.min(540, parent ? parent.width : 540)
    visible: opened
    color: Theme.surface
    border.width: 1
    border.color: Theme.border
    z: 40
    focus: opened
    Keys.onEscapePressed: {
        if (selectionActive) endSharedSelection()
        else if (sharedView) sharedView = false
        else closeRequested()
    }
    Accessible.role: Accessible.Pane
    Accessible.name: sharedView ? qsTr("Shared content") : qsTr("Contact information")

    readonly property var chat: info && info.chat ? info.chat : selectedChat
    readonly property string title: chat && chat.title ? chat.title : ""
    readonly property string avatarPath: chat && chat.avatar_path ? chat.avatar_path : ""
    readonly property bool isMuted: Number(chat && chat.muted_until || 0) > Date.now()
    readonly property int sharedCount: Number(info && info.shared_count || 0)

    function localUrl(path) {
        const value = String(path || "")
        return Theme.fileUrl(value)
    }

    function messageUrl(message) {
        if (message && message.link_url)
            return String(message.link_url)
        const match = String(message && message.body || "").match(/https?:\/\/[^\s<>"']+/i)
        return match ? match[0] : ""
    }

    function originalAvatarPath(path) {
        return String(path || "").replace(/-round(?:-[0-9]+)?\.png$/, ".jpg")
    }

    function showShared(category) {
        activeCategory = category || "media"
        sharedView = true
        sharedRequested(activeCategory)
    }

    function endSharedSelection() {
        selectionActive = false
        selectedSharedIds = []
    }

    function toggleSharedItem(item) {
        if (!opened || !sharedView || !sharedChatJid || !item
                || (item.chat_jid && String(item.chat_jid) !== sharedChatJid)
                || !sharedItems.some(row => String(row.id) === String(item.id))) return
        selectionActive = true
        const id = String(item.id)
        selectedSharedIds = selectedSharedIds.indexOf(id) >= 0
            ? selectedSharedIds.filter(value => value !== id) : selectedSharedIds.concat([id])
    }

    function forwardSharedSelection() {
        if (!opened || !sharedView || !selectionActive || !sharedChatJid || !selectedSharedItems.length) return
        forwardSharedRequested(selectedSharedItems.map(item => ({ id: String(item.id), chat_jid: sharedChatJid })))
    }

    function downloadBusy(item) {
        return documentDownloads.indexOf(sharedChatJid + "/" + String(item.id)) >= 0
    }

    function activateSharedItem(item) {
        if (!opened || !sharedView || !item
                || (item.chat_jid && String(item.chat_jid) !== sharedChatJid)
                || !sharedItems.some(row => String(row.id) === String(item.id))) return
        if (selectionActive) toggleSharedItem(item)
        else if (activeCategory === "links") openLinkRequested(messageUrl(item))
        else if (activeCategory === "documents") {
            if (!downloadBusy(item)) saveDocumentRequested(Object.assign({}, item, { chat_jid: sharedChatJid }))
        } else if (item.kind === "video") videoPreviewRequested(item)
        else if (item.media_path) imagePreviewRequested(item)
        else downloadRequested(String(item.id))
    }

    function sharedMediaBadge(item) {
        if (item.gif_playback || item.media_mime === "image/gif") return qsTr("GIF")
        if (item.kind !== "video") return ""
        const seconds = Math.max(0, Math.floor(Number(item.media_duration || 0)))
        if (!isFinite(seconds) || seconds === 0) return qsTr("Video")
        return Math.floor(seconds / 60) + ":" + String(seconds % 60).padStart(2, "0")
    }

    function revealSharedItem(item, view) {
        if (!item.activeFocus) return
        const top = item.mapToItem(view.contentItem, 0, 0).y
        if (top < view.contentY) view.contentY = top
        else if (top + item.height > view.contentY + view.height)
            view.contentY = top + item.height - view.height
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 64
            color: Theme.surface

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                spacing: 8

                ThemedToolButton {
                    objectName: "contactInfoBackButton"
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Qt.resolvedUrl(root.sharedView ? "icons/back.svg" : "icons/close.svg")
                    iconSize: 22
                    Accessible.name: root.sharedView ? qsTr("Back to contact information") : qsTr("Close contact information")
                    onClicked: {
                        if (root.selectionActive)
                            root.endSharedSelection()
                        else if (root.sharedView)
                            root.sharedView = false
                        else
                            root.closeRequested()
                    }
                    background: Rectangle { radius: 24; color: parent.hovered ? Theme.hoverRow : "transparent" }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }

                Label {
                    Layout.fillWidth: true
                    text: root.sharedView ? qsTr("Shared content") : (root.chat && root.chat.is_group ? qsTr("Group info") : qsTr("Contact info"))
                    color: Theme.text
                    font.pixelSize: 18
                    font.weight: Font.Medium
                }
                ThemedToolButton {
                    objectName: "contactSharedSelectButton"
                    visible: root.sharedView
                    enabled: root.selectionActive || root.sharedItems.length > 0
                    iconSource: Qt.resolvedUrl(root.selectionActive ? "icons/close.svg" : "icons/check.svg")
                    Accessible.name: root.selectionActive ? qsTr("Cancel selection") : qsTr("Select shared items")
                    ToolTip.visible: hovered || activeFocus
                    ToolTip.text: Accessible.name
                    onClicked: {
                        if (root.selectionActive) root.endSharedSelection()
                        else root.selectionActive = true
                    }
                }
            }

            Rectangle { anchors.left: parent.left; anchors.right: parent.right; anchors.bottom: parent.bottom; height: 1; color: Theme.border }
        }

        Loader {
            Layout.fillWidth: true
            Layout.fillHeight: true
            sourceComponent: root.sharedView ? sharedPage : infoPage
        }
    }

    Component {
        id: infoPage
        ScrollView {
            objectName: "contactInfoScroll"
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
            ScrollBar.vertical: OverlayScrollBar {}

            Column {
                width: parent.width
                spacing: 0

                Column {
                    width: parent.width
                    topPadding: 24
                    bottomPadding: 18
                    spacing: 8

                    Button {
                        id: avatarButton
                        objectName: "contactAvatarButton"
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 120
                        height: 120
                        padding: 0
                        flat: true
                        enabled: Boolean(root.avatarPath)
                        Accessible.name: enabled ? qsTr("Open profile picture for %1").arg(root.title)
                                                 : qsTr("No profile picture for %1").arg(root.title)
                        onClicked: root.avatarPreviewRequested(root.originalAvatarPath(root.avatarPath), root.title)
                        background: Rectangle {
                            radius: width / 2
                            color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                        }
                        contentItem: Avatar {
                            diameter: 120
                            title: root.title
                            fallbackIdentity: !root.avatarPath
                            source: root.localUrl(root.avatarPath)
                        }
                        HoverHandler { cursorShape: avatarButton.enabled ? Qt.PointingHandCursor : Qt.ArrowCursor }
                    }
                    ExpressionButton {
                        objectName: "groupPhotoEditButton"
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: root.avatarPath ? qsTr("Edit group photo") : qsTr("Add group photo")
                        visible: root.isGroup && Boolean(root.groupInfo.can_edit_info)
                        enabled: !root.groupLoading && !root.groupBusy
                        onClicked: {
                            forceActiveFocus(Qt.MouseFocusReason)
                            root.groupActionRequested("edit_photo", ({}))
                        }
                    }
                    CopyableInfoText {
                        objectName: "contactInfoName"
                        width: parent.width - 48
                        anchors.horizontalCenter: parent.horizontalCenter
                        centered: true
                        text: root.isGroup && root.groupInfo.name ? root.groupInfo.name : root.title
                        copyLabel: root.isGroup ? qsTr("Copy group name") : qsTr("Copy name")
                        editEnabled: root.isGroup && Boolean(root.groupInfo.can_edit_info) && !root.groupLoading && !root.groupBusy
                        editLabel: qsTr("Edit group name")
                        onEditRequested: root.groupActionRequested("edit_name", ({}))
                        onCopyRequested: value => root.copyRequested(value)
                        color: Theme.text
                        font.pixelSize: 24
                        font.weight: Font.Medium
                    }
                    CopyableInfoText {
                        objectName: "contactInfoPhone"
                        width: parent.width - 48
                        anchors.horizontalCenter: parent.horizontalCenter
                        centered: true
                        text: root.info && root.info.phone ? "+" + root.info.phone : (root.isGroup ? (root.groupInfo.jid ? qsTr("Group · %1 members").arg(root.groupInfo.participant_count) : qsTr("Group conversation")) : qsTr("WhatsApp contact"))
                        copyEnabled: Boolean(root.info && root.info.phone)
                        copyLabel: qsTr("Copy phone number")
                        onCopyRequested: value => root.copyRequested(value)
                        color: Theme.textMuted
                        font.pixelSize: 15
                    }

                    CopyableInfoText {
                        objectName: "groupInfoDescription"
                        width: parent.width - 48
                        anchors.horizontalCenter: parent.horizontalCenter
                        visible: root.isGroup && (Boolean(root.groupInfo.description) || Boolean(root.groupInfo.can_edit_info))
                        text: root.groupInfo.description || qsTr("Add group description")
                        copyEnabled: Boolean(root.groupInfo.description)
                        copyLabel: qsTr("Copy group description")
                        onCopyRequested: value => root.copyRequested(value)
                        editEnabled: Boolean(root.groupInfo.can_edit_info) && !root.groupLoading && !root.groupBusy
                        editLabel: qsTr("Edit group description")
                        onEditRequested: root.groupActionRequested("edit_description", ({}))
                        color: Theme.text
                        font.pixelSize: 14
                    }
                    Row {
                        anchors.horizontalCenter: parent.horizontalCenter
                        topPadding: 8
                        spacing: 22

                        Repeater {
                            model: [
                                { label: qsTr("Call"), icon: "icons/phone.svg", enabled: false },
                                { label: qsTr("Video"), icon: "icons/video.svg", enabled: false },
                                { label: qsTr("Search"), icon: "icons/search.svg", enabled: true }
                            ]
                            delegate: Column {
                                width: 68
                                spacing: 6
                                ThemedToolButton {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    width: 52
                                    height: 52
                                    enabled: modelData.enabled
                                    iconSource: Qt.resolvedUrl(modelData.icon)
                                    iconSize: 23
                                    Accessible.name: modelData.enabled ? modelData.label : qsTr("%1 is unavailable in the linked-device API").arg(modelData.label)
                                    onClicked: root.searchRequested()
                                    background: Rectangle {
                                        radius: 26
                                        color: parent.hovered ? Theme.hoverRow : Theme.surfaceMuted
                                        border.width: 1
                                        border.color: Theme.border
                                    }
                                    ToolTip.visible: hovered
                                    ToolTip.text: Accessible.name
                                }
                                Label {
                                    width: parent.width
                                    horizontalAlignment: Text.AlignHCenter
                                    text: modelData.label
                                    color: modelData.enabled ? Theme.text : Theme.textMuted
                                    font.pixelSize: 12
                                }
                            }
                        }
                    }
                }

                Rectangle { width: parent.width; height: 8; color: Theme.surfaceMuted }

                ItemDelegate {
                    width: parent.width
                    height: 72
                    Accessible.name: qsTr("Media, links and documents, %1 items").arg(root.sharedCount)
                    onClicked: root.showShared("media")
                    background: Rectangle { color: parent.hovered ? Theme.hoverRow : Theme.surface }
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/gallery.svg"); tint: Theme.icon }
                        Label { Layout.fillWidth: true; text: qsTr("Media, links and documents"); color: Theme.text; font.pixelSize: 15 }
                        Label { text: root.sharedCount; color: Theme.textMuted; font.pixelSize: 14 }
                        TintedIcon { Layout.preferredWidth: 20; Layout.preferredHeight: 20; Layout.rightMargin: 14; source: Qt.resolvedUrl("icons/chevron-right.svg"); tint: Theme.icon }
                    }
                }

                Flickable {
                    width: parent.width
                    height: root.info && root.info.preview && root.info.preview.length ? 112 : 0
                    visible: height > 0
                    contentWidth: previewRow.implicitWidth + 32
                    contentHeight: height
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    Row {
                        id: previewRow
                        x: 16
                        spacing: 8
                        Repeater {
                            model: root.info && root.info.preview ? root.info.preview : []
                            delegate: Rectangle {
                                width: 96
                                height: 96
                                radius: 8
                                color: Theme.surfaceMuted
                                clip: true
                                Image {
                                    id: previewImage
                                    anchors.fill: parent
                                    source: root.localUrl(modelData.media_thumbnail || modelData.media_path || modelData.link_thumbnail)
                                    fillMode: Image.PreserveAspectCrop
                                    asynchronous: true
                                    visible: source !== "" && status !== Image.Error
                                }
                                TintedIcon {
                                    objectName: "contactPreviewFallback"
                                    anchors.centerIn: parent
                                    width: 28
                                    height: 28
                                    visible: previewImage.source === "" || previewImage.status === Image.Error
                                    source: Qt.resolvedUrl("icons/gallery.svg")
                                    tint: Theme.icon
                                }
                                MouseArea {
                                    anchors.fill: parent
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: root.showShared(modelData.kind === "document" ? "documents" : (root.messageUrl(modelData) ? "links" : "media"))
                                }
                            }
                        }
                    }
                }

                Rectangle { width: parent.width; height: 8; color: Theme.surfaceMuted }

                GroupInfoContent {
                    width: parent.width
                    visible: root.isGroup
                    groupInfo: root.groupInfo
                    chat: root.chat
                    loading: root.groupLoading
                    busy: root.groupBusy
                    error: root.groupError
                    onActionRequested: (action, member) => root.groupActionRequested(action, member)
                    onAvatarRequested: jid => root.memberAvatarRequested(jid)
                }

                Column {
                    width: parent.width
                    visible: !root.isGroup
                ItemDelegate {
                    objectName: "drawerFavoriteRow"
                    width: parent.width
                    height: 64
                    Accessible.name: root.chat && root.chat.favorite ? qsTr("Remove from Favorites") : qsTr("Add to Favorites")
                    onClicked: root.favoriteChanged(!(root.chat && root.chat.favorite))
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/heart.svg"); tint: root.chat && root.chat.favorite ? Theme.primary : Theme.icon }
                        Label { Layout.fillWidth: true; text: root.chat && root.chat.favorite ? qsTr("Remove from Favorites") : qsTr("Add to Favorites"); color: Theme.text; font.pixelSize: 15 }
                    }
                }

                ItemDelegate {
                    objectName: "drawerStarredRow"
                    width: parent.width
                    height: 64
                    Accessible.name: qsTr("Starred messages")
                    onClicked: root.starredRequested()
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/star.svg"); tint: Theme.icon }
                        Label { Layout.fillWidth: true; text: qsTr("Starred messages"); color: Theme.text; font.pixelSize: 15 }
                    }
                }

                ItemDelegate {
                    objectName: "drawerDisappearingRow"
                    width: parent.width
                    height: 68
                    Accessible.name: qsTr("Disappearing messages")
                    onClicked: root.disappearingRequested()
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/mute.svg"); tint: Theme.icon }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 1
                            Label { text: qsTr("Disappearing messages"); color: Theme.text; font.pixelSize: 15 }
                            Label {
                                readonly property int seconds: Number(root.chat && root.chat.disappearing_seconds || 0)
                                text: seconds === 0 ? qsTr("Off")
                                    : seconds === 86400 ? qsTr("24 hours")
                                    : seconds === 604800 ? qsTr("7 days")
                                    : seconds === 7776000 ? qsTr("90 days")
                                    : qsTr("On")
                                color: Theme.textMuted
                                font.pixelSize: 12
                            }
                        }
                    }
                }

                ItemDelegate {
                    width: parent.width
                    height: 68
                    Accessible.name: qsTr("Mute notifications")
                    onClicked: root.muteChanged(!root.isMuted)
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/mute.svg"); tint: Theme.icon }
                        Label { Layout.fillWidth: true; text: qsTr("Mute notifications"); color: Theme.text; font.pixelSize: 15 }
                        Switch {
                            id: muteSwitch
                            checked: root.isMuted
                            enabled: false
                            opacity: 1
                            Accessible.name: qsTr("Mute notifications")
                        }
                    }
                }

                Rectangle { width: parent.width; height: 8; color: Theme.surfaceMuted }

                ItemDelegate {
                    width: parent.width
                    height: 72
                    enabled: false
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/lock.svg"); tint: Theme.icon }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 2
                            Label { text: qsTr("Encryption"); color: Theme.text; font.pixelSize: 15 }
                            Label { Layout.fillWidth: true; text: qsTr("Messages are end-to-end encrypted."); color: Theme.textMuted; font.pixelSize: 12; wrapMode: Text.Wrap }
                        }
                    }
                }

                ItemDelegate {
                    width: parent.width
                    height: 64
                    Accessible.name: root.chat && root.chat.archived ? qsTr("Restore chat from archive") : qsTr("Archive chat")
                    onClicked: root.archiveChanged(!(root.chat && root.chat.archived))
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/archive.svg"); tint: Theme.icon }
                        Label { Layout.fillWidth: true; text: root.chat && root.chat.archived ? qsTr("Restore from archive") : qsTr("Archive chat"); color: Theme.text; font.pixelSize: 15 }
                    }
                }

                ItemDelegate {
                    objectName: "drawerExportRow"
                    width: parent.width
                    height: 64
                    Accessible.name: qsTr("Export chat")
                    onClicked: root.exportRequested()
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/document.svg"); tint: Theme.icon }
                        Label { Layout.fillWidth: true; text: qsTr("Export chat"); color: Theme.text; font.pixelSize: 15 }
                    }
                }

                Rectangle { width: parent.width; height: 8; color: Theme.surfaceMuted }

                ItemDelegate {
                    id: blockRow
                    objectName: "drawerBlockRow"
                    width: parent.width
                    // Groups have nobody to block, so the row stays out of a
                    // group's drawer rather than failing when pressed. The
                    // decision is its own property: `visible` reports effective
                    // visibility, which says nothing while an ancestor is hidden.
                    readonly property bool blockable: !(root.chat && root.chat.is_group)
                    visible: blockable
                    height: blockable ? 64 : 0
                    readonly property bool blocked: backend.blockedContacts.some(
                        jid => String(jid).split("@")[0] === String(root.chat && root.chat.jid || "").split("@")[0])
                    Accessible.name: blocked ? qsTr("Unblock") : qsTr("Block")
                    onClicked: root.blockChanged(!blocked)
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/block.svg"); tint: Theme.danger }
                        Label { Layout.fillWidth: true; text: blockRow.blocked ? qsTr("Unblock") : qsTr("Block"); color: Theme.danger; font.pixelSize: 15 }
                    }
                }

                ItemDelegate {
                    objectName: "drawerClearRow"
                    width: parent.width
                    height: 64
                    Accessible.name: qsTr("Clear chat")
                    onClicked: root.clearRequested()
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/block.svg"); tint: Theme.danger }
                        Label { Layout.fillWidth: true; text: qsTr("Clear chat"); color: Theme.danger; font.pixelSize: 15 }
                    }
                }

                ItemDelegate {
                    objectName: "drawerDeleteRow"
                    width: parent.width
                    height: 64
                    Accessible.name: qsTr("Delete chat")
                    onClicked: root.deleteRequested()
                    contentItem: RowLayout {
                        spacing: 14
                        TintedIcon { Layout.preferredWidth: 24; Layout.preferredHeight: 24; Layout.leftMargin: 16; source: Qt.resolvedUrl("icons/delete.svg"); tint: Theme.danger }
                        Label { Layout.fillWidth: true; text: qsTr("Delete chat"); color: Theme.danger; font.pixelSize: 15 }
                    }
                }

                Label {
                    width: parent.width - 32
                    leftPadding: 16
                    rightPadding: 16
                    topPadding: 16
                    bottomPadding: 24
                    text: qsTr("Calling and reporting are not supported by this client. Use the official WhatsApp app for these actions.")
                    color: Theme.textMuted
                    font.pixelSize: 12
                    wrapMode: Text.Wrap
                }
                }
            }
        }
    }

    Component {
        id: sharedPage
        ColumnLayout {
            anchors.fill: parent
            spacing: 0

            RowLayout {
                Layout.fillWidth: true
                Layout.fillHeight: false
                Layout.preferredHeight: 58
                Layout.minimumHeight: 58
                Layout.maximumHeight: 58
                spacing: 0
                Repeater {
                    model: [
                        { key: "media", label: qsTr("Media") },
                        { key: "documents", label: qsTr("Documents") },
                        { key: "links", label: qsTr("Links") }
                    ]
                    delegate: Button {
                        objectName: "contactSharedTab_" + modelData.key
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        flat: true
                        text: modelData.label
                        Accessible.name: modelData.label
                        Accessible.description: root.activeCategory === modelData.key ? qsTr("Selected") : ""
                        onClicked: {
                            root.activeCategory = modelData.key
                            root.sharedRequested(modelData.key)
                        }
                        contentItem: Label {
                            text: parent.text
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            color: root.activeCategory === modelData.key ? Theme.text : Theme.textMuted
                            font.pixelSize: 14
                            font.weight: root.activeCategory === modelData.key ? Font.Medium : Font.Normal
                        }
                        background: Item {
                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.bottom: parent.bottom
                                height: 3
                                color: Theme.primary
                                visible: root.activeCategory === modelData.key
                            }
                        }
                    }
                }
            }

            Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Theme.border }

            RowLayout {
                visible: root.selectionActive
                Layout.fillWidth: true
                Layout.leftMargin: 16
                Layout.rightMargin: 16
                Layout.minimumHeight: 48
                BulkMediaActions {
                    id: sharedBulkActions
                    Layout.fillWidth: true
                    scope: "shared"
                    items: root.selectedSharedItems
                }
                Button {
                    objectName: "contactSharedForwardButton"
                    text: qsTr("Forward")
                    enabled: root.selectedSharedItems.length > 0
                    onClicked: root.forwardSharedSelection()
                }
            }

            Loader {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.minimumHeight: 0
                sourceComponent: root.activeCategory === "media" ? mediaGrid : sharedList
            }
            Button {
                objectName: "contactSharedLoadMoreButton"
                Layout.alignment: Qt.AlignHCenter
                visible: root.sharedContentHasMore
                enabled: !root.sharedContentLoading
                text: root.sharedContentLoading ? qsTr("Loading…") : qsTr("Load more")
                onClicked: root.loadMoreRequested(root.activeCategory)
            }
            Button {
                id: allChatsButton
                objectName: "contactSharedAllChatsButton"
                Layout.fillWidth: true
                Layout.minimumHeight: 48
                flat: true
                text: root.activeCategory === "documents" ? qsTr("View documents from all chats")
                    : root.activeCategory === "links" ? qsTr("View links from all chats") : qsTr("View media from all chats")
                contentItem: Label {
                    text: allChatsButton.text
                    textFormat: Text.PlainText
                    color: Theme.primary
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                    wrapMode: Text.Wrap
                }
                background: Rectangle {
                    color: allChatsButton.hovered ? Theme.hoverRow : Theme.surface
                    border.width: allChatsButton.visualFocus ? 2 : 0
                    border.color: Theme.primary
                }
                onClicked: {
                    root.endSharedSelection()
                    root.allMediaRequested(root.activeCategory)
                }
            }
        }
    }

    Component {
        id: mediaGrid
        Item {
            id: grid
            objectName: "contactMediaGrid"
            readonly property int count: root.sharedItems.length
            readonly property int columnCount: Math.max(2, Math.floor(Math.max(1, width - 32) / 128))
            readonly property real cellWidth: Math.floor(Math.max(1, width - 32) / columnCount)
            ListView {
                id: months
                anchors.fill: parent
                clip: true
                model: root.mediaMonths
                topMargin: 8
                bottomMargin: 8
                ScrollBar.vertical: OverlayScrollBar {}
                onAtYEndChanged: if (atYEnd && root.sharedContentHasMore && !root.sharedContentLoading) root.loadMoreRequested(root.activeCategory)
                delegate: Column {
                    required property var modelData
                    width: months.width
                    Label {
                        objectName: "contactSharedMonth_" + modelData.key
                        height: 44
                        leftPadding: 20
                        verticalAlignment: Text.AlignVCenter
                        text: modelData.title
                        color: Theme.textMuted
                        textFormat: Text.PlainText
                    }
                    Grid {
                        x: 16
                        columns: grid.columnCount
                        Repeater {
                            model: modelData.items
                            delegate: AbstractButton {
                                id: tile
                                required property var modelData
                                objectName: "contactSharedItem_" + modelData.id
                                width: grid.cellWidth
                                height: width
                                padding: 4
                                hoverEnabled: true
                                focusPolicy: Qt.StrongFocus
                                readonly property bool picked: root.selectedSharedIds.indexOf(String(modelData.id)) >= 0
                                Accessible.name: (root.selectionActive ? qsTr("Select %1") : qsTr("Open %1"))
                                    .arg(modelData.media_name || (modelData.kind === "video" ? qsTr("video") : qsTr("photo")))
                                Accessible.description: root.sharedMediaBadge(modelData)
                                Accessible.checkable: root.selectionActive
                                Accessible.checked: picked
                                onClicked: root.activateSharedItem(modelData)
                                Keys.onReturnPressed: clicked()
                                Keys.onEnterPressed: clicked()
                                onActiveFocusChanged: root.revealSharedItem(tile, months)
                                background: Rectangle {
                                    anchors.fill: parent
                                    anchors.margins: 2
                                    radius: 8
                                    color: Theme.surfaceMuted
                                    border.width: tile.activeFocus || tile.picked ? 3 : 0
                                    border.color: Theme.primary
                                }
                                contentItem: Item {
                                    Image {
                                        id: mediaImage
                                        objectName: "contactMediaImage"
                                        anchors.fill: parent
                                        source: root.localUrl(tile.modelData.media_thumbnail
                                            || (tile.modelData.kind === "video" ? "" : tile.modelData.media_path))
                                        fillMode: Image.PreserveAspectCrop
                                        sourceSize: Qt.size(Math.ceil(grid.cellWidth * 2), Math.ceil(grid.cellWidth * 2))
                                        asynchronous: true
                                    }
                                    TintedIcon {
                                        anchors.centerIn: parent
                                        width: 32
                                        height: 32
                                        visible: mediaImage.status === Image.Null || mediaImage.status === Image.Error
                                        source: Qt.resolvedUrl("icons/gallery.svg")
                                        tint: Theme.icon
                                    }
                                    Rectangle {
                                        anchors.left: parent.left
                                        anchors.bottom: parent.bottom
                                        anchors.margins: 6
                                        width: badge.implicitWidth + 12
                                        height: 26
                                        radius: 5
                                        color: "#B3000000"
                                        visible: badge.text !== ""
                                        Label {
                                            id: badge
                                            objectName: "contactSharedBadge_" + tile.modelData.id
                                            anchors.centerIn: parent
                                            text: root.sharedMediaBadge(tile.modelData)
                                            color: "white"
                                            font.pixelSize: 12
                                        }
                                    }
                                    CheckBox {
                                        objectName: "contactSharedCheck_" + tile.modelData.id
                                        anchors.top: parent.top
                                        anchors.left: parent.left
                                        width: 40
                                        height: 40
                                        visible: root.selectionActive || tile.hovered || tile.activeFocus || activeFocus
                                        checked: tile.picked
                                        Accessible.name: tile.picked ? qsTr("Deselect shared item") : qsTr("Select shared item")
                                        onClicked: root.toggleSharedItem(tile.modelData)
                                        onActiveFocusChanged: if (activeFocus) root.revealSharedItem(this, months)
                                        background: Rectangle { radius: 8; color: Theme.surface; opacity: 0.92 }
                                    }
                                }
                            }
                        }
                    }
                }
            }
            Label {
                anchors.centerIn: parent
                visible: grid.count === 0 && !root.sharedContentLoading
                text: qsTr("No media in this chat")
                color: Theme.textMuted
                font.pixelSize: 16
            }
            BusyIndicator {
                anchors.centerIn: parent
                width: 44
                height: 44
                running: visible
                visible: grid.count === 0 && root.sharedContentLoading
                Accessible.name: qsTr("Loading shared media")
            }
        }
    }

    Component {
        id: sharedList
        ListView {
            id: list
            objectName: "contactSharedList"
            clip: true
            model: root.sharedItems
            spacing: 1
            topMargin: 8
            bottomMargin: 8
            ScrollBar.vertical: OverlayScrollBar {}
            onAtYEndChanged: if (atYEnd && root.sharedContentHasMore && !root.sharedContentLoading) root.loadMoreRequested(root.activeCategory)
            delegate: ItemDelegate {
                id: sharedRow
                required property var modelData
                objectName: "contactSharedItem_" + modelData.id
                width: list.width
                height: root.activeCategory === "links" ? 112 : 72
                readonly property bool picked: root.selectedSharedIds.indexOf(String(modelData.id)) >= 0
                focusPolicy: Qt.StrongFocus
                Accessible.name: root.activeCategory === "links" ? root.messageUrl(modelData)
                    : qsTr("Download %1").arg(modelData.media_name || qsTr("Document"))
                Accessible.description: root.selectionActive ? qsTr("Select shared item")
                    : root.activeCategory === "documents" ? (root.downloadBusy(modelData) ? qsTr("Downloading…") : qsTr("Save to Downloads")) : ""
                Accessible.checkable: root.selectionActive
                Accessible.checked: picked
                onClicked: root.activateSharedItem(modelData)
                Keys.onReturnPressed: clicked()
                Keys.onEnterPressed: clicked()
                onActiveFocusChanged: root.revealSharedItem(sharedRow, list)
                background: Rectangle {
                    color: sharedRow.hovered || sharedRow.picked ? Theme.hoverRow : Theme.surface
                    border.width: sharedRow.activeFocus ? 2 : 0
                    border.color: Theme.primary
                }
                contentItem: RowLayout {
                    spacing: 12
                    CheckBox {
                        objectName: "contactSharedCheck_" + sharedRow.modelData.id
                        Layout.preferredWidth: 40
                        Layout.preferredHeight: 40
                        visible: root.selectionActive || sharedRow.hovered || sharedRow.activeFocus || activeFocus
                        checked: sharedRow.picked
                        Accessible.name: sharedRow.picked ? qsTr("Deselect shared item") : qsTr("Select shared item")
                        onClicked: root.toggleSharedItem(sharedRow.modelData)
                        onActiveFocusChanged: if (activeFocus) root.revealSharedItem(this, list)
                    }
                    Rectangle {
                        Layout.preferredWidth: root.activeCategory === "links" ? 88 : 48
                        Layout.preferredHeight: root.activeCategory === "links" ? 88 : 48
                        Layout.leftMargin: 12
                        radius: 8
                        color: Theme.surfaceMuted
                        clip: true
                        Image {
                            id: sharedThumbnail
                            anchors.fill: parent
                            source: root.localUrl(modelData.link_thumbnail)
                            fillMode: Image.PreserveAspectCrop
                            asynchronous: true
                            visible: root.activeCategory === "links" && source !== ""
                        }
                        TintedIcon {
                            anchors.centerIn: parent
                            width: 24
                            height: 24
                            source: Qt.resolvedUrl(root.activeCategory === "links" ? "icons/link.svg" : "icons/document.svg")
                            tint: Theme.icon
                            visible: root.activeCategory !== "links" || sharedThumbnail.source === "" || sharedThumbnail.status === Image.Error
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        spacing: 3
                        Label {
                            Layout.fillWidth: true
                            text: root.activeCategory === "links" ? (modelData.link_title || root.messageUrl(modelData)) : (modelData.media_name || qsTr("Document"))
                            color: Theme.text
                            font.pixelSize: 14
                            font.weight: Font.Medium
                            textFormat: Text.PlainText
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            text: root.activeCategory === "links" ? root.messageUrl(modelData)
                                : root.downloadBusy(modelData) ? qsTr("Downloading…") : (modelData.media_mime || qsTr("Shared document"))
                            color: root.activeCategory === "links" ? Theme.primary : Theme.textMuted
                            font.pixelSize: 12
                            textFormat: Text.PlainText
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            visible: root.activeCategory === "links" && Boolean(modelData.body)
                            text: modelData.body || ""
                            textFormat: Text.PlainText
                            color: Theme.textMuted
                            font.pixelSize: 12
                            elide: Text.ElideRight
                        }
                    }
                    TintedIcon { Layout.preferredWidth: 20; Layout.preferredHeight: 20; Layout.rightMargin: 12; source: Qt.resolvedUrl(root.activeCategory === "documents" ? "icons/download.svg" : "icons/chevron-right.svg"); tint: Theme.icon }
                }
            }
            Label {
                anchors.centerIn: parent
                visible: list.count === 0 && !root.sharedContentLoading
                text: root.activeCategory === "documents" ? qsTr("No documents in this chat") : qsTr("No links in this chat")
                color: Theme.textMuted
                font.pixelSize: 16
            }
            BusyIndicator {
                anchors.centerIn: parent
                width: 44
                height: 44
                running: visible
                visible: list.count === 0 && root.sharedContentLoading
                Accessible.name: qsTr("Loading shared content")
            }
        }
    }
}
