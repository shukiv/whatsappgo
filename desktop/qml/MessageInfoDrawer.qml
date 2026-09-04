import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Rectangle {
    id: root
    objectName: "messageInfoDrawer"

    property bool opened: false
    property var message: ({})
    signal closeRequested()

    width: Math.min(540, parent ? parent.width : 540)
    visible: opened
    color: Theme.surface
    border.width: 1
    border.color: Theme.border
    z: 42
    focus: opened
    Keys.onEscapePressed: closeRequested()
    Accessible.role: Accessible.Pane
    Accessible.name: qsTr("Message info")

    readonly property int statusRank: {
        switch (String(message.status || "")) {
        case "played": return 4
        case "read": return 3
        case "delivered": return 2
        case "sent": return 1
        default: return 0
        }
    }

    function formatTimestamp(value, reached) {
        let timestamp = Number(value || 0)
        if (timestamp <= 0)
            return reached ? qsTr("Time unavailable") : qsTr("Pending")
        if (timestamp < 1000000000000)
            timestamp *= 1000
        const date = new Date(timestamp)
        const today = new Date()
        const sameDay = date.getFullYear() === today.getFullYear()
            && date.getMonth() === today.getMonth() && date.getDate() === today.getDate()
        const day = sameDay ? qsTr("Today") : Qt.formatDate(date, Qt.DefaultLocaleShortDate)
        return qsTr("%1 at %2").arg(day).arg(Qt.formatTime(date, Qt.DefaultLocaleShortDate))
    }

    readonly property var milestones: {
        const rows = []
        if (message.kind === "audio" || message.kind === "video") {
            rows.push({ label: qsTr("Played"), kind: "played",
                        reached: statusRank >= 4, timestamp: Number(message.played_at || 0) })
        }
        rows.push({ label: qsTr("Read"), kind: "read",
                    reached: statusRank >= 3, timestamp: Number(message.read_at || 0) })
        rows.push({ label: qsTr("Delivered"), kind: "delivered",
                    reached: statusRank >= 2, timestamp: Number(message.delivered_at || 0) })
        return rows
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
                anchors.rightMargin: 16
                spacing: 8

                ThemedToolButton {
                    objectName: "messageInfoCloseButton"
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    iconSource: Qt.resolvedUrl("icons/close.svg")
                    iconSize: 22
                    Accessible.name: qsTr("Close message info")
                    onClicked: root.closeRequested()
                    background: Rectangle { radius: 24; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Message info")
                    color: Theme.text
                    font.pixelSize: 18
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

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
            ScrollBar.vertical: OverlayScrollBar {}

            Column {
                width: parent.width
                spacing: 0

                Rectangle {
                    width: parent.width
                    height: Math.max(160, messagePreview.implicitHeight + 40)
                    color: Theme.chatBackground
                    clip: true

                    Image {
                        anchors.fill: parent
                        source: Qt.resolvedUrl("assets/chat-background.png")
                        fillMode: Image.Tile
                        opacity: Theme.patternOpacity
                    }

                    MessageDelegate {
                        id: messagePreview
						objectName: "messageInfoPreview"
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 16
                        anchors.rightMargin: 16
                        modelData: root.message
                        actionsEnabled: false
                    }
                }

                Repeater {
                    model: root.milestones

                    Rectangle {
                        required property var modelData
                        width: parent.width
                        height: 104
                        color: Theme.surface

                        RowLayout {
                            anchors.fill: parent
                            anchors.leftMargin: 24
                            anchors.rightMargin: 24
                            spacing: 18

                            Item {
                                Layout.preferredWidth: 28
                                Layout.preferredHeight: 28

                                TintedIcon {
                                    visible: modelData.kind === "played"
                                    anchors.fill: parent
                                    anchors.margins: 3
                                    source: Qt.resolvedUrl("icons/mic.svg")
                                    tint: modelData.reached ? Theme.readReceipt : Theme.textMuted
                                }

                                ReadReceipt {
                                    visible: modelData.kind !== "played"
                                    anchors.centerIn: parent
                                    scale: 1.55
                                    status: modelData.kind === "read" && modelData.reached
                                        ? "read" : modelData.reached ? "delivered" : "sent"
                                }
                            }

                            ColumnLayout {
                                Layout.fillWidth: true
                                spacing: 5

                                Label {
                                    Layout.fillWidth: true
                                    text: modelData.label
                                    color: Theme.text
                                    font.pixelSize: 17
                                    font.weight: Font.Medium
                                }
                                Label {
                                    Layout.fillWidth: true
                                    text: root.formatTimestamp(modelData.timestamp, modelData.reached)
                                    color: Theme.textMuted
                                    font.pixelSize: 14
                                }
                            }
                        }

                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            anchors.leftMargin: 70
                            height: 1
                            color: Theme.border
                        }
                    }
                }
            }
        }
    }
}
