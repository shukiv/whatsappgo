import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.whatsappgo

ItemDelegate {
    id: root
    required property var modelData
    required property bool current
    signal chosen(string jid, string title)

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

    width: ListView.view ? ListView.view.width : 360
    height: 84
    padding: 0
    leftPadding: 12
    rightPadding: 12
    hoverEnabled: true
    clip: true
    Accessible.name: displayTitle + (modelData.unread_count > 0 ? qsTr(", %1 unread messages").arg(modelData.unread_count) : "")
    onClicked: chosen(modelData.jid, displayTitle)

    background: Rectangle {
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
        spacing: 14

        Avatar {
            Layout.preferredWidth: 56
            Layout.preferredHeight: 56
            Layout.alignment: Qt.AlignVCenter
            diameter: 56
            title: root.displayTitle
            fallbackIdentity: root.fallbackIdentity
            source: root.modelData.avatar_path ? "file://" + root.modelData.avatar_path : ""
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.minimumWidth: 0

            Item {
                id: statusColumn
                width: 46
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.right: parent.right

                Label {
                    objectName: "chatTimestamp"
                    anchors.top: parent.top
                    anchors.topMargin: 15
                    anchors.right: parent.right
                    width: parent.width
                    text: root.modelData.last_message_at ? Qt.formatTime(new Date(root.modelData.last_message_at), "HH:mm") : ""
                    color: root.modelData.unread_count > 0 ? Theme.primary : Theme.textMuted
                    font.pixelSize: 12
                    font.weight: root.modelData.unread_count > 0 ? Font.Medium : Font.Normal
                    horizontalAlignment: Text.AlignRight
                }

                Rectangle {
                    objectName: "unreadBadge"
                    visible: root.modelData.unread_count > 0
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: 12
                    width: root.modelData.unread_count > 99 ? 30 : 24
                    height: 24
                    radius: height / 2
                    color: Theme.primary
                    Label {
                        id: unreadLabel
                        anchors.centerIn: parent
                        text: root.modelData.unread_count > 99 ? "99+" : root.modelData.unread_count
                        color: Theme.primaryText
                        font.pixelSize: 12
                        font.weight: Font.DemiBold
                    }
                }
            }

            Label {
                anchors.left: parent.left
                anchors.right: statusColumn.left
                anchors.rightMargin: 8
                anchors.top: parent.top
                anchors.topMargin: 14
                text: root.displayTitle
                color: Theme.text
                font.pixelSize: 17
                font.weight: Font.Medium
                elide: Text.ElideRight
                maximumLineCount: 1
            }

            Label {
                objectName: "chatPreview"
                anchors.left: parent.left
                anchors.right: statusColumn.left
                anchors.rightMargin: 8
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 14
                text: Theme.emojiRichText(String(root.modelData.last_message_preview || qsTr("No messages yet")).replace(/\s+/g, " ").trim())
                color: Theme.textMuted
                font.pixelSize: 14
                elide: Text.ElideRight
                maximumLineCount: 1
                textFormat: Text.RichText
                clip: true
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.leftMargin: 82
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: Theme.border
    }
}
