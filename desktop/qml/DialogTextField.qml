import QtQuick
import QtQuick.Controls
import org.whatsappgo

// The application's own field for dialogs and pickers: the same rounded, borderless
// input the composer and the chat search use, instead of the platform style.
TextField {
    id: root

    property bool search: false

    implicitHeight: 40
    leftPadding: root.search ? 40 : 16
    rightPadding: 16
    color: Theme.text
    placeholderTextColor: Theme.textMuted
    font.pixelSize: 14
    selectByMouse: true
    Accessible.name: placeholderText

    background: Rectangle {
        radius: 8
        color: Theme.surfaceMuted
        border.width: root.activeFocus ? 1 : 0
        border.color: Theme.primary
    }

    TintedIcon {
        anchors.left: parent.left
        anchors.leftMargin: 12
        anchors.verticalCenter: parent.verticalCenter
        width: 18
        height: 18
        visible: root.search
        source: Qt.resolvedUrl("icons/search.svg")
        tint: Theme.textMuted
    }
}
