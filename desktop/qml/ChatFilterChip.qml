import QtQuick
import QtQuick.Controls
import org.whatsappgo

Button {
    id: root
    property bool selected: false

    implicitHeight: 32
    implicitWidth: Math.max(32, contentItem.implicitWidth + 24)
    leftPadding: 12
    rightPadding: 12
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    checkable: true
    checked: selected
    Accessible.role: Accessible.RadioButton
    Accessible.name: text
    Accessible.checked: checked

    contentItem: Label {
        text: root.text
        color: root.selected ? Theme.filterChipSelectedText : Theme.textMuted
        // Measured from the live client: 13.33 px at regular weight in both
        // states. The chip does not gain weight when it is selected; the filled
        // pill is what marks the selection.
        font.pixelSize: 13
        font.weight: Font.Normal
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideNone
    }

    background: Rectangle {
        radius: height / 2
        color: root.selected
            ? Theme.primaryContainer
            : root.down ? Theme.pressedRow
            : root.hovered || root.activeFocus ? Theme.hoverRow
            : Theme.surface
        // The live client outlines both states with the same hairline; only the
        // fill distinguishes a selected filter.
        border.width: root.activeFocus ? 2 : 1
        border.color: root.activeFocus ? Theme.primary : Theme.filterChipBorder
    }
}
