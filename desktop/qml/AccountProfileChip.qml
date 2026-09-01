import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Button {
    id: root
    required property string profileName
    property int unreadCount: 0

    width: Math.max(62, chipContent.implicitWidth + 22)
    height: 34
    checkable: true
    Accessible.name: unreadCount > 0
        ? qsTr("Switch to account %1, %2 unread messages").arg(profileName).arg(unreadCount)
        : qsTr("Switch to account %1").arg(profileName)

    contentItem: RowLayout {
        id: chipContent
        spacing: 6

        Label {
            text: root.profileName
            color: root.checked ? Theme.primary : Theme.textMuted
            font.pixelSize: 12
            font.weight: root.checked ? Font.DemiBold : Font.Normal
            verticalAlignment: Text.AlignVCenter
        }

        Rectangle {
            id: unreadBadge
            objectName: "accountUnreadBadge"
            visible: root.unreadCount > 0
            Layout.preferredWidth: root.unreadCount > 99 ? 25 : 18
            Layout.preferredHeight: 18
            radius: 9
            color: Theme.primary

            Label {
                anchors.centerIn: parent
                text: root.unreadCount > 99 ? "99+" : root.unreadCount
                color: Theme.primaryText
                font.pixelSize: 10
                font.weight: Font.Bold
            }
        }
    }

    background: Rectangle {
        radius: 17
        color: root.checked ? Theme.primaryContainer : root.hovered ? Theme.hoverRow : Theme.surfaceMuted
        border.color: root.checked ? Theme.primary : Theme.border
    }
}
