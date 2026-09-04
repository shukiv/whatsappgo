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
    signal avatarPreviewRequested(string path, string title)
    signal downloadRequested(string messageId)
    signal openLinkRequested(string url)

    width: Math.min(540, parent ? parent.width : 540)
    visible: opened
    color: Theme.surface
    border.width: 1
    border.color: Theme.border
    z: 40
    focus: opened
    Keys.onEscapePressed: closeRequested()
    Accessible.role: Accessible.Pane
    Accessible.name: sharedView ? qsTr("Shared content") : qsTr("Contact information")

    readonly property var chat: info && info.chat ? info.chat : selectedChat
    readonly property string title: chat && chat.title ? chat.title : ""
    readonly property string avatarPath: chat && chat.avatar_path ? chat.avatar_path : ""
    readonly property bool isMuted: Number(chat && chat.muted_until || 0) > Date.now()
    readonly property int sharedCount: Number(info && info.shared_count || 0)

    function localUrl(path) {
        const value = String(path || "")
        return value ? (value.indexOf("file:") === 0 || value.indexOf("data:") === 0 || value.indexOf("qrc:") === 0 ? value : "file://" + value) : ""
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
                        if (root.sharedView)
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
                    Label {
                        width: parent.width - 48
                        anchors.horizontalCenter: parent.horizontalCenter
                        horizontalAlignment: Text.AlignHCenter
                        text: root.title
                        color: Theme.text
                        font.pixelSize: 24
                        font.weight: Font.Medium
                        wrapMode: Text.Wrap
                    }
                    Label {
                        width: parent.width - 48
                        anchors.horizontalCenter: parent.horizontalCenter
                        horizontalAlignment: Text.AlignHCenter
                        text: root.info && root.info.phone ? "+" + root.info.phone : (root.chat && root.chat.is_group ? qsTr("Group conversation") : qsTr("WhatsApp contact"))
                        color: Theme.textMuted
                        font.pixelSize: 15
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
                    text: qsTr("Reporting and calls are not carried by the linked-device protocol, so WhatsAppGo does not show controls that would fail silently.")
                    color: Theme.textMuted
                    font.pixelSize: 12
                    wrapMode: Text.Wrap
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

            Loader {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.minimumHeight: 0
                sourceComponent: root.activeCategory === "media" ? mediaGrid : sharedList
            }
        }
    }

    Component {
        id: mediaGrid
        GridView {
            id: grid
            objectName: "contactMediaGrid"
            clip: true
            model: root.sharedContent
            readonly property int columnCount: Math.max(2, Math.floor(Math.max(1, width - leftMargin - rightMargin) / 128))
            cellWidth: Math.floor(Math.max(1, width - leftMargin - rightMargin) / columnCount)
            cellHeight: cellWidth
            topMargin: 16
            leftMargin: 16
            rightMargin: 16
            bottomMargin: 16
            ScrollBar.vertical: OverlayScrollBar {}
            onAtYEndChanged: if (atYEnd && root.sharedContentHasMore) root.loadMoreRequested(root.activeCategory)
            delegate: Item {
                width: grid.cellWidth
                height: grid.cellHeight
                Rectangle {
                    anchors.fill: parent
                    anchors.margins: 4
                    radius: 8
                    color: Theme.surfaceMuted
                    clip: true
                    Image {
                        id: mediaImage
                        objectName: "contactMediaImage"
                        anchors.fill: parent
                        source: root.localUrl(modelData.media_thumbnail || modelData.media_path)
                        fillMode: Image.PreserveAspectCrop
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
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            if (modelData.media_path)
                                root.imagePreviewRequested(modelData)
                            else
                                root.downloadRequested(modelData.id)
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
            model: root.sharedContent
            spacing: 1
            topMargin: 8
            bottomMargin: 8
            ScrollBar.vertical: OverlayScrollBar {}
            onAtYEndChanged: if (atYEnd && root.sharedContentHasMore) root.loadMoreRequested(root.activeCategory)
            delegate: ItemDelegate {
                width: list.width
                height: root.activeCategory === "links" ? 112 : 72
                Accessible.name: root.activeCategory === "links" ? root.messageUrl(modelData) : (modelData.media_name || qsTr("Document"))
                onClicked: {
                    if (root.activeCategory === "links")
                        root.openLinkRequested(root.messageUrl(modelData))
                    else if (modelData.media_path)
                        root.openFileRequested(modelData.media_path)
                    else
                        root.downloadRequested(modelData.id)
                }
                background: Rectangle { color: parent.hovered ? Theme.hoverRow : Theme.surface }
                contentItem: RowLayout {
                    spacing: 12
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
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            text: root.activeCategory === "links" ? root.messageUrl(modelData) : (modelData.media_mime || qsTr("Shared document"))
                            color: root.activeCategory === "links" ? Theme.primary : Theme.textMuted
                            font.pixelSize: 12
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            visible: root.activeCategory === "links" && Boolean(modelData.body)
                            text: modelData.body || ""
                            color: Theme.textMuted
                            font.pixelSize: 12
                            elide: Text.ElideRight
                        }
                    }
                    TintedIcon { Layout.preferredWidth: 20; Layout.preferredHeight: 20; Layout.rightMargin: 12; source: Qt.resolvedUrl("icons/chevron-right.svg"); tint: Theme.icon }
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
