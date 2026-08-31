import QtQuick
import QtQuick.Controls
import org.whatsappgo

ToolButton {
    id: control

    property url iconSource
    property color iconTint: Theme.icon
    property int iconSize: 22

    contentItem: Item {
        TintedIcon {
            anchors.centerIn: parent
            width: control.iconSize
            height: control.iconSize
            source: control.iconSource
            tint: control.iconTint
        }
    }
}
