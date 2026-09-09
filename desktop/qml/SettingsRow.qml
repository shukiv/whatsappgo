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
    property string description: ""
    property bool showChevron: false
    signal actionClicked()

    implicitHeight: Math.max(description ? 68 : 56, contentItem.implicitHeight + 16)
    enabled: true
    opacity: enabled ? 1 : 0.5
    focusPolicy: Qt.StrongFocus
    Accessible.name: text
    Accessible.description: description

    background: Rectangle {
        radius: 10
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
        border.width: root.visualFocus ? 2 : 0
        border.color: Theme.primary
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
        ColumnLayout {
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            spacing: 3
            Label {
                Layout.fillWidth: true
                text: root.text
                textFormat: Text.PlainText
                color: root.destructive ? Theme.danger : Theme.text
                font.pixelSize: 15
                wrapMode: Text.Wrap
            }
            Label {
                Layout.fillWidth: true
                visible: root.description !== ""
                text: root.description
                textFormat: Text.PlainText
                color: Theme.textMuted
                font.pixelSize: 13
                wrapMode: Text.Wrap
            }
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
            visible: root.actionText === "" && !root.showChevron
            Layout.rightMargin: 22
            Layout.preferredWidth: 0
        }
        TintedIcon {
            visible: root.showChevron
            Layout.rightMargin: 22
            Layout.preferredWidth: 16
            Layout.preferredHeight: 16
            source: Qt.resolvedUrl("icons/chevron-right.svg")
            tint: Theme.iconMuted
        }
    }
}
