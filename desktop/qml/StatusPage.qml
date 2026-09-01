import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

RowLayout {
    id: root
    property var groups: []
    property string ownName: qsTr("My status")
    property url ownAvatar
    signal groupRequested(int index)
    signal avatarRequested(string jid)
    spacing: 0

    function relativeTime(milliseconds) {
        const date = new Date(Number(milliseconds || 0))
        const today = new Date()
        const sameDay = date.getFullYear() === today.getFullYear()
            && date.getMonth() === today.getMonth() && date.getDate() === today.getDate()
        const time = Qt.formatTime(date, "HH:mm")
        return sameDay ? qsTr("Today at %1").arg(time) : qsTr("Yesterday at %1").arg(time)
    }

    Rectangle {
        Layout.preferredWidth: Math.max(420, Math.min(560, root.width * 0.34))
        Layout.fillHeight: true
        color: Theme.surface
        border.color: Theme.border

        ColumnLayout {
            anchors.fill: parent
            spacing: 0

            RowLayout {
                Layout.fillWidth: true
                Layout.preferredHeight: 72
                Layout.leftMargin: 24
                Layout.rightMargin: 16

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Status")
                    color: Theme.text
                    font.pixelSize: 22
                    font.weight: Font.DemiBold
                }
                ThemedToolButton {
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    iconSource: Qt.resolvedUrl("icons/menu.svg")
                    Accessible.name: qsTr("Status menu")
                    background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }
                ThemedToolButton {
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    Accessible.name: qsTr("Add status update")
                    contentItem: Label {
                        text: "+"
                        color: Theme.text
                        font.pixelSize: 27
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }
            }

            ItemDelegate {
                Layout.fillWidth: true
                Layout.preferredHeight: 86
                leftPadding: 24
                rightPadding: 20
                background: Rectangle { color: parent.hovered ? Theme.hoverRow : Theme.surface }
                contentItem: RowLayout {
                    spacing: 14
                    Item {
                        Layout.preferredWidth: 56
                        Layout.preferredHeight: 56
                        Avatar {
                            anchors.fill: parent
                            diameter: 56
                            title: root.ownName
                            source: root.ownAvatar
                            fallbackIdentity: !root.ownAvatar
                        }
                        Rectangle {
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            width: 22
                            height: 22
                            radius: 11
                            color: Theme.primary
                            border.color: Theme.surface
                            border.width: 2
                            Label {
                                anchors.centerIn: parent
                                text: "+"
                                color: Theme.primaryText
                                font.pixelSize: 17
                                font.weight: Font.Bold
                            }
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 2
                        Label {
                            Layout.fillWidth: true
                            text: qsTr("My status")
                            color: Theme.text
                            font.pixelSize: 16
                            font.weight: Font.Medium
                        }
                        Label {
                            Layout.fillWidth: true
                            text: qsTr("Click to add a status update")
                            color: Theme.textMuted
                            font.pixelSize: 14
                            elide: Text.ElideRight
                        }
                    }
                }
            }

            Label {
                Layout.fillWidth: true
                Layout.leftMargin: 24
                Layout.topMargin: 24
                Layout.bottomMargin: 12
                text: qsTr("RECENT")
                color: Theme.textMuted
                font.pixelSize: 12
                font.weight: Font.Medium
            }

            ListView {
                id: statusGroupList
                objectName: "statusGroupList"
                Layout.fillWidth: true
                Layout.fillHeight: true
                model: root.groups
                clip: true
                reuseItems: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: OverlayScrollBar {}

                delegate: ItemDelegate {
                    required property var modelData
                    required property int index
                    width: ListView.view.width
                    height: 90
                    leftPadding: 24
                    rightPadding: 20
                    background: Rectangle {
                        color: parent.hovered ? Theme.hoverRow : Theme.surface
                    }
                    contentItem: RowLayout {
                        spacing: 14
                        StatusAvatar {
                            Layout.preferredWidth: 60
                            Layout.preferredHeight: 60
                            diameter: 60
                            title: modelData.sender_name || "?"
                            source: modelData.avatar_path ? "file://" + modelData.avatar_path : ""
                            itemCount: modelData.items ? modelData.items.length : 1
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            Layout.minimumWidth: 0
                            spacing: 3
                            Label {
                                Layout.fillWidth: true
                                text: modelData.sender_name || qsTr("Unknown contact")
                                color: Theme.text
                                font.pixelSize: 16
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                            }
                            Label {
                                Layout.fillWidth: true
                                text: root.relativeTime(modelData.latest_at)
                                color: Theme.textMuted
                                font.pixelSize: 14
                                elide: Text.ElideRight
                            }
                        }
                    }
                    onClicked: root.groupRequested(index)
                    Component.onCompleted: {
                        if (!modelData.avatar_path && modelData.sender_jid)
                            root.avatarRequested(modelData.sender_jid)
                    }
                }

                Label {
                    anchors.centerIn: parent
                    visible: statusGroupList.count === 0
                    text: qsTr("No recent status updates")
                    color: Theme.textMuted
                    font.pixelSize: 14
                }
            }
        }
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.fillHeight: true
        color: Theme.emptyBackground

        ColumnLayout {
            anchors.centerIn: parent
            width: Math.min(parent.width - 64, 520)
            spacing: 14
            Label {
                Layout.alignment: Qt.AlignHCenter
                text: "◎"
                color: Theme.icon
                font.pixelSize: 60
            }
            Label {
                Layout.fillWidth: true
                text: qsTr("Share status updates")
                color: Theme.text
                font.pixelSize: 29
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                Layout.fillWidth: true
                text: qsTr("Share photos, videos and text that disappear after 24 hours.")
                color: Theme.textMuted
                font.pixelSize: 15
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
            }
        }
    }
}
