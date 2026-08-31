import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    property url iconSource
    property bool destructive: false

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
            tint: root.destructive ? Theme.danger : Theme.text
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
            visible: root.checkable && root.checked
            text: "✓"
            color: Theme.primary
            font.pixelSize: 16
            font.weight: Font.DemiBold
            Accessible.ignored: true
        }
    }
}
