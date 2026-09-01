import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.whatsappgo

ItemDelegate {
    id: root
    required property var modelData
    required property bool current
    property int statusGroupIndex: -1
    property int statusItemCount: 0
    signal chosen(string jid, string title)
    signal statusRequested(string jid)
    signal avatarRequested(string jid)

    function requestAvatarRefresh() {
        const jid = String(root.modelData.jid || "")
        if (root.visible && jid.length > 0)
            root.avatarRequested(jid)
    }

    Component.onCompleted: Qt.callLater(root.requestAvatarRefresh)
    onModelDataChanged: Qt.callLater(root.requestAvatarRefresh)
    onVisibleChanged: {
        if (visible)
            Qt.callLater(root.requestAvatarRefresh)
    }
    ListView.onReused: Qt.callLater(root.requestAvatarRefresh)

    readonly property string displayTitle: {
        const jid = String(root.modelData.jid || "")
        const local = jid.indexOf("@") >= 0 ? jid.slice(0, jid.indexOf("@")) : jid
        const title = String(root.modelData.title || "")
        if (title && title !== local && title !== jid)
            return title
        if (jid.endsWith("@s.whatsapp.net"))
            return "+" + local
        if (jid.endsWith("@g.us"))
            return qsTr("Group")
        return qsTr("Contact · %1").arg(local.slice(-4))
    }
    readonly property bool fallbackIdentity: displayTitle.startsWith("+") || displayTitle.startsWith(qsTr("Contact ·"))

    x: 8
    width: Math.max(0, (ListView.view ? ListView.view.width : 376) - 16)
    height: 72
    padding: 0
    leftPadding: 12
    rightPadding: 12
    hoverEnabled: true
    clip: true
    Accessible.name: displayTitle
        + (modelData.unread_count > 0 ? qsTr(", %1 unread messages").arg(modelData.unread_count) : "")
        + (statusGroupIndex >= 0 ? qsTr(", has a status update") : "")
    onClicked: chosen(modelData.jid, displayTitle)

    background: Rectangle {
        objectName: "chatRowBackground"
        radius: 12
        color: root.down ? Theme.pressedRow : root.current ? Theme.selectedRow : root.hovered ? Theme.hoverRow : Theme.surface
        Rectangle {
            visible: root.activeFocus
            anchors.fill: parent
            anchors.margins: 2
            color: "transparent"
            border.color: Theme.primary
            border.width: 2
            radius: 4
        }
    }

    contentItem: RowLayout {
        spacing: 13

        Button {
            id: statusButton
            objectName: "chatStatusButton"
            Layout.preferredWidth: 56
            Layout.preferredHeight: 56
            Layout.alignment: Qt.AlignVCenter
            padding: 0
            flat: true
            Accessible.name: root.statusGroupIndex >= 0
                ? qsTr("View %1's status").arg(root.displayTitle)
                : qsTr("Open chat with %1").arg(root.displayTitle)
            onClicked: {
                if (root.statusGroupIndex >= 0)
                    root.statusRequested(root.modelData.jid)
                else
                    root.chosen(root.modelData.jid, root.displayTitle)
            }
            background: Rectangle {
                radius: width / 2
                color: statusButton.hovered ? Theme.hoverRow : "transparent"
            }
            contentItem: Item {
                StatusAvatar {
                    objectName: "chatStatusAvatar"
                    anchors.centerIn: parent
                    visible: root.statusGroupIndex >= 0
                    diameter: 56
                    itemCount: Math.max(1, root.statusItemCount)
                    title: root.displayTitle
                    source: root.modelData.avatar_path ? "file://" + root.modelData.avatar_path : ""
                }
                Avatar {
                    objectName: "chatAvatar"
                    anchors.centerIn: parent
                    visible: root.statusGroupIndex < 0
                    diameter: 49
                    title: root.displayTitle
                    fallbackIdentity: root.fallbackIdentity
                    source: root.modelData.avatar_path ? "file://" + root.modelData.avatar_path : ""
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }

        // Name and preview belong together as one block, so they sit on
        // consecutive lines rather than being pushed to opposite edges of
        // the row.
        ColumnLayout {
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            Layout.alignment: Qt.AlignVCenter
            spacing: 2

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                Label {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    text: root.displayTitle
                    color: Theme.text
                    font.pixelSize: 16
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }
                Label {
                    objectName: "chatTimestamp"
                    text: root.modelData.last_message_at ? Qt.formatTime(new Date(root.modelData.last_message_at), "HH:mm") : ""
                    color: root.modelData.unread_count > 0 ? Theme.primary : Theme.textMuted
                    font.pixelSize: 12
                    font.weight: root.modelData.unread_count > 0 ? Font.Medium : Font.Normal
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                Label {
                    objectName: "chatPreview"
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    text: Theme.emojiRichText(String(root.modelData.last_message_preview || qsTr("No messages yet")).replace(/\s+/g, " ").trim())
                    color: Theme.textMuted
                    font.pixelSize: 14
                    elide: Text.ElideRight
                    maximumLineCount: 1
                    textFormat: Text.RichText
                    clip: true
                }
                TintedIcon {
                    objectName: "pinnedMark"
                    visible: Boolean(root.modelData.pinned)
                    Layout.preferredWidth: visible ? 14 : 0
                    Layout.preferredHeight: 14
                    source: Qt.resolvedUrl("icons/pin.svg")
                    tint: Theme.textMuted
                    Accessible.name: qsTr("Pinned")
                }
                Rectangle {
                    objectName: "unreadBadge"
                    visible: root.modelData.unread_count > 0
                    Layout.preferredWidth: root.modelData.unread_count > 99 ? 26 : 20
                    Layout.preferredHeight: 20
                    radius: height / 2
                    color: Theme.primary
                    Label {
                        anchors.centerIn: parent
                        text: root.modelData.unread_count > 99 ? "99+" : root.modelData.unread_count
                        color: Theme.primaryText
                        font.pixelSize: 11
                        font.weight: Font.DemiBold
                    }
                }
            }
        }
    }

    TapHandler {
        acceptedButtons: Qt.RightButton
        onTapped: (eventPoint, button) => {
            const mapped = root.mapToItem(chatMenu.parent, eventPoint.position.x, eventPoint.position.y)
            chatMenu.x = Math.max(8, Math.min(chatMenu.parent.width - chatMenu.width - 8, mapped.x))
            chatMenu.y = Math.max(8, Math.min(chatMenu.parent.height - chatMenu.implicitHeight - 8, mapped.y))
            chatMenu.open()
        }
    }

    WhatsAppMenuPopup {
        id: chatMenu
        objectName: "chatContextMenu"
        parent: Overlay.overlay
        width: 236

        WhatsAppMenuItem {
            text: root.modelData.archived ? qsTr("Unarchive chat") : qsTr("Archive chat")
            iconSource: Qt.resolvedUrl("icons/archive.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatArchived(root.modelData.jid, !root.modelData.archived)
            }
        }
        WhatsAppMenuItem {
            readonly property bool muted: Number(root.modelData.muted_until || 0) > Date.now()
            text: muted ? qsTr("Unmute notifications") : qsTr("Mute notifications")
            iconSource: Qt.resolvedUrl("icons/mute.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatMuted(root.modelData.jid, !muted)
            }
        }
        WhatsAppMenuItem {
            text: root.modelData.pinned ? qsTr("Unpin chat") : qsTr("Pin chat")
            iconSource: Qt.resolvedUrl("icons/pin.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatPinned(root.modelData.jid, !root.modelData.pinned)
            }
        }
        WhatsAppMenuItem {
            text: Number(root.modelData.unread_count || 0) > 0 ? qsTr("Mark as read") : qsTr("Mark as unread")
            iconSource: Qt.resolvedUrl("icons/chats.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatRead(root.modelData.jid, Number(root.modelData.unread_count || 0) > 0)
            }
        }
    }

}
