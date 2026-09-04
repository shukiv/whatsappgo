import QtQuick
import QtQuick.Controls
import org.whatsappgo

Item {
    id: root
    signal newListRequested()
    property string selectedFilter: "all"
    property int unreadCount: 0
    signal filterSelected(string filter)

    implicitHeight: 42

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
                text: qsTr("Unread")
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
                visible: root.width >= 440
                text: qsTr("Groups")
                selected: root.selectedFilter === "groups"
                onClicked: root.selectFilter("groups")
            }
            Repeater {
                model: backend.chatLabels
                ChatFilterChip {
                    required property var modelData
                    readonly property string filterKey: "label:" + String(modelData.id)
                    visible: root.width >= 440
                    text: modelData.name || modelData.id
                    selected: root.selectedFilter === filterKey
                    onClicked: root.selectFilter(filterKey)
                }
            }
            ThemedToolButton {
                objectName: "filterAddButton"
                width: 32
                height: 32
                // WhatsApp Web shows a "+" that makes a new list once the chips
                // fit, and collapses to a chevron over the hidden ones when they
                // do not. Showing "+" while chips are hidden would describe the
                // wrong action.
                readonly property bool collapsed: root.width < 440
                iconSource: Qt.resolvedUrl(collapsed ? "icons/chevron-right.svg" : "icons/plus.svg")
                iconSize: collapsed ? 14 : 16
                rotation: collapsed ? 90 : 0
                Accessible.name: collapsed ? qsTr("More chat filters") : qsTr("New list")
                onClicked: collapsed ? filterMenu.open() : root.newListRequested()
                background: Rectangle {
                    radius: width / 2
                    color: parent.down ? Theme.pressedRow
                        : parent.hovered || parent.activeFocus ? Theme.hoverRow
                        : Theme.surface
                    border.width: parent.activeFocus ? 2 : 1
                    border.color: parent.activeFocus ? Theme.primary
                        : Theme.dark ? "#3B4A54" : "#D1D7DB"
                }
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
