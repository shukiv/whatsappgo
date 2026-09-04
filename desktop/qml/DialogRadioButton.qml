import QtQuick
import QtQuick.Controls
import org.whatsappgo

// The application's own radio, so a choice inside a dialog is drawn in the
// product's palette rather than the desktop platform's.
RadioButton {
    id: root

    implicitHeight: 32
    Accessible.name: text

    indicator: Rectangle {
        x: 0
        y: (root.height - height) / 2
        width: 18
        height: 18
        radius: 9
        color: "transparent"
        border.width: 2
        border.color: root.checked ? Theme.primary : Theme.textMuted
        Rectangle {
            anchors.centerIn: parent
            width: 10
            height: 10
            radius: 5
            visible: root.checked
            color: Theme.primary
        }
    }

    contentItem: Label {
        leftPadding: 28
        text: root.text
        color: Theme.text
        font.pixelSize: 14
        verticalAlignment: Text.AlignVCenter
    }

    background: null
}
