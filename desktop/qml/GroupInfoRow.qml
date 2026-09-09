import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    property url iconSource
    property string subtitle: ""
    property bool destructive: false
    opacity: enabled ? 1 : 0.5
    width: parent ? parent.width : 440
    implicitHeight: Math.max(subtitle ? 72 : 64, row.implicitHeight + 24)
    Accessible.name: text + (subtitle ? ". " + subtitle : "")
    background: Rectangle {
        radius: 10
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
        border.width: root.activeFocus ? 1 : 0
        border.color: Theme.primary
    }
    contentItem: RowLayout {
        id: row
        spacing: 22
        TintedIcon { Layout.leftMargin: 14; Layout.preferredWidth: 24; Layout.preferredHeight: 24; source: root.iconSource; tint: root.destructive ? Theme.danger : Theme.icon }
        ColumnLayout {
            Layout.fillWidth: true
            Layout.rightMargin: 18
            spacing: 3
            Label { Layout.fillWidth: true; text: root.text; color: root.destructive ? Theme.danger : Theme.text; font.pixelSize: 15; wrapMode: Text.Wrap }
            Label { Layout.fillWidth: true; visible: root.subtitle !== ""; text: root.subtitle; color: Theme.textMuted; font.pixelSize: 13; wrapMode: Text.Wrap }
        }
    }
}
