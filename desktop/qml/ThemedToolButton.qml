import QtQuick
import QtQuick.Controls
import org.whatsappgo

ToolButton {
    id: control

    property url iconSource
    property color iconTint: Theme.icon
    property int iconSize: 22
    // Turns the icon while something the reader started is still running.
    property bool iconSpinning: false

    contentItem: Item {
        TintedIcon {
            id: buttonIcon
            anchors.centerIn: parent
            width: control.iconSize
            height: control.iconSize
            source: control.iconSource
            tint: control.iconTint
            transformOrigin: Item.Center

            RotationAnimator {
                target: buttonIcon
                running: control.iconSpinning
                from: 0
                to: 360
                duration: 900
                loops: Animation.Infinite
                onRunningChanged: {
                    if (!running)
                        buttonIcon.rotation = 0
                }
            }
        }
    }
}
