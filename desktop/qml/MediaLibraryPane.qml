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
    property string senderFilter: "all"
    property bool largestFirst: false
    property bool selectionActive: false
    property var selectedItems: []
    property var menuIdentity: null
    readonly property var menuMessage: {
        const items = backend.mediaLibrary || []
        if (!menuIdentity || menuIdentity.profile !== backend.profile)
            return null
        for (let item of items) {
            if (item.id === menuIdentity.id && item.chat_jid === menuIdentity.chat_jid)
                return item
        }
        return null
    }
    readonly property bool menuEligible: !!menuMessage && !menuMessage.revoked
        && menuMessage.kind !== "view_once"

    signal messageRequested(string chatJid, string messageId)
    signal forwardRequested(var items)
    signal closeRequested()

    function isSelected(item) {
        const id = String(item.id || "")
        for (let i = 0; i < root.selectedItems.length; ++i) {
            if (String(root.selectedItems[i].id) === id
                    && String(root.selectedItems[i].chat_jid) === String(item.chat_jid || ""))
                return true
        }
        return false
    }

    function toggleSelection(item) {
        const id = String(item.id || "")
        const next = []
        let found = false
        for (let i = 0; i < root.selectedItems.length; ++i) {
            if (String(root.selectedItems[i].id) === id
                    && String(root.selectedItems[i].chat_jid) === String(item.chat_jid || "")) {
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
        itemMenu.close()
        sortMenu.close()
        root.endSelection()
        root.activeCategory = category
        backend.refreshMediaLibrary(category, false)
    }

    function activateItem(item) {
        if (!item || !item.id || !item.chat_jid || item.revoked || item.kind === "view_once")
            return
        if (root.selectionActive)
            root.toggleSelection(item)
        else
            root.messageRequested(String(item.chat_jid), String(item.id))
    }

    function openItemMenu(item, anchor) {
        if (!item || !item.id || !item.chat_jid || root.selectionActive)
            return
        root.menuIdentity = { id: item.id, chat_jid: item.chat_jid, profile: backend.profile }
        sortMenu.close()
        itemMenu.openUnder(anchor)
    }

    function performItemAction(action) {
        const item = root.menuMessage
        const eligible = root.menuEligible && root.visible
        itemMenu.close()
        if (!eligible)
            return
        switch (action) {
        case "select":
            root.selectionActive = true
            if (!root.isSelected(item)) root.toggleSelection(item)
            break
        case "jump": root.messageRequested(String(item.chat_jid), String(item.id)); break
        case "forward": root.forwardRequested([{ id: String(item.id), chat_jid: String(item.chat_jid) }]); break
        case "star": backend.starLibraryItem(item, !item.starred, backend.profile); break
        case "download": backend.downloadLibraryItem(item, backend.profile); break
        }
    }

    function resetLibraryInteraction() {
        itemMenu.close()
        sortMenu.close()
        menuIdentity = null
        root.endSelection()
    }

    onVisibleChanged: {
        if (!visible) resetLibraryInteraction()
        // Main.showSection always refreshes the Media category on entry.
        // Do not render that payload as rows under a retained Links tab.
        else activeCategory = "media"
    }
    Connections {
        target: backend
        function onProfileChanged() {
            root.resetLibraryInteraction()
            root.searchText = ""
            root.searchActive = false
        }
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
            if (item.revoked || item.kind === "view_once") continue
            if (root.senderFilter === "you" && !item.from_me) continue
            if (root.senderFilter === "others" && item.from_me) continue
            if (needle !== "") {
                const haystack = [item.media_name, item.link_title, item.body, root.itemLabel(item)]
                    .join(" ").toLowerCase()
                if (haystack.indexOf(needle) < 0)
                    continue
            }
            kept.push(item)
        }
        kept.sort((a, b) => {
            if (root.largestFirst) {
                const size = Number(b.media_size || 0) - Number(a.media_size || 0)
                if (size !== 0) return size
            }
            const date = Number(a.timestamp || 0) - Number(b.timestamp || 0)
            if (date !== 0) return root.oldestFirst && !root.largestFirst ? date : -date
            return (String(a.chat_jid) + "/" + String(a.id)).localeCompare(String(b.chat_jid) + "/" + String(b.id))
        })
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
            const key = root.largestFirst ? "largest" : Qt.formatDate(when, "yyyy-MM-dd")
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
        Layout.preferredHeight: root.width < 980 ? 120 : 72
        color: Theme.surface

        RowLayout {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 72
            anchors.leftMargin: 22
            anchors.rightMargin: 16
            spacing: root.width < 980 ? 6 : 16

            ColumnLayout {
                Layout.preferredWidth: 240
                Layout.minimumWidth: 0
                Layout.fillWidth: root.width < 980
                spacing: 2
                Label {
                    Layout.fillWidth: true
                    text: qsTr("Media")
                    color: Theme.text
                    font.pixelSize: 19
                    font.weight: Font.Medium
                }
                Label {
                    Layout.fillWidth: true
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
                id: categoryTabs
                parent: root.width < 980 ? compactTabHost : headerTabHost
                anchors.centerIn: parent
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
                        implicitWidth: root.width < 980 ? Math.min(140, (root.width - 20) / 3) : Math.max(120, tabLabel.implicitWidth + 32)
                        implicitHeight: 56
                        Accessible.name: modelData.label
                        onClicked: root.reload(modelData.key)
                        background: Rectangle {
                            color: tab.hovered && !tab.current ? Theme.hoverRow : "transparent"
                            border.width: tab.activeFocus ? 2 : 0
                            border.color: Theme.primary
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

            Item {
                id: headerTabHost
                visible: root.width >= 980
                Layout.preferredWidth: 390
                Layout.preferredHeight: 56
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
                id: sortButton
                objectName: "mediaLibrarySortButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/sort.svg")
                iconSize: 20
                Accessible.name: qsTr("Sort and filter media")
                onClicked: { itemMenu.close(); sortMenu.toggleUnder(sortButton) }
                background: Rectangle { radius: 20; color: parent.hovered || sortMenu.visible ? Theme.hoverRow : "transparent" }
                ToolTip.visible: hovered && !sortMenu.visible
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
        Item {
            id: compactTabHost
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 48
            visible: root.width < 980
        }
    }

    Rectangle {
        objectName: "mediaSelectionBar"
        Layout.fillWidth: true
        Layout.preferredHeight: root.selectionActive ? Math.max(56, libraryBulkActions.implicitHeight + 12) : 0
        visible: root.selectionActive
        color: Theme.surfaceMuted
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 22
            anchors.rightMargin: 16
            spacing: 12
            BulkMediaActions {
                id: libraryBulkActions
                objectName: "mediaSelectionCountLabel"
                Layout.fillWidth: true
                scope: "library"
                items: (backend.mediaLibrary || []).filter(item => root.isSelected(item))
            }
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
            text: root.searchText
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
                    text: root.largestFirst ? qsTr("Largest files") : root.dayHeading(modelData.when)
                    color: Theme.text
                    font.pixelSize: 15
                    font.weight: Font.Medium
                }
                Label {
                    visible: !root.largestFirst
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
                    delegate: AbstractButton {
                        id: tile
                        objectName: "mediaLibraryItem_" + modelData.id
                        required property var modelData
                        width: mediaDays.cellSize
                        height: mediaDays.cellSize
                        clip: true
                        hoverEnabled: true
                        focusPolicy: Qt.StrongFocus
                        Accessible.name: root.itemLabel(modelData) + ", " + String(modelData.media_name || modelData.kind || "")
                        Accessible.checkable: root.selectionActive
                        Accessible.checked: root.isSelected(modelData)
                        onClicked: root.activateItem(modelData)
                        Keys.onReturnPressed: root.activateItem(modelData)
                        Keys.onEnterPressed: root.activateItem(modelData)
                        Keys.onPressed: event => {
                            if (event.key === Qt.Key_Menu || (event.key === Qt.Key_F10 && event.modifiers === Qt.ShiftModifier)) {
                                root.openItemMenu(modelData, tile)
                                event.accepted = true
                            }
                        }
                        background: Rectangle { color: Theme.surfaceMuted }

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
                                textFormat: Text.PlainText
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
                            acceptedButtons: Qt.RightButton
                            cursorShape: Qt.PointingHandCursor
                            onClicked: { tile.forceActiveFocus(); root.openItemMenu(tile.modelData, tile) }
                        }
                        ThemedToolButton {
                            anchors.top: parent.top
                            anchors.right: parent.right
                            anchors.margins: 6
                            width: 36; height: 36
                            visible: !root.selectionActive && (tile.hovered || tile.activeFocus || activeFocus)
                            iconSource: Qt.resolvedUrl("icons/chevron-down.svg")
                            iconTint: "#FFFFFF"
                            Accessible.name: qsTr("Message menu")
                            background: Rectangle { radius: 18; color: "#99000000" }
                            onClicked: root.openItemMenu(tile.modelData, this)
                        }
                        Rectangle {
                            anchors.fill: parent
                            color: "transparent"
                            border.width: tile.activeFocus ? 3 : 0
                            border.color: Theme.primary
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
            id: libraryRow
            objectName: "mediaLibraryItem_" + modelData.id
            required property var modelData
            width: ListView.view ? ListView.view.width : 0
            height: 64
            Accessible.name: String(modelData.media_name || modelData.link_title || modelData.body || modelData.kind || "") + ", " + root.itemLabel(modelData)
            Accessible.checkable: root.selectionActive
            Accessible.checked: root.isSelected(modelData)
            onClicked: root.activateItem(modelData)
            Keys.onReturnPressed: root.activateItem(modelData)
            Keys.onEnterPressed: root.activateItem(modelData)
            Keys.onPressed: event => {
                if (event.key === Qt.Key_Menu || (event.key === Qt.Key_F10 && event.modifiers === Qt.ShiftModifier)) {
                    root.openItemMenu(modelData, libraryRow)
                    event.accepted = true
                }
            }
            background: Rectangle {
                color: libraryRow.hovered || (root.selectionActive && root.isSelected(libraryRow.modelData)) ? Theme.hoverRow : Theme.surface
                border.width: libraryRow.activeFocus ? 2 : 0
                border.color: Theme.primary
            }
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.RightButton
                onClicked: { libraryRow.forceActiveFocus(); root.openItemMenu(libraryRow.modelData, libraryRow) }
            }
            contentItem: RowLayout {
                spacing: 13
                CheckBox {
                    visible: root.selectionActive
                    checked: root.isSelected(libraryRow.modelData)
                    Accessible.name: qsTr("Select %1").arg(libraryRow.Accessible.name)
                    onClicked: root.toggleSelection(libraryRow.modelData)
                }
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
                        textFormat: Text.PlainText
                        color: Theme.text
                        font.pixelSize: 15
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                    Label {
                        Layout.fillWidth: true
                        text: root.itemLabel(modelData)
                        textFormat: Text.PlainText
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
                ThemedToolButton {
                    visible: !root.selectionActive
                    Layout.preferredWidth: 36
                    Layout.preferredHeight: 36
                    iconSource: Qt.resolvedUrl("icons/chevron-down.svg")
                    Accessible.name: qsTr("Message menu")
                    onClicked: root.openItemMenu(libraryRow.modelData, this)
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
            text: root.searchText.trim() !== "" || root.senderFilter !== "all"
                ? qsTr("No matches") : qsTr("Nothing shared yet")
            color: Theme.text
            font.pixelSize: 17
        }
        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.searchText.trim() !== "" || root.senderFilter !== "all"
                ? qsTr("No loaded items match your search and filters.")
                : qsTr("Photos, documents and links from every chat collect here.")
            color: Theme.textMuted
            font.pixelSize: 13
        }
    }

    RowLayout {
        Layout.fillWidth: true
        Layout.margins: 12
        visible: backend.mediaLibraryHasMore || backend.mediaLibraryLoading
        Label {
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            text: qsTr("Sorting and filters apply to loaded items.")
            wrapMode: Text.WordWrap
            color: Theme.textMuted
            font.pixelSize: 12
        }
        Button {
            objectName: "mediaLibraryLoadMore"
            text: backend.mediaLibraryLoading ? qsTr("Loading…") : qsTr("Load more")
            enabled: !backend.mediaLibraryLoading
            onClicked: backend.refreshMediaLibrary(root.activeCategory, true)
        }
    }

    WhatsAppMenuPopup {
        id: sortMenu
        objectName: "mediaLibrarySortMenu"
        parent: Overlay.overlay
        Label { text: qsTr("Sent by"); padding: 10; color: Theme.textMuted }
        Repeater {
            model: [{key:"all", label:qsTr("Everyone")}, {key:"you", label:qsTr("You")}, {key:"others", label:qsTr("Others")}]
            WhatsAppMenuItem {
                required property var modelData
                objectName: "mediaFilter_" + modelData.key
                text: modelData.label
                checkable: true
                checked: root.senderFilter === modelData.key
                onClicked: { root.senderFilter = modelData.key; sortMenu.close() }
            }
        }
        MenuSeparator { width: parent.width }
        Label { text: qsTr("Sort by"); padding: 10; color: Theme.textMuted }
        Repeater {
            model: [{key:"newest", label:qsTr("Newest")}, {key:"oldest", label:qsTr("Oldest")}, {key:"largest", label:qsTr("Largest")}]
            WhatsAppMenuItem {
                required property var modelData
                objectName: "mediaSort_" + modelData.key
                text: modelData.label
                checkable: true
                checked: modelData.key === (root.largestFirst ? "largest" : root.oldestFirst ? "oldest" : "newest")
                onClicked: {
                    root.largestFirst = modelData.key === "largest"
                    root.oldestFirst = modelData.key === "oldest"
                    sortMenu.close()
                }
            }
        }
    }

    WhatsAppMenuPopup {
        id: itemMenu
        objectName: "mediaLibraryItemMenu"
        parent: Overlay.overlay
        Repeater {
            model: [
                {key:"select", label:qsTr("Select"), icon:"check"},
                {key:"jump", label:qsTr("Go to message"), icon:"chats"},
                {key:"download", label:qsTr("Download"), icon:"download"},
                {key:"forward", label:qsTr("Forward"), icon:"forward"},
                {key:"star", label:root.menuMessage && root.menuMessage.starred ? qsTr("Unstar") : qsTr("Star"), icon:"star"}
            ]
            WhatsAppMenuItem {
                required property var modelData
                objectName: "mediaItem_" + modelData.key
                text: modelData.label
                iconSource: Qt.resolvedUrl("icons/" + modelData.icon + ".svg")
                visible: modelData.key !== "download" || (root.menuMessage && ["image", "video", "audio", "document", "sticker"].indexOf(String(root.menuMessage.kind)) >= 0)
                enabled: root.menuEligible && (modelData.key !== "download" || backend.documentDownloads.indexOf(String(root.menuMessage.chat_jid) + "/" + String(root.menuMessage.id)) < 0)
                    && (modelData.key !== "star" || !backend.libraryStarBusy)
                onClicked: root.performItemAction(modelData.key)
            }
        }
    }
}
