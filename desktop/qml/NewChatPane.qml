import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Rectangle {
    id: root

    property var chats: []
    signal closeRequested()
    signal chatSelected(string jid, string title)
    signal phoneRequested(string phone)
    signal unavailableRequested(string feature)

    readonly property var filteredChats: {
        const needle = searchField.text.trim().toLowerCase()
        if (!needle)
            return root.chats
        const result = []
        for (let i = 0; i < root.chats.length; ++i) {
            const chat = root.chats[i]
            const haystack = (String(chat.title || "") + " " + String(chat.jid || "")).toLowerCase()
            if (haystack.indexOf(needle) >= 0)
                result.push(chat)
        }
        return result
    }

    function displayTitle(chat) {
        const jid = String(chat.jid || "")
        const local = jid.indexOf("@") >= 0 ? jid.slice(0, jid.indexOf("@")) : jid
        const title = String(chat.title || "")
        if (title && title !== local && title !== jid)
            return title
        if (jid.endsWith("@s.whatsapp.net"))
            return "+" + local
        return jid.endsWith("@g.us") ? qsTr("Group") : qsTr("Contact · %1").arg(local.slice(-4))
    }

    color: Theme.surface
    border.color: Theme.border

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
                anchors.rightMargin: 12
                spacing: 8

                ThemedToolButton {
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: "back.svg"
                    iconSize: 22
                    Accessible.name: qsTr("Back to chats")
                    onClicked: root.closeRequested()
                    background: Rectangle {
                        radius: 24
                        color: parent.down ? Theme.pressedRow : parent.hovered || parent.activeFocus ? Theme.hoverRow : "transparent"
                        border.width: parent.activeFocus ? 2 : 0
                        border.color: Theme.primary
                    }
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("New chat")
                    color: Theme.text
                    font.pixelSize: 21
                    font.weight: Font.Medium
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
                border.width: searchField.activeFocus ? 2 : 0
                border.color: Theme.primary

                TintedIcon {
                    anchors.left: parent.left
                    anchors.leftMargin: 16
                    anchors.verticalCenter: parent.verticalCenter
                    width: 20
                    height: 20
                    source: Qt.resolvedUrl("icons/search.svg")
                    tint: Theme.icon
                }

                TextField {
                    id: searchField
                    anchors.fill: parent
                    leftPadding: 48
                    rightPadding: 14
                    placeholderText: qsTr("Search name or number")
                    color: Theme.text
                    font.pixelSize: 14
                    Accessible.name: qsTr("Search contacts")
                    background: Item {}
                    Keys.onEscapePressed: root.closeRequested()
                }
            }
        }

        ListView {
            id: contactsList
            Layout.fillWidth: true
            Layout.fillHeight: true
            model: root.filteredChats
            clip: true
            reuseItems: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: OverlayScrollBar {}

            header: Column {
                width: contactsList.width

                Repeater {
                    model: [
                        { title: qsTr("New group"), icon: "group-add.svg", action: "group" },
                        { title: qsTr("New contact"), icon: "user-add.svg", action: "contact" },
                        { title: qsTr("New community"), icon: "communities.svg", action: "community" }
                    ]

                    ItemDelegate {
                        required property var modelData
                        width: contactsList.width
                        height: 72
                        leftPadding: 16
                        rightPadding: 16
                        hoverEnabled: true
                        background: Rectangle {
                            color: parent.down ? Theme.pressedRow : parent.hovered || parent.activeFocus ? Theme.hoverRow : Theme.surface
                            border.width: parent.activeFocus ? 2 : 0
                            border.color: Theme.primary
                        }
                        contentItem: RowLayout {
                            spacing: 16
                            Rectangle {
                                Layout.preferredWidth: 48
                                Layout.preferredHeight: 48
                                radius: 24
                                color: Theme.primary
                                TintedIcon {
                                    anchors.centerIn: parent
                                    width: 24
                                    height: 24
                                    source: modelData.icon
                                    tint: Theme.primaryText
                                }
                            }
                            Label {
                                Layout.fillWidth: true
                                text: modelData.title
                                color: Theme.text
                                font.pixelSize: 16
                            }
                        }
                        Accessible.name: modelData.title
                        onClicked: {
                            if (modelData.action === "contact") {
                                phoneEditor.visible = true
                                phoneField.forceActiveFocus()
                            } else {
                                root.unavailableRequested(modelData.title)
                            }
                        }
                    }
                }

                Rectangle {
                    id: phoneEditor
                    width: parent.width
                    height: visible ? 74 : 0
                    visible: false
                    color: Theme.surface
                    clip: true
                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 16
                        anchors.rightMargin: 16
                        spacing: 8
                        Rectangle {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 46
                            radius: 23
                            color: Theme.surfaceMuted
                            border.width: phoneField.activeFocus ? 2 : 1
                            border.color: phoneField.activeFocus ? Theme.primary : Theme.border
                            TextField {
                                id: phoneField
                                anchors.fill: parent
                                leftPadding: 16
                                rightPadding: 16
                                placeholderText: qsTr("International phone number")
                                inputMethodHints: Qt.ImhDialableCharactersOnly
                                color: Theme.text
                                font.pixelSize: 14
                                background: Item {}
                                Accessible.name: qsTr("Phone number in international format")
                                onAccepted: if (text.trim()) root.phoneRequested(text.trim())
                            }
                        }
                        Button {
                            Layout.preferredWidth: 72
                            Layout.preferredHeight: 46
                            text: qsTr("Chat")
                            enabled: Boolean(phoneField.text.trim())
                            contentItem: Label {
                                text: parent.text
                                color: parent.enabled ? Theme.primaryText : Theme.textMuted
                                font.pixelSize: 14
                                font.weight: Font.DemiBold
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                            background: Rectangle {
                                radius: 23
                                color: parent.enabled ? Theme.primary : Theme.surfaceMuted
                            }
                            onClicked: root.phoneRequested(phoneField.text.trim())
                        }
                    }
                }

                Label {
                    width: parent.width - 32
                    height: 42
                    leftPadding: 16
                    text: qsTr("Contacts and recent chats")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    verticalAlignment: Text.AlignVCenter
                }
            }

            delegate: ItemDelegate {
                required property var modelData
                width: contactsList.width
                height: 80
                leftPadding: 16
                rightPadding: 16
                hoverEnabled: true
                background: Rectangle {
                    color: parent.down ? Theme.pressedRow : parent.hovered || parent.activeFocus ? Theme.hoverRow : Theme.surface
                    border.width: parent.activeFocus ? 2 : 0
                    border.color: Theme.primary
                }
                contentItem: RowLayout {
                    spacing: 14
                    Avatar {
                        Layout.preferredWidth: 52
                        Layout.preferredHeight: 52
                        diameter: 52
                        title: root.displayTitle(modelData)
                        source: modelData.avatar_path ? "file://" + modelData.avatar_path : ""
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        spacing: 3
                        Label {
                            Layout.fillWidth: true
                            text: root.displayTitle(modelData)
                            color: Theme.text
                            font.pixelSize: 16
                            font.weight: Font.Medium
                            elide: Text.ElideRight
                        }
                        Label {
                            Layout.fillWidth: true
                            text: modelData.is_group ? qsTr("Group") : (modelData.last_message || qsTr("WhatsApp contact"))
                            color: Theme.textMuted
                            font.pixelSize: 13
                            elide: Text.ElideRight
                        }
                    }
                }
                Accessible.name: root.displayTitle(modelData)
                onClicked: root.chatSelected(modelData.jid, root.displayTitle(modelData))
            }

            Label {
                anchors.centerIn: parent
                visible: contactsList.count === 0
                text: qsTr("No matching contacts")
                color: Theme.textMuted
            }
        }
    }
}
