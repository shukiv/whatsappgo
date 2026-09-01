import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ThemedToolButton {
    id: root
    property var profiles: []
    property string currentProfile: "default"
    property var unreadCounts: ({})
    property Item popupParent
    signal switchRequested(string profile)

    readonly property int totalUnread: {
        let total = 0
        for (let i = 0; i < profiles.length; ++i)
            total += Number(unreadCounts[profiles[i]] || 0)
        return total
    }

    width: 44
    height: 44
    iconSource: Qt.resolvedUrl("icons/user.svg")
    iconSize: 20
    Accessible.name: qsTr("Switch account. Current account: %1").arg(currentProfile)
    Accessible.description: totalUnread > 0
        ? qsTr("%1 unread messages across all accounts").arg(totalUnread)
        : qsTr("No unread messages")
    onClicked: accountMenu.opened ? accountMenu.close() : accountMenu.open()

    contentItem: Item {
        TintedIcon {
            anchors.centerIn: parent
            width: 20
            height: 20
            source: root.iconSource
            tint: Theme.icon
        }

        Rectangle {
            id: unreadBadge
            objectName: "accountSwitcherUnreadBadge"
            visible: root.totalUnread > 0
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.topMargin: 2
            anchors.rightMargin: 1
            width: root.totalUnread > 99 ? 26 : 19
            height: 19
            radius: height / 2
            color: Theme.primary
            border.color: Theme.surface
            border.width: 2

            Label {
                anchors.centerIn: parent
                text: root.totalUnread > 99 ? "99+" : root.totalUnread
                color: Theme.primaryText
                font.pixelSize: 10
                font.weight: Font.Bold
            }
        }
    }

    background: Rectangle {
        radius: width / 2
        color: root.down ? Theme.pressedRow : root.hovered || root.activeFocus ? Theme.hoverRow : "transparent"
        border.color: root.activeFocus ? Theme.primary : "transparent"
        border.width: root.activeFocus ? 2 : 0
    }

    WhatsAppMenuPopup {
        id: accountMenu
        objectName: "accountSwitcherMenu"
        parent: root.popupParent || root
        width: 244
        x: root.popupParent
            ? Math.max(8, Math.min(root.popupParent.width - width - 8,
                                   root.mapToItem(root.popupParent, 0, 0).x + root.width - width))
            : 0
        y: root.popupParent
            ? root.mapToItem(root.popupParent, 0, 0).y + root.height + 2
            : root.height + 2

        Item {
            width: parent.width
            height: 34
            Label {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: qsTr("Accounts")
                color: Theme.textMuted
                font.pixelSize: 12
                font.weight: Font.DemiBold
            }
        }

        Repeater {
            model: root.profiles
            WhatsAppMenuItem {
                required property string modelData
                objectName: "accountSwitcherMenuItem"
                text: modelData
                trailingText: Number(root.unreadCounts[modelData] || 0) > 0
                    ? String(root.unreadCounts[modelData])
                    : ""
                iconSource: Qt.resolvedUrl("icons/user.svg")
                checkable: true
                checked: modelData === root.currentProfile
                onClicked: {
                    accountMenu.close()
                    root.switchRequested(modelData)
                }
            }
        }
    }
}
