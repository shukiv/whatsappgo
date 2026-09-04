import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window

ThemedToolButton {
    id: root
    property var profiles: []
    property string currentProfile: "default"
    property var displayNames: ({})
    property var unreadCounts: ({})
    property Item popupParent
    // The menu is clamped inside whatever it is parented to. A caller that does
    // not name a surface still gets the window's own, so the menu cannot open
    // past the edge of the window as it did on the pairing page.
    readonly property Item menuSurface: popupParent
        || (Window.window ? Window.window.contentItem : null)
    signal switchRequested(string profile)
    signal removeRequested(string profile)
    signal renameRequested(string profile)

    function displayName(profile) {
        return String(displayNames[profile] || profile)
    }

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
    Accessible.name: qsTr("Switch account. Current account: %1").arg(displayName(currentProfile))
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
        parent: root.menuSurface || root
        width: 244
        x: root.menuSurface
            ? Math.max(8, Math.min(root.menuSurface.width - width - 8,
                                   root.mapToItem(root.menuSurface, 0, 0).x + root.width - width))
            : 0
        y: root.menuSurface
            ? Math.max(8, Math.min(root.menuSurface.height - implicitHeight - 8,
                                   root.mapToItem(root.menuSurface, 0, 0).y + root.height + 2))
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
                text: root.displayName(modelData)
                trailingText: Number(root.unreadCounts[modelData] || 0) > 0
                    ? String(root.unreadCounts[modelData])
                    : ""
                iconSource: Qt.resolvedUrl("icons/user.svg")
                actionIconSource: Qt.resolvedUrl("icons/edit.svg")
                actionAccessibleName: qsTr("Rename %1").arg(root.displayName(modelData))
                actionObjectName: "accountRenameButton"
                // The first account holds the shared application data, so it has
                // nothing to remove; an account is also never the last one left.
                secondaryActionIconSource: backend.profileRemovable(modelData)
                    ? Qt.resolvedUrl("icons/delete.svg") : ""
                secondaryActionAccessibleName: qsTr("Remove %1").arg(root.displayName(modelData))
                secondaryActionObjectName: "accountRemoveButton"
                checkable: true
                checked: modelData === root.currentProfile
                onActionTriggered: {
                    accountMenu.close()
                    root.renameRequested(modelData)
                }
                onSecondaryActionTriggered: {
                    accountMenu.close()
                    root.removeRequested(modelData)
                }
                onClicked: {
                    accountMenu.close()
                    root.switchRequested(modelData)
                }
            }
        }
    }
}
