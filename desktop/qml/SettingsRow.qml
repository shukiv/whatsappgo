import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// One line in the settings tree: an icon, a label, an optional value or button
// on the right, and a whole-row press.
ItemDelegate {
    id: root

    property url iconSource
    property string trailingText: ""
    property string actionText: ""
    property bool destructive: false
    signal actionClicked()

    implicitHeight: 56
    enabled: true

    background: Rectangle {
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
    }

    contentItem: RowLayout {
        spacing: 16
        TintedIcon {
            Layout.leftMargin: 22
            Layout.preferredWidth: 20
            Layout.preferredHeight: 20
            source: root.iconSource
            tint: root.destructive ? Theme.danger : Theme.icon
        }
        Label {
            Layout.fillWidth: true
            text: root.text
            color: root.destructive ? Theme.danger : Theme.text
            font.pixelSize: 15
            elide: Text.ElideRight
        }
        Label {
            visible: root.trailingText !== ""
            text: root.trailingText
            color: Theme.textMuted
            font.pixelSize: 14
        }
        AbstractButton {
            id: action
            objectName: "settingsRowAction"
            visible: root.actionText !== ""
            Layout.rightMargin: 22
            implicitWidth: actionLabel.implicitWidth + 28
            implicitHeight: 32
            Accessible.name: root.actionText
            onClicked: root.actionClicked()
            background: Rectangle {
                radius: 16
                color: action.hovered ? Theme.hoverRow : "transparent"
                border.width: 1
                border.color: Theme.border
            }
            contentItem: Label {
                id: actionLabel
                text: root.actionText
                color: Theme.primary
                font.pixelSize: 13
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
        }
        Item {
            visible: root.actionText === ""
            Layout.rightMargin: 22
            Layout.preferredWidth: 0
        }
    }
}
