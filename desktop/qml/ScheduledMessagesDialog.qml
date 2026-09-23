import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Everything this conversation still owes, with a way to take any of it back.
// A message nobody can see yet is a message that has to be cancellable, or
// scheduling one is a decision that cannot be undone.
WhatsAppDialog {
    id: root
    objectName: "scheduledMessagesDialog"

    property var messages: []

    signal cancelRequested(string id)

    title: qsTr("Scheduled messages")
    showAccept: false
    cancelText: qsTr("Close")
    preferredWidth: 460
    chatScoped: true

    // A Repeater rather than a list view: a conversation holds a handful of
    // these at most, and every row exists whether or not it has been scrolled
    // to, which is what makes them reachable. It is wrapped in a column of its
    // own so it has a visual parent to build the rows into.
    ColumnLayout {
        Layout.fillWidth: true
        spacing: 8

    Repeater {
        id: scheduledRepeater
        objectName: "scheduledMessagesList"
        model: root.messages
        delegate: Rectangle {
            Layout.fillWidth: true
            radius: 8
            color: Theme.surfaceMuted
            implicitHeight: row.implicitHeight + 20

            RowLayout {
                id: row
                anchors.fill: parent
                anchors.margins: 10
                spacing: 10

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 2
                    Label {
                        Layout.fillWidth: true
                        text: Qt.formatDateTime(new Date(Number(modelData.send_at || 0)), "dddd d MMMM, HH:mm")
                        color: Theme.primary
                        font.pixelSize: 12
                        font.weight: Font.DemiBold
                    }
                    Label {
                        objectName: "scheduledMessageText"
                        Layout.fillWidth: true
                        text: String(modelData.text || "")
                        color: Theme.text
                        font.pixelSize: 13
                        wrapMode: Text.Wrap
                        maximumLineCount: 3
                        elide: Text.ElideRight
                    }
                    Label {
                        // Whatever went wrong last time. The message is still
                        // in the queue, so this is a note rather than an end.
                        Layout.fillWidth: true
                        visible: String(modelData.error || "") !== ""
                        text: qsTr("Last attempt failed: %1").arg(String(modelData.error || ""))
                        color: Theme.danger
                        font.pixelSize: 12
                        wrapMode: Text.Wrap
                    }
                }

                AbstractButton {
                    objectName: "cancelScheduledButton"
                    implicitWidth: cancelRowLabel.implicitWidth + 24
                    implicitHeight: 32
                    Accessible.name: qsTr("Cancel this message")
                    onClicked: root.cancelRequested(String(modelData.id || ""))
                    background: Rectangle {
                        radius: 16
                        color: parent.hovered ? Theme.hoverRow : "transparent"
                        border.color: Theme.border
                        border.width: 1
                    }
                    contentItem: Label {
                        id: cancelRowLabel
                        text: qsTr("Cancel")
                        color: Theme.danger
                        font.pixelSize: 13
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                }
            }
        }
    }

    }

    Label {
        Layout.fillWidth: true
        visible: root.messages.length === 0
        text: qsTr("Nothing is waiting to be sent in this conversation.")
        color: Theme.textMuted
        font.pixelSize: 13
        wrapMode: Text.Wrap
    }
}
