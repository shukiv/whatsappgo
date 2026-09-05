import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    required property var modelData
    required property bool current
    property int statusGroupIndex: -1
    property int statusItemCount: 0
    // Set while the row stands for a search hit, so the matched run is tinted
    // the way WhatsApp Web tints it.
    property string highlightQuery: ""
    signal chosen(string jid, string title)
    signal statusRequested(string jid)
    signal avatarRequested(string jid)

    // The chevron sticks around while its own menu is open, so the row does not
    // appear to lose the control the pointer just used.
    readonly property bool showRowMenu: !root.selectionActive && (root.hovered || chatMenu.opened)

    // Clamped inside the overlay so a row near an edge still gets a whole menu.
    function openChatMenuAt(item, x, y) {
        const mapped = item.mapToItem(chatMenu.parent, x, y)
        chatMenu.x = Math.max(8, Math.min(chatMenu.parent.width - chatMenu.width - 8, mapped.x))
        chatMenu.y = Math.max(8, Math.min(chatMenu.parent.height - chatMenu.implicitHeight - 8, mapped.y))
        chatMenu.open()
    }

    // Both mute menus close together: the duration list is a branch of the row
    // menu, not a popup that should outlive it.
    function applyMute(durationSeconds) {
        muteDurationMenu.close()
        chatMenu.close()
        backend.setChatMuted(root.modelData.jid, true, durationSeconds)
    }

    // WhatsApp Web labels the last message with a type icon rather than the
    // word for it, and only when there is no text to show instead.
    readonly property string previewKindIcon: {
        if (String(root.modelData.last_message_preview || "").length === 0)
            return ""
        switch (String(root.modelData.last_message_kind || "")) {
        case "image": return "gallery.svg"
        case "video": return "camera.svg"
        case "audio": return "mic.svg"
        case "document": return "document.svg"
        case "sticker": return "sticker.svg"
        case "contact": return "contact.svg"
        case "location": return "pin.svg"
        default: return ""
        }
    }

    // A voice note reads as its length in the list, the way it does in the
    // conversation; everything else keeps the stored preview text.
    readonly property string previewText: {
        const preview = String(root.modelData.last_message_preview || "").replace(/\s+/g, " ").trim()
        if (preview.length === 0)
            return qsTr("No messages yet")
        const duration = Number(root.modelData.last_message_duration || 0)
        if (preview === "Audio" && duration > 0) {
            const minutes = Math.floor(duration / 60)
            const seconds = Math.floor(duration % 60)
            return minutes + ":" + (seconds < 10 ? "0" : "") + seconds
        }
        return preview
    }

    readonly property string rowTimestampText: RowTime.label(root.modelData.last_message_at)

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
    // In selection mode a row is a checkbox, not a way into the conversation.
    property bool selectionActive: false
    property bool selected: false
    signal selectionToggled(string jid)

    onClicked: {
        if (selectionActive)
            selectionToggled(String(modelData.jid || ""))
        else
            chosen(modelData.jid, displayTitle)
    }

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
                    source: Theme.fileUrl(root.modelData.avatar_path)
                }
                Avatar {
                    objectName: "chatAvatar"
                    anchors.centerIn: parent
                    visible: root.statusGroupIndex < 0
                    diameter: 49
                    title: root.displayTitle
                    fallbackIdentity: root.fallbackIdentity
                    source: Theme.fileUrl(root.modelData.avatar_path)
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
                    text: root.highlightQuery.length > 0
                        ? SearchHighlight.markup(root.displayTitle, root.highlightQuery, Theme.primary)
                        : root.displayTitle
                    textFormat: root.highlightQuery.length > 0 ? Text.StyledText : Text.PlainText
                    color: Theme.text
                    font.pixelSize: 16
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }
                Label {
                    objectName: "chatTimestamp"
                    text: root.rowTimestampText
                    color: root.modelData.unread_count > 0 ? Theme.primary : Theme.textMuted
                    font.pixelSize: 12
                    font.weight: root.modelData.unread_count > 0 ? Font.Medium : Font.Normal
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 4

                ReadReceipt {
                    objectName: "chatPreviewReceipt"
                    visible: Boolean(root.modelData.last_message_from_me)
                        && String(root.modelData.last_message_status || "") !== ""
                    Layout.alignment: Qt.AlignVCenter
                    status: root.modelData.last_message_status || ""
                }

                TintedIcon {
                    objectName: "chatPreviewKindIcon"
                    visible: root.previewKindIcon !== ""
                    Layout.preferredWidth: visible ? 15 : 0
                    Layout.preferredHeight: 15
                    Layout.alignment: Qt.AlignVCenter
                    source: root.previewKindIcon !== "" ? Qt.resolvedUrl("icons/" + root.previewKindIcon) : ""
                    tint: Theme.textMuted
                }

                Label {
                    objectName: "chatPreview"
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    Layout.leftMargin: root.previewKindIcon !== "" ? 2 : 0
                    text: Theme.emojiRichText(root.previewText)
                    color: Theme.textMuted
                    font.pixelSize: 14
                    elide: Text.ElideRight
                    maximumLineCount: 1
                    textFormat: Text.RichText
                    clip: true
                }
                // Search results reach onto the archived shelf, so a row from
                // there says where it came from. The chat list itself never
                // needs the mark: everything in it is unarchived.
                Rectangle {
                    objectName: "chatArchivedBadge"
                    visible: root.highlightQuery.length > 0 && Boolean(root.modelData.archived)
                    Layout.preferredWidth: visible ? archivedBadgeLabel.implicitWidth + 12 : 0
                    Layout.preferredHeight: visible ? 18 : 0
                    Layout.leftMargin: visible ? 6 : 0
                    Layout.alignment: Qt.AlignVCenter
                    radius: 4
                    color: Theme.surfaceMuted
                    Label {
                        id: archivedBadgeLabel
                        anchors.centerIn: parent
                        text: qsTr("Archived")
                        color: Theme.textMuted
                        font.pixelSize: 11
                    }
                }

                // WhatsApp Web reveals a chevron on the hovered row that opens the
                // same menu as right-click, and hides the pin/unread marks while
                // it shows.
                AbstractButton {
                    id: rowMenuButton
                    objectName: "chatRowMenuButton"
                    visible: root.showRowMenu
                    Layout.preferredWidth: visible ? 20 : 0
                    Layout.preferredHeight: 20
                    Layout.alignment: Qt.AlignVCenter
                    Accessible.name: qsTr("Chat menu")
                    contentItem: TintedIcon {
                        source: Qt.resolvedUrl("icons/chevron-down.svg")
                        tint: Theme.textMuted
                    }
                    onClicked: root.openChatMenuAt(rowMenuButton, 0, rowMenuButton.height)
                }

                Rectangle {
                    objectName: "chatSelectionMark"
                    visible: root.selectionActive
                    Layout.preferredWidth: visible ? 22 : 0
                    Layout.preferredHeight: 22
                    radius: 11
                    color: root.selected ? Theme.primary : "transparent"
                    border.width: root.selected ? 0 : 2
                    border.color: Theme.textMuted
                    TintedIcon {
                        anchors.centerIn: parent
                        width: 14
                        height: 14
                        visible: root.selected
                        source: Qt.resolvedUrl("icons/check.svg")
                        tint: Theme.primaryText
                    }
                }
                TintedIcon {
                    objectName: "pinnedMark"
                    visible: Boolean(root.modelData.pinned) && !root.selectionActive && !root.showRowMenu
                    Layout.preferredWidth: visible ? 14 : 0
                    Layout.preferredHeight: 14
                    source: Qt.resolvedUrl("icons/pin.svg")
                    tint: Theme.textMuted
                    Accessible.name: qsTr("Pinned")
                }
                Rectangle {
                    objectName: "unreadBadge"
                    visible: root.modelData.unread_count > 0 && !root.showRowMenu
                    Layout.preferredWidth: root.modelData.unread_count > 99 ? 26 : 20
                    Layout.preferredHeight: 20
                    radius: height / 2
                    color: Theme.primary
                    Label {
                        anchors.centerIn: parent
                        text: root.modelData.unread_count > 99 ? "99+" : Number(root.modelData.unread_count || 0)
                        color: Theme.primaryText
                        font.pixelSize: 11
                        font.weight: Font.DemiBold
                    }
                }
            }
        }
    }

    TapHandler {
        enabled: !root.selectionActive
        acceptedButtons: Qt.RightButton
        onTapped: (eventPoint, button) => root.openChatMenuAt(root, eventPoint.position.x, eventPoint.position.y)
    }

    WhatsAppMenuPopup {
        id: chatMenu
        objectName: "chatContextMenu"
        parent: Overlay.overlay
        width: 238

        WhatsAppMenuItem {
            text: root.modelData.archived ? qsTr("Unarchive chat") : qsTr("Archive chat")
            iconSource: Qt.resolvedUrl("icons/archive.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatArchived(root.modelData.jid, !root.modelData.archived)
            }
        }
        WhatsAppMenuItem {
            id: muteItem
            objectName: "chatMuteItem"
            readonly property bool muted: Number(root.modelData.muted_until || 0) > Date.now()
            text: muted ? qsTr("Unmute notifications") : qsTr("Mute notifications")
            iconSource: Qt.resolvedUrl("icons/mute.svg")
            // Muting asks for how long, the way WhatsApp Web does; unmuting has
            // nothing to ask, so it stays a single click.
            onClicked: {
                if (muted) {
                    chatMenu.close()
                    backend.setChatMuted(root.modelData.jid, false)
                    return
                }
                muteDurationMenu.opened ? muteDurationMenu.close() : muteDurationMenu.open()
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

        WhatsAppMenuItem {
            objectName: "chatFavoriteItem"
            text: Boolean(root.modelData.favorite) ? qsTr("Remove from Favorites") : qsTr("Add to Favorites")
            iconSource: Qt.resolvedUrl("icons/heart.svg")
            onClicked: {
                chatMenu.close()
                backend.setChatFavorite(root.modelData.jid, !Boolean(root.modelData.favorite))
            }
        }

        WhatsAppMenuItem {
            objectName: "chatListsItem"
            text: qsTr("Add to list")
            iconSource: Qt.resolvedUrl("icons/poll.svg")
            onClicked: chatListsMenu.opened ? chatListsMenu.close() : chatListsMenu.open()
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 1
            Layout.topMargin: 4
            Layout.bottomMargin: 4
            color: Theme.border
        }

        WhatsAppMenuItem {
            objectName: "chatBlockItem"
            readonly property string domain: String(root.modelData.jid || "").split("@")[1] || ""
            visible: domain !== "g.us" && domain !== "newsletter" && domain !== "broadcast" 
            // Compared on the user part: the blocklist comes back in phone form
            // while a chat may be keyed by its LID alias.
            readonly property string userPart: String(root.modelData.jid || "").split("@")[0]
            readonly property bool blocked: backend.blockedContacts.some(
                jid => String(jid).split("@")[0] === userPart)
            text: blocked ? qsTr("Unblock") : qsTr("Block")
            iconSource: Qt.resolvedUrl("icons/block.svg")
            onClicked: {
                chatMenu.close()
                backend.setContactBlocked(root.modelData.jid, !blocked)
            }
        }

        WhatsAppMenuItem {
            objectName: "chatClearItem"
            text: qsTr("Clear chat")
            iconSource: Qt.resolvedUrl("icons/block.svg")
            onClicked: {
                chatMenu.close()
                clearChatDialog.open()
            }
        }

        WhatsAppMenuItem {
            objectName: "chatDeleteItem"
            text: qsTr("Delete chat")
            destructive: true
            iconSource: Qt.resolvedUrl("icons/delete.svg")
            onClicked: {
                chatMenu.close()
                deleteChatDialog.open()
            }
        }
    }

    WhatsAppDialog {
        id: clearChatDialog
        objectName: "chatClearDialog"
        title: qsTr("Clear this chat?")
        subtitle: qsTr("The messages are removed from this computer and from your other devices. The chat itself stays in the list.")
        acceptText: qsTr("Clear chat")
        destructive: true
        onAccepted: backend.clearChat(root.modelData.jid)
    }

    WhatsAppDialog {
        id: deleteChatDialog
        objectName: "chatDeleteDialog"
        title: qsTr("Delete this chat?")
        subtitle: qsTr("The conversation is removed from this computer and from your other devices. This cannot be undone.")
        acceptText: qsTr("Delete")
        destructive: true
        onAccepted: backend.deleteChat(root.modelData.jid)
    }

    WhatsAppMenuPopup {
        id: chatListsMenu
        objectName: "chatListsMenu"
        parent: Overlay.overlay
        width: 220
        x: Math.max(8, Math.min(Overlay.overlay.width - width - 8, chatMenu.x + chatMenu.width - 12))
        y: Math.max(8, Math.min(Overlay.overlay.height - height - 8, chatMenu.y + 4 * 36))
        onAboutToShow: backend.refreshChatLabels()

        Repeater {
            model: backend.chatLabels
            WhatsAppMenuItem {
                required property var modelData
                text: modelData.name || modelData.id
                iconSource: Qt.resolvedUrl("icons/poll.svg")
                onClicked: {
                    chatListsMenu.close()
                    chatMenu.close()
                    backend.setChatLabeled(root.modelData.jid, String(modelData.id), true)
                }
            }
        }

        WhatsAppMenuItem {
            objectName: "chatListsEmptyItem"
            visible: backend.chatLabels.length === 0
            enabled: false
            text: qsTr("No lists yet")
        }
    }

    WhatsAppMenuPopup {
        id: muteDurationMenu
        objectName: "chatMuteDurationMenu"
        parent: Overlay.overlay
        width: 190
        x: Math.max(8, Math.min(Overlay.overlay.width - width - 8, chatMenu.x + chatMenu.width - 12))
        y: Math.max(8, Math.min(Overlay.overlay.height - height - 8, chatMenu.y + 36))

        WhatsAppMenuItem {
            objectName: "chatMuteEightHoursItem"
            text: qsTr("8 hours")
            onClicked: root.applyMute(8 * 60 * 60)
        }
        WhatsAppMenuItem {
            objectName: "chatMuteOneWeekItem"
            text: qsTr("1 week")
            onClicked: root.applyMute(7 * 24 * 60 * 60)
        }
        WhatsAppMenuItem {
            objectName: "chatMuteAlwaysItem"
            text: qsTr("Always")
            onClicked: root.applyMute(0)
        }
    }

}
