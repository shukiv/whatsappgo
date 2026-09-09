import QtQuick
import QtQuick.Controls
import org.whatsappgo

Button {
    id: control
    property bool primary: false
    implicitHeight: 44
    implicitWidth: Math.max(80, label.implicitWidth + 28)
    padding: 8
    font.pixelSize: 14
    opacity: enabled ? 1 : 0.5
    contentItem: Label {
        id: label
        text: control.text
        font: control.font
        color: control.primary ? Theme.primaryText : control.checked ? Theme.primary : Theme.text
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    background: Rectangle {
        radius: 22
        color: control.primary ? Theme.primary : control.checked || control.hovered ? Theme.hoverRow : "transparent"
        border.color: control.activeFocus || control.checked ? Theme.primary : Theme.border
        border.width: control.activeFocus ? 2 : 1
    }
}
