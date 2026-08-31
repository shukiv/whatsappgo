import QtQuick
import QtQuick.Controls
import org.whatsappgo

Item {
    id: root
    property string selectedFilter: "all"
    property int unreadCount: 0
    signal filterSelected(string filter)

    implicitHeight: 52

    function selectFilter(filter) {
        selectedFilter = filter
        filterSelected(filter)
    }

    function accepts(chat) {
        if (!chat)
            return false
        if (selectedFilter === "unread")
            return Number(chat.unread_count || 0) > 0
        if (selectedFilter === "favorites")
            return Boolean(chat.favorite)
        if (selectedFilter === "groups")
            return Boolean(chat.is_group)
        return true
    }

    Flickable {
        id: filterFlick
        anchors.fill: parent
        contentWidth: filterRow.width + 24
        contentHeight: height
        flickableDirection: Flickable.HorizontalFlick
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Row {
            id: filterRow
            x: 12
            anchors.verticalCenter: parent.verticalCenter
            spacing: 8

            ChatFilterChip {
                objectName: "filterAllButton"
                text: qsTr("All")
                selected: root.selectedFilter === "all"
                onClicked: root.selectFilter("all")
            }
            ChatFilterChip {
                objectName: "filterUnreadButton"
                text: root.unreadCount > 0 ? qsTr("Unread %1").arg(root.unreadCount) : qsTr("Unread")
                selected: root.selectedFilter === "unread"
                onClicked: root.selectFilter("unread")
            }
            ChatFilterChip {
                objectName: "filterFavoritesButton"
                text: qsTr("Favorites")
                selected: root.selectedFilter === "favorites"
                onClicked: root.selectFilter("favorites")
            }
            ChatFilterChip {
                objectName: "filterGroupsButton"
                text: qsTr("Groups")
                selected: root.selectedFilter === "groups"
                onClicked: root.selectFilter("groups")
            }
            ChatFilterChip {
                objectName: "filterAddButton"
                text: "+"
                Accessible.name: qsTr("More chat filters")
                onClicked: filterMenu.open()
            }
        }
    }

    WhatsAppMenuPopup {
        id: filterMenu
        width: 210
        x: Math.max(8, root.width - width - 12)
        y: root.height - 2

        WhatsAppMenuItem {
            text: qsTr("Unread chats")
            checkable: true
            checked: root.selectedFilter === "unread"
            onClicked: { root.selectFilter("unread"); filterMenu.close() }
        }
        WhatsAppMenuItem {
            text: qsTr("Favorite chats")
            checkable: true
            checked: root.selectedFilter === "favorites"
            onClicked: { root.selectFilter("favorites"); filterMenu.close() }
        }
        WhatsAppMenuItem {
            text: qsTr("Groups")
            checkable: true
            checked: root.selectedFilter === "groups"
            onClicked: { root.selectFilter("groups"); filterMenu.close() }
        }
    }
}
