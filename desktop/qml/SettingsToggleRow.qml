import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// A settings line that carries a switch. The whole row toggles, the way it does
// in the PWA, rather than only the control at the end.
ItemDelegate {
    id: root

    property string description: ""
    // `checked` and `toggled` are final on AbstractButton, so the switch state
    // travels under its own name.
    property bool on: false
    signal switched(bool value)

    implicitHeight: root.description === "" ? 56 : 72
    onClicked: root.switched(!root.on)

    background: Rectangle {
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
    }

    contentItem: RowLayout {
        spacing: 16
        ColumnLayout {
            Layout.leftMargin: 22
            Layout.fillWidth: true
            spacing: 2
            Label {
                Layout.fillWidth: true
                text: root.text
                color: Theme.text
                font.pixelSize: 15
                elide: Text.ElideRight
            }
            Label {
                Layout.fillWidth: true
                visible: root.description !== ""
                text: root.description
                color: Theme.textMuted
                font.pixelSize: 13
                elide: Text.ElideRight
            }
        }
        Rectangle {
            Layout.rightMargin: 22
            Layout.preferredWidth: 40
            Layout.preferredHeight: 22
            radius: 11
            color: root.on ? Theme.primary : Theme.surfaceMuted
            border.width: root.on ? 0 : 1
            border.color: Theme.border
            Rectangle {
                width: 18
                height: 18
                radius: 9
                y: 2
                x: root.on ? parent.width - width - 2 : 2
                color: root.on ? Theme.primaryText : Theme.textMuted
                Behavior on x { NumberAnimation { duration: 120 } }
            }
        }
    }
}
