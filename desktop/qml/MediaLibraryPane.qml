import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// WhatsApp Web's "Media from all chats": a header with the three categories in
// the middle, then the pictures as a grid of square tiles grouped by the day
// they were sent, each labelled with the conversation it came from. Documents
// and links stay lists, as they are there.
ColumnLayout {
    id: root
    spacing: 0

    property string activeCategory: "media"
    property bool searchActive: false
    property string searchText: ""
    property bool oldestFirst: false
    property bool selectionActive: false
    property var selectedItems: []

    signal messageRequested(string chatJid, string messageId)
    signal forwardRequested(var items)
    signal closeRequested()

    function isSelected(item) {
        const id = String(item.id || "")
        for (let i = 0; i < root.selectedItems.length; ++i) {
            if (String(root.selectedItems[i].id) === id)
                return true
        }
        return false
    }

    function toggleSelection(item) {
        const id = String(item.id || "")
        const next = []
        let found = false
        for (let i = 0; i < root.selectedItems.length; ++i) {
            if (String(root.selectedItems[i].id) === id) {
                found = true
                continue
            }
            next.push(root.selectedItems[i])
        }
        if (!found)
            next.push({ id: id, chat_jid: String(item.chat_jid || "") })
        root.selectedItems = next
    }

    function endSelection() {
        root.selectionActive = false
        root.selectedItems = []
    }

    function reload(category) {
        root.activeCategory = category
        backend.refreshMediaLibrary(category, false)
    }

    function localUrl(path) {
        const value = String(path || "")
        if (value === "")
            return ""
        return Theme.fileUrl(value)
    }

    function itemLabel(item) {
        return String(item.chat_title || item.sender_name || item.chat_jid || "")
    }

    // Filtering happens here rather than in the daemon: the page is already in
    // memory, and a round trip per keystroke would make the field feel stuck.
    readonly property var visibleItems: {
        const source = backend.mediaLibrary || []
        const needle = root.searchText.trim().toLowerCase()
        const kept = []
        for (let i = 0; i < source.length; ++i) {
            const item = source[i]
            if (needle !== "") {
                const haystack = [item.media_name, item.link_title, item.body, root.itemLabel(item)]
                    .join(" ").toLowerCase()
                if (haystack.indexOf(needle) < 0)
                    continue
            }
            kept.push(item)
        }
        if (root.oldestFirst)
            kept.reverse()
        return kept
    }

    // The grid is grouped by day, so the model is a list of days rather than of
    // pictures; a GridView cannot carry section headers.
    readonly property var dayGroups: {
        const groups = []
        const items = root.visibleItems
        let currentKey = ""
        for (let i = 0; i < items.length; ++i) {
            const when = new Date(Number(items[i].timestamp || 0))
            const key = Qt.formatDate(when, "yyyy-MM-dd")
            if (key !== currentKey) {
                currentKey = key
                groups.push({ key: key, when: when, items: [] })
            }
            groups[groups.length - 1].items.push(items[i])
        }
        return groups
    }

    function durationText(seconds) {
        const total = Math.max(0, Math.round(Number(seconds || 0)))
        const minutes = Math.floor(total / 60)
        const rest = total % 60
        return minutes + ":" + (rest < 10 ? "0" + rest : rest)
    }

    function dayHeading(when) {
        const today = new Date()
        const sameDay = (a, b) => a.getFullYear() === b.getFullYear()
            && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
        if (sameDay(when, today))
            return qsTr("Today")
        const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000)
        if (sameDay(when, yesterday))
            return qsTr("Yesterday")
        return Qt.formatDate(when, "dddd")
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.preferredHeight: 72
        color: Theme.surface

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 22
            anchors.rightMargin: 16
            spacing: 16

            ColumnLayout {
                Layout.preferredWidth: 240
                spacing: 2
                Label {
                    text: qsTr("Media")
                    color: Theme.text
                    font.pixelSize: 19
                    font.weight: Font.Medium
                }
                Label {
                    text: qsTr("Media from all chats")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    elide: Text.ElideRight
                }
            }

            Item { Layout.fillWidth: true }

            // Underlined tabs, the way the PWA draws this header; the pill chips
            // belong to the chat list, not here.
            RowLayout {
                spacing: 0
                Repeater {
                    model: [
                        { key: "media", label: qsTr("Media") },
                        { key: "documents", label: qsTr("Documents") },
                        { key: "links", label: qsTr("Links") }
                    ]
                    AbstractButton {
                        id: tab
                        required property var modelData
                        objectName: "mediaLibraryTab_" + modelData.key
                        readonly property bool current: root.activeCategory === modelData.key
                        implicitWidth: Math.max(120, tabLabel.implicitWidth + 32)
                        implicitHeight: 56
                        Accessible.name: modelData.label
                        onClicked: root.reload(modelData.key)
                        background: Rectangle {
                            color: tab.hovered && !tab.current ? Theme.hoverRow : "transparent"
                            Rectangle {
                                anchors.bottom: parent.bottom
                                anchors.left: parent.left
                                anchors.right: parent.right
                                height: 3
                                visible: tab.current
                                color: Theme.primary
                            }
                        }
                        contentItem: Label {
                            id: tabLabel
                            text: tab.modelData.label
                            color: tab.current ? Theme.primary : Theme.text
                            font.pixelSize: 15
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                    }
                }
            }

            Item { Layout.fillWidth: true }

            ThemedToolButton {
                objectName: "mediaLibrarySearchButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/search.svg")
                iconSize: 20
                Accessible.name: qsTr("Search media")
                onClicked: {
                    root.searchActive = !root.searchActive
                    if (!root.searchActive)
                        root.searchText = ""
                    else
                        mediaSearchField.forceActiveFocus()
                }
                background: Rectangle { radius: 20; color: parent.hovered || root.searchActive ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
            ThemedToolButton {
                objectName: "mediaLibrarySelectButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/check.svg")
                iconSize: 20
                Accessible.name: root.selectionActive ? qsTr("Cancel selection") : qsTr("Select items")
                onClicked: root.selectionActive ? root.endSelection() : root.selectionActive = true
                background: Rectangle { radius: 20; color: parent.hovered || root.selectionActive ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
            ThemedToolButton {
                objectName: "mediaLibrarySortButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/sort.svg")
                iconSize: 20
                Accessible.name: root.oldestFirst ? qsTr("Show newest first") : qsTr("Show oldest first")
                onClicked: root.oldestFirst = !root.oldestFirst
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
            ThemedToolButton {
                objectName: "mediaLibraryCloseButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconSize: 20
                Accessible.name: qsTr("Close")
                onClicked: root.closeRequested()
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
        }
    }

    Rectangle {
        objectName: "mediaSelectionBar"
        Layout.fillWidth: true
        Layout.preferredHeight: root.selectionActive ? 56 : 0
        visible: root.selectionActive
        color: Theme.surfaceMuted
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 22
            anchors.rightMargin: 16
            spacing: 12
            Label {
                objectName: "mediaSelectionCountLabel"
                text: root.selectedItems.length === 0
                    ? qsTr("Select items to forward")
                    : qsTr("%1 selected").arg(root.selectedItems.length)
                color: Theme.text
                font.pixelSize: 14
            }
            Item { Layout.fillWidth: true }
            ThemedToolButton {
                objectName: "mediaSelectionForwardButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                enabled: root.selectedItems.length > 0
                iconSource: Qt.resolvedUrl("icons/forward.svg")
                iconSize: 20
                Accessible.name: qsTr("Forward")
                onClicked: root.forwardRequested(root.selectedItems)
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
            ThemedToolButton {
                objectName: "mediaSelectionCancelButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconSize: 20
                Accessible.name: qsTr("Cancel selection")
                onClicked: root.endSelection()
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
        }
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.preferredHeight: root.searchActive ? 56 : 0
        visible: root.searchActive
        color: Theme.surface
        DialogTextField {
            id: mediaSearchField
            objectName: "mediaLibrarySearchField"
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 22
            anchors.rightMargin: 22
            search: true
            placeholderText: qsTr("Search media, documents and links")
            onTextChanged: root.searchText = text
        }
    }

    // The grid of pictures, grouped by day.
    ListView {
        id: mediaDays
        objectName: "mediaLibraryGrid"
        Layout.fillWidth: true
        Layout.fillHeight: true
        visible: root.activeCategory === "media"
        clip: true
        model: root.visible && root.activeCategory === "media" ? root.dayGroups : []
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: OverlayScrollBar {}
        onAtYEndChanged: if (atYEnd && backend.mediaLibraryHasMore && !backend.mediaLibraryLoading)
            backend.refreshMediaLibrary(root.activeCategory, true)

        readonly property int columnCount: Math.max(2, Math.floor(width / 300))
        readonly property int cellSize: Math.floor((width - 2) / columnCount) - 2

        delegate: Column {
            required property var modelData
            width: mediaDays.width
            spacing: 0

            Column {
                padding: 16
                spacing: 2
                Label {
                    text: root.dayHeading(modelData.when)
                    color: Theme.text
                    font.pixelSize: 15
                    font.weight: Font.Medium
                }
                Label {
                    text: Qt.formatDate(modelData.when, "d MMMM yyyy")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
            }

            Grid {
                columns: mediaDays.columnCount
                spacing: 2
                leftPadding: 1

                Repeater {
                    model: modelData.items
                    delegate: Rectangle {
                        id: tile
                        required property var modelData
                        width: mediaDays.cellSize
                        height: mediaDays.cellSize
                        color: Theme.surfaceMuted
                        clip: true

                        Image {
                            id: tileImage
                            objectName: "mediaLibraryTile"
                            anchors.fill: parent
                            fillMode: Image.PreserveAspectCrop
                            asynchronous: true
                            cache: false
                            source: root.localUrl(tile.modelData.media_thumbnail || tile.modelData.media_path)
                        }

                        TintedIcon {
                            anchors.centerIn: parent
                            width: 34
                            height: 34
                            visible: tileImage.status === Image.Null || tileImage.status === Image.Error
                            source: Qt.resolvedUrl(String(tile.modelData.kind) === "video" ? "icons/video.svg" : "icons/gallery.svg")
                            tint: Theme.icon
                        }

                        // A video is only distinguishable from a photo by its
                        // badge once both are cropped to the same square, and
                        // WhatsApp Web puts the running time next to it.
                        Rectangle {
                            anchors.centerIn: parent
                            width: 44
                            height: 44
                            radius: 22
                            visible: String(tile.modelData.kind) === "video"
                            color: "#66000000"
                            TintedIcon {
                                anchors.centerIn: parent
                                anchors.horizontalCenterOffset: 1
                                width: 20
                                height: 20
                                source: Qt.resolvedUrl("icons/play.svg")
                                tint: "#FFFFFF"
                            }
                        }

                        Label {
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            anchors.margins: 10
                            visible: String(tile.modelData.kind) === "video"
                                && Number(tile.modelData.media_duration || 0) > 0
                            text: root.durationText(tile.modelData.media_duration)
                            color: "#FFFFFF"
                            font.pixelSize: 13
                        }

                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            height: 46
                            gradient: Gradient {
                                GradientStop { position: 0.0; color: "#00000000" }
                                GradientStop { position: 1.0; color: "#B3000000" }
                            }
                            Label {
                                objectName: "mediaLibraryTileLabel"
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.bottom: parent.bottom
                                anchors.margins: 10
                                text: root.itemLabel(tile.modelData)
                                color: "#FFFFFF"
                                font.pixelSize: 14
                                elide: Text.ElideRight
                            }
                        }

                        Rectangle {
                            anchors.top: parent.top
                            anchors.right: parent.right
                            anchors.margins: 10
                            width: 24
                            height: 24
                            radius: 12
                            visible: root.selectionActive
                            color: root.isSelected(tile.modelData) ? Theme.primary : "#66000000"
                            border.width: root.isSelected(tile.modelData) ? 0 : 2
                            border.color: "#FFFFFF"
                            TintedIcon {
                                anchors.centerIn: parent
                                width: 15
                                height: 15
                                visible: root.isSelected(tile.modelData)
                                source: Qt.resolvedUrl("icons/check.svg")
                                tint: Theme.primaryText
                            }
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.selectionActive) {
                                    root.toggleSelection(tile.modelData)
                                    return
                                }
                                root.messageRequested(String(tile.modelData.chat_jid || ""),
                                                      String(tile.modelData.id || ""))
                            }
                        }
                    }
                }
            }
        }
    }

    // Documents and links keep the row layout they have in the PWA.
    ListView {
        id: libraryList
        objectName: "mediaLibraryList"
        Layout.fillWidth: true
        Layout.fillHeight: true
        visible: root.activeCategory !== "media"
        clip: true
        model: root.visible && root.activeCategory !== "media" ? root.visibleItems : []
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: OverlayScrollBar {}
        onAtYEndChanged: if (atYEnd && backend.mediaLibraryHasMore && !backend.mediaLibraryLoading)
            backend.refreshMediaLibrary(root.activeCategory, true)

        delegate: ItemDelegate {
            required property var modelData
            width: ListView.view ? ListView.view.width : 0
            height: 64
            onClicked: root.messageRequested(String(modelData.chat_jid || ""), String(modelData.id || ""))
            contentItem: RowLayout {
                spacing: 13
                Item {
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    Layout.leftMargin: 22
                    Rectangle {
                        anchors.fill: parent
                        radius: 8
                        color: Theme.surfaceMuted
                    }
                    Image {
                        id: rowThumbnail
                        anchors.fill: parent
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        source: root.localUrl(modelData.media_thumbnail || modelData.link_thumbnail)
                    }
                    TintedIcon {
                        anchors.centerIn: parent
                        width: 20
                        height: 20
                        visible: rowThumbnail.status === Image.Null || rowThumbnail.status === Image.Error
                        source: Qt.resolvedUrl(root.activeCategory === "documents" ? "icons/document.svg" : "icons/link.svg")
                        tint: Theme.icon
                    }
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    spacing: 2
                    Label {
                        Layout.fillWidth: true
                        text: modelData.media_name || modelData.link_title || modelData.body || modelData.kind || ""
                        color: Theme.text
                        font.pixelSize: 15
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                    Label {
                        Layout.fillWidth: true
                        text: root.itemLabel(modelData)
                        color: Theme.textMuted
                        font.pixelSize: 13
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                }
                Label {
                    Layout.rightMargin: 22
                    text: modelData.timestamp ? Qt.formatDate(new Date(modelData.timestamp), "d MMM") : ""
                    color: Theme.textMuted
                    font.pixelSize: 12
                }
            }
        }
    }

    Column {
        Layout.fillWidth: true
        Layout.fillHeight: true
        Layout.topMargin: 80
        spacing: 8
        visible: root.visibleItems.length === 0 && !backend.mediaLibraryLoading
        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.searchText.trim() !== "" ? qsTr("No matches") : qsTr("Nothing shared yet")
            color: Theme.text
            font.pixelSize: 17
        }
        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.searchText.trim() !== ""
                ? qsTr("No media, document or link matches that search.")
                : qsTr("Photos, documents and links from every chat collect here.")
            color: Theme.textMuted
            font.pixelSize: 13
        }
    }
}
