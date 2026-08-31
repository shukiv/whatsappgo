import QtQuick
import QtQuick.Controls
import org.whatsappgo

ScrollBar {
    id: control
    policy: ScrollBar.AsNeeded
    interactive: true
    minimumSize: 0.06
    padding: 2
    implicitWidth: 10
    implicitHeight: 10

    background: Item {}

    contentItem: Rectangle {
        visible: control.size < 1.0
        implicitWidth: 6
        implicitHeight: 36
        radius: width / 2
        color: control.hovered || control.pressed ? Theme.scrollbarHandleHover : Theme.scrollbarHandle
        opacity: control.active || control.hovered || control.pressed ? 0.9 : 0.55

        Behavior on color { ColorAnimation { duration: 100 } }
        Behavior on opacity { NumberAnimation { duration: 100 } }
    }
}
