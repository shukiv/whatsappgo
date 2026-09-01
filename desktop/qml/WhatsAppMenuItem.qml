import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    property url iconSource
    property bool destructive: false
    property color iconTint: destructive ? Theme.danger : Theme.text
    property string trailingText: ""
    property url actionIconSource
    property string actionAccessibleName: ""
    property string actionObjectName: ""
    signal actionTriggered()

    width: parent ? parent.width : 256
    height: 46
    leftPadding: 12
    rightPadding: 12
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.name: text
    Accessible.checked: checkable && checked

    background: Rectangle {
        color: root.down ? Theme.pressedRow : root.hovered || root.activeFocus ? Theme.hoverRow : "transparent"
        radius: 7
        border.color: root.activeFocus ? Theme.primary : "transparent"
        border.width: root.activeFocus ? 2 : 0
    }

    contentItem: RowLayout {
        spacing: 13

        TintedIcon {
            Layout.preferredWidth: 20
            Layout.preferredHeight: 20
            source: root.iconSource
            tint: root.iconTint
        }

        Label {
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            text: root.text
            color: root.destructive ? Theme.danger : Theme.text
            font.pixelSize: 14
            elide: Text.ElideRight
        }

        Label {
            visible: root.trailingText.length > 0
            text: root.trailingText
            color: Theme.primary
            font.pixelSize: 12
            font.weight: Font.DemiBold
        }

        ThemedToolButton {
            objectName: root.actionObjectName
            visible: root.actionIconSource.toString().length > 0
            Layout.preferredWidth: 40
            Layout.preferredHeight: 40
            iconSource: root.actionIconSource
            iconSize: 17
            Accessible.name: root.actionAccessibleName
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            background: Rectangle {
                radius: width / 2
                color: parent.down ? Theme.pressedRow
                    : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                border.color: parent.activeFocus ? Theme.primary : "transparent"
                border.width: parent.activeFocus ? 2 : 0
            }
            onClicked: root.actionTriggered()
        }

        Label {
            visible: root.checkable && root.checked
            text: "✓"
            color: Theme.primary
            font.pixelSize: 16
            font.weight: Font.DemiBold
            Accessible.ignored: true
        }
    }
}
