import QtQuick
import QtQuick.Controls
import org.whatsappgo

Button {
    id: root
    property bool selected: false

    implicitHeight: 40
    implicitWidth: Math.max(40, contentItem.implicitWidth + 20)
    leftPadding: 10
    rightPadding: 10
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    checkable: true
    checked: selected
    Accessible.role: Accessible.RadioButton
    Accessible.name: text
    Accessible.checked: checked

    contentItem: Label {
        text: root.text
        color: root.selected ? (Theme.dark ? Theme.text : "#006B55") : Theme.textMuted
        font.pixelSize: 13
        font.weight: root.selected ? Font.DemiBold : Font.Medium
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideNone
    }

    background: Rectangle {
        radius: height / 2
        color: root.selected
            ? (Theme.dark ? Theme.primaryContainer : "#E7FCE3")
            : root.down ? Theme.pressedRow
            : root.hovered || root.activeFocus ? Theme.hoverRow
            : Theme.surface
        border.width: root.activeFocus ? 2 : 1
        border.color: root.activeFocus ? Theme.primary
            : root.selected ? (Theme.dark ? Theme.primary : "#A9DCA4")
            : Theme.dark ? "#3B4A54" : "#D1D7DB"
    }
}
