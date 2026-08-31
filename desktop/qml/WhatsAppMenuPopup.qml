import QtQuick
import QtQuick.Controls
import org.whatsappgo

Popup {
    id: root
    default property alias menuItems: menuColumn.data

    width: 268
    implicitHeight: menuColumn.implicitHeight + topPadding + bottomPadding
    padding: 6
    modal: false
    focus: true
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

    enter: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: 90 }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: 70 }
    }

    background: Item {
        Rectangle {
            anchors.fill: parent
            anchors.leftMargin: 2
            anchors.topMargin: 4
            color: Theme.dark ? "#52000000" : "#26000000"
            radius: 12
        }
        Rectangle {
            anchors.fill: parent
            anchors.rightMargin: 2
            anchors.bottomMargin: 4
            color: Theme.surfaceRaised
            border.color: Theme.border
            border.width: 1
            radius: 12
        }
    }

    contentItem: Column {
        id: menuColumn
        spacing: 1
    }
}
