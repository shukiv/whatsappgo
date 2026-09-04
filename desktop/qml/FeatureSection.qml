import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

RowLayout {
    id: root

    required property string section
    spacing: 0

    readonly property string sectionTitle: {
        switch (section) {
        case "status": return qsTr("Status")
        case "calls": return qsTr("Calls")
        case "channels": return qsTr("Channels")
        case "communities": return qsTr("Communities")
        case "profile": return qsTr("Profile")
        default: return ""
        }
    }
    readonly property var sectionModel: {
        switch (section) {
        case "status": return backend.statusUpdates
        case "calls": return backend.callLogs
        case "channels": return backend.channels
        case "communities": return backend.communities
        default: return []
        }
    }
    readonly property string emptyTitle: {
        switch (section) {
        case "status": return qsTr("Share status updates")
        case "calls": return qsTr("Your call history")
        case "channels": return qsTr("Stay updated with channels")
        case "communities": return qsTr("Keep communities organized")
        case "profile": return qsTr("WhatsAppGo for %1").arg(Theme.platformName)
        default: return ""
        }
    }
    readonly property string emptyDescription: {
        switch (section) {
        case "status": return qsTr("Recent status photos and messages received by the daemon appear here.")
        case "calls": return qsTr("Synced voice and video call records appear here. Placing calls is not supported by the protocol library yet.")
        case "channels": return qsTr("Channels you follow are loaded directly from WhatsApp.")
        case "communities": return qsTr("Communities associated with your joined groups appear here.")
        case "profile": return qsTr("Your linked-device identity and application preferences are available on this computer.")
        default: return ""
        }
    }

    // A channel or community description carries its own line breaks, and rich
    // text honours them however the label is elided, so the row collapses them
    // before showing one line of it.
    function oneLine(text) {
        return String(text || "").replace(/\s+/g, " ").trim()
    }

    signal createChannelRequested()
    signal followChannelRequested()
    signal createCommunityRequested()

    function jidLabel(jid) {
        const value = String(jid || "")
        const local = (value.indexOf("@") >= 0 ? value.slice(0, value.indexOf("@")) : value).replace(/:[0-9]+$/, "")
        return value.endsWith("@s.whatsapp.net") ? "+" + local : qsTr("Contact · %1").arg(local.slice(-4))
    }

    Rectangle {
        Layout.preferredWidth: Math.max(360, Math.min(520, root.width * 0.30))
        Layout.fillHeight: true
        color: Theme.surface
        border.color: Theme.border

        ColumnLayout {
            anchors.fill: parent
            spacing: 0

            RowLayout {
                Layout.fillWidth: true
                Layout.preferredHeight: 64
                Layout.leftMargin: 18
                Layout.rightMargin: 12
                Label {
                    Layout.fillWidth: true
                    text: root.sectionTitle
                    color: Theme.text
                    font.pixelSize: 22
                    font.weight: Font.Bold
                }
                ThemedToolButton {
                    id: sectionAddButton
                    objectName: "featureSectionAddButton"
                    visible: root.section === "channels" || root.section === "communities"
                    Layout.preferredWidth: 40
                    Layout.preferredHeight: 40
                    iconSource: Qt.resolvedUrl("icons/plus.svg")
                    iconSize: 18
                    Accessible.name: root.section === "channels"
                        ? qsTr("New channel or follow one") : qsTr("New community")
                    onClicked: {
                        if (root.section === "communities") {
                            root.createCommunityRequested()
                            return
                        }
                        sectionAddMenu.opened ? sectionAddMenu.close() : sectionAddMenu.open()
                    }
                    background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                    ToolTip.visible: hovered
                    ToolTip.text: Accessible.name
                }
            }

            Rectangle {
                visible: root.section !== "profile"
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
                        anchors.leftMargin: 14
                        anchors.verticalCenter: parent.verticalCenter
                        width: 18
                        height: 18
                        source: "search.svg"
                        tint: Theme.icon
                    }
                    TextField {
                        anchors.fill: parent
                        leftPadding: 42
                        rightPadding: 12
                        placeholderText: qsTr("Search")
                        color: Theme.text
                        background: Item {}
                    }
                }
            }

            ListView {
                id: featureList
                visible: root.section !== "profile"
                Layout.fillWidth: true
                Layout.fillHeight: true
                model: root.sectionModel
                clip: true
                reuseItems: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: OverlayScrollBar {}
                delegate: ItemDelegate {
                    required property var modelData
                    width: ListView.view.width
                    height: 84
                    leftPadding: 16
                    rightPadding: 16
                    background: Rectangle { color: parent.hovered ? Theme.hoverRow : Theme.surface }
                    contentItem: RowLayout {
                        spacing: 12
                        Avatar {
                            Layout.preferredWidth: 56
                            Layout.preferredHeight: 56
                            diameter: 56
                            title: {
                                if (root.section === "status") return modelData.sender_name || root.jidLabel(modelData.sender_jid)
								if (root.section === "calls") return modelData.peer_name || root.jidLabel(modelData.peer_jid)
                                return modelData.name || "?"
                            }
						source: root.section === "calls" && modelData.peer_avatar_path ? "file://" + modelData.peer_avatar_path : ""
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            Layout.minimumWidth: 0
                            spacing: 3
                            Label {
                                Layout.fillWidth: true
                                text: {
                                    if (root.section === "status") return modelData.sender_name || root.jidLabel(modelData.sender_jid)
									if (root.section === "calls") return modelData.peer_name || root.jidLabel(modelData.peer_jid)
                                    return modelData.name || qsTr("Unnamed")
                                }
                                color: root.section === "calls" && modelData.result === "missed" ? Theme.danger : Theme.text
                                font.pixelSize: 16
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                            }
                            Label {
                                Layout.fillWidth: true
                                text: Theme.emojiRichText(
                                    root.section === "status"
                                        ? modelData.body || (modelData.kind === "image" ? qsTr("Photo") : modelData.kind)
                                        : root.section === "calls"
                                            ? (modelData.video ? qsTr("Video") : qsTr("Voice")) + " · " + modelData.result
                                            : root.section === "channels"
                                                ? root.oneLine(modelData.description) || qsTr("%1 followers").arg(modelData.subscriber_count || 0)
                                                : root.oneLine(modelData.description) || qsTr("%1 participants").arg(modelData.participant_count || 0))
                                color: Theme.textMuted
                                font.pixelSize: 14
                                elide: Text.ElideRight
                                // A channel description carries its own line
                                // breaks; without a cap the row grows past its
                                // height and runs under the actions beside it.
                                maximumLineCount: 1
                                wrapMode: Text.NoWrap
                                textFormat: Text.RichText
                            }
                        }
                        Label {
                            visible: root.section === "status" || root.section === "calls"
                            text: Qt.formatTime(new Date(modelData.timestamp || 0), "HH:mm")
                            color: Theme.textMuted
                            font.pixelSize: 11
                        }
                        // A followed channel can be muted or left from its row,
                        // which is what the PWA offers on the channel itself.
                        ThemedToolButton {
                            objectName: "channelMuteButton"
                            visible: root.section === "channels"
                            Layout.preferredWidth: 36
                            Layout.preferredHeight: 36
                            iconSource: Qt.resolvedUrl("icons/mute.svg")
                            iconSize: 17
                            iconTint: modelData.muted ? Theme.primary : Theme.icon
                            Accessible.name: modelData.muted ? qsTr("Unmute channel") : qsTr("Mute channel")
                            onClicked: backend.setChannelMuted(modelData.jid, !modelData.muted)
                            background: Rectangle { radius: 18; color: parent.hovered ? Theme.hoverRow : "transparent" }
                            ToolTip.visible: hovered
                            ToolTip.text: Accessible.name
                        }
                        AbstractButton {
                            id: leaveChannel
                            objectName: "channelLeaveButton"
                            visible: root.section === "channels"
                            implicitWidth: leaveLabel.implicitWidth + 28
                            implicitHeight: 32
                            Accessible.name: qsTr("Leave %1").arg(modelData.name || "")
                            onClicked: backend.setChannelFollowed(modelData.jid, false)
                            background: Rectangle {
                                radius: 16
                                color: leaveChannel.hovered ? Theme.hoverRow : "transparent"
                                border.width: 1
                                border.color: Theme.border
                            }
                            contentItem: Label {
                                id: leaveLabel
                                text: qsTr("Leave")
                                color: Theme.danger
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                    }
                    onClicked: {
                        if (root.section === "status" && modelData.media_path)
                            backend.openFile(modelData.media_path)
                    }
                }

                Label {
                    anchors.centerIn: parent
                    visible: featureList.count === 0
                    width: parent.width - 48
                    text: root.section === "calls" ? qsTr("No call history synced yet") : qsTr("Nothing here yet")
                    color: Theme.textMuted
                    horizontalAlignment: Text.AlignHCenter
                }
            }

            ScrollView {
                visible: root.section === "profile"
                Layout.fillWidth: true
                Layout.fillHeight: true
                contentWidth: availableWidth
                ScrollBar.vertical: OverlayScrollBar {}
                ColumnLayout {
                    width: parent.width
                    spacing: 4
                    Item { Layout.preferredHeight: 18 }
                    Avatar {
                        Layout.alignment: Qt.AlignHCenter
                        Layout.preferredWidth: 116
                        Layout.preferredHeight: 116
                        diameter: 116
                        title: backend.status.user_name || backend.profile
                    }
                    Label {
                        Layout.alignment: Qt.AlignHCenter
                        text: backend.status.user_name || backend.profile
                        color: Theme.text
                        font.pixelSize: 21
                        font.weight: Font.Medium
                    }
                    Label {
                        Layout.alignment: Qt.AlignHCenter
                        text: root.jidLabel(backend.status.user_jid)
                        color: Theme.textMuted
                        font.pixelSize: 13
                    }
                    Item { Layout.preferredHeight: 18 }
                    Repeater {
                        model: [
                            {title: qsTr("Account"), subtitle: qsTr("Security and linked-device identity")},
                            {title: qsTr("Privacy"), subtitle: qsTr("Contacts and blocked messages")},
                            {title: qsTr("Chats"), subtitle: qsTr("Theme, wallpaper and history")},
                            {title: qsTr("Notifications"), subtitle: qsTr("Messages, groups and calls")}
                        ]
                        ItemDelegate {
                            required property var modelData
                            Layout.fillWidth: true
                            Layout.preferredHeight: 72
                            leftPadding: 22
                            rightPadding: 22
                            background: Rectangle { color: parent.hovered ? Theme.hoverRow : Theme.surface }
                            contentItem: Column {
                                spacing: 3
                                Label { text: modelData.title; color: Theme.text; font.pixelSize: 15 }
                                Label { text: modelData.subtitle; color: Theme.textMuted; font.pixelSize: 12 }
                            }
                        }
                    }
                }
            }
        }
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.fillHeight: true
        color: Theme.emptyBackground
        Column {
            anchors.centerIn: parent
            width: Math.min(520, parent.width - 80)
            spacing: 14
            Rectangle {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 86
                height: 86
                radius: 43
                color: Theme.surfaceMuted
                TintedIcon {
                    anchors.centerIn: parent
                    width: 42
                    height: 42
                    source: root.section + ".svg"
                    tint: Theme.icon
                }
            }
            Label {
                width: parent.width
                text: root.emptyTitle
                color: Theme.text
                font.pixelSize: 28
                font.weight: Font.Light
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                width: parent.width
                text: root.emptyDescription
                color: Theme.textMuted
                font.pixelSize: 14
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                lineHeight: 1.35
            }
        }
    }


    WhatsAppMenuPopup {
        id: sectionAddMenu
        objectName: "featureSectionAddMenu"
        parent: Overlay.overlay
        width: 240
        x: Math.max(8, Math.min(Overlay.overlay.width - width - 8,
            sectionAddButton.mapToItem(Overlay.overlay, 0, 0).x - width + 40))
        y: sectionAddButton.mapToItem(Overlay.overlay, 0, sectionAddButton.height).y + 4

        WhatsAppMenuItem {
            objectName: "newChannelItem"
            text: qsTr("New channel")
            iconSource: Qt.resolvedUrl("icons/channels.svg")
            onClicked: {
                sectionAddMenu.close()
                root.createChannelRequested()
            }
        }
        WhatsAppMenuItem {
            objectName: "followChannelItem"
            text: qsTr("Follow with a link")
            iconSource: Qt.resolvedUrl("icons/link.svg")
            onClicked: {
                sectionAddMenu.close()
                root.followChannelRequested()
            }
        }
    }
}
