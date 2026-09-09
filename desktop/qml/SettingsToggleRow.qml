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

    implicitHeight: Math.max(root.description === "" ? 56 : 72,
        titleLabel.implicitHeight + (root.description ? detailLabel.implicitHeight + 2 : 0)
        + topPadding + bottomPadding + 20)
    opacity: enabled ? 1 : 0.5
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.CheckBox
    Accessible.name: text
    Accessible.description: description
    Accessible.checked: on
    onClicked: root.switched(!root.on)

    background: Rectangle {
        border.width: root.visualFocus ? 2 : 0
        border.color: Theme.primary
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
    }

    contentItem: RowLayout {
        spacing: 16
        ColumnLayout {
            Layout.leftMargin: 22
            Layout.fillWidth: true
            spacing: 2
            Label {
                id: titleLabel
                Layout.fillWidth: true
                text: root.text
                textFormat: Text.PlainText
                color: Theme.text
                font.pixelSize: 15
                wrapMode: Text.Wrap
            }
            Label {
                id: detailLabel
                Layout.fillWidth: true
                visible: root.description !== ""
                text: root.description
                textFormat: Text.PlainText
                color: Theme.textMuted
                font.pixelSize: 13
                wrapMode: Text.Wrap
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
