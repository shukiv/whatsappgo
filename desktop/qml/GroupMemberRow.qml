import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    objectName: "groupMember_" + String(member.jid || "")
    property var member: ({})
    signal avatarRequested(string jid)
    width: parent ? parent.width : 440
    height: 72
    Accessible.name: (member.is_self ? qsTr("You") : String(member.name || ""))
        + (member.is_admin ? ", " + qsTr("Group admin") : "")
    background: Rectangle {
        radius: 10
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
        border.width: root.activeFocus ? 1 : 0
        border.color: Theme.primary
    }
    contentItem: RowLayout {
        spacing: 14
        Avatar {
            Layout.leftMargin: 12
            diameter: 48
            title: root.member.name || ""
            source: Theme.fileUrl(root.member.avatar_path || "")
            fallbackIdentity: !root.member.avatar_path
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 4
            Label { Layout.fillWidth: true; text: root.member.is_self ? qsTr("You") : root.member.name || qsTr("WhatsApp member"); color: Theme.text; font.pixelSize: 16; elide: Text.ElideRight }
            Label { Layout.fillWidth: true; visible: Boolean(root.member.phone); text: "+" + String(root.member.phone || ""); color: Theme.textMuted; font.pixelSize: 12; elide: Text.ElideRight }
        }
        Rectangle {
            Layout.rightMargin: 12
            visible: Boolean(root.member.is_admin)
            implicitWidth: badge.implicitWidth + 12
            implicitHeight: 24
            radius: 4
            color: Theme.primaryContainer
            Label { id: badge; anchors.centerIn: parent; text: qsTr("Group admin"); color: Theme.filterChipSelectedText; font.pixelSize: 11 }
        }
    }
    function fetchAvatar() {
        if (visible && member.jid && !member.avatar_path)
            avatarRequested(member.jid)
    }
    Component.onCompleted: fetchAvatar()
    ListView.onReused: fetchAvatar()
}
