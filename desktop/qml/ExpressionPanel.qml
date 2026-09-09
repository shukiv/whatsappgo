import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window
import QtQuick.Dialogs
import org.whatsappgo

ColumnLayout {
    id: root
    objectName: "expressionPanel"
    required property var client
    property string kind: "gif"
    property string targetProfile: ""
    property string targetChat: ""
    property string targetTitle: ""
    property string replyTo: ""
    property var selected: null
    property bool previewPaused: false
    onSelectedChanged: previewPaused = false
    property bool favorites: false
    property string sendingToken: ""
    property string sendingProfile: ""
    property string sendingChat: ""
    property string sendError: ""
    property int viewGeneration: 0
    property int sendingGeneration: 0
    property string pickerProfile: ""
    property string pickerChat: ""
    readonly property bool creating: !!selected && selected.created === true
    readonly property bool sending: sendingToken !== ""
    signal sent()
    signal settingsRequested()
    spacing: 8

    function start() {
        selected = null
        sendError = ""
        search.text = ""
        if (kind === "gif") catalog.search("")
        else if (client) client.loadStickers(favorites)
    }
    function clear() {
        ++viewGeneration
        stickerFilePicker.close()
        maker.clear()
        debounce.stop()
        if (playerLoader.item) playerLoader.item.stop()
        catalog.clear()
        selected = null
        search.text = ""
    }
    function back() {
        if (!selected) return false
        ++viewGeneration
        maker.clear()
        sendError = ""
        selected = null
        if (playerLoader.item) playerLoader.item.stop()
        catalog.cancelPreview()
        Qt.callLater(() => { if (root.visible && root.kind === "sticker") createButton.forceActiveFocus(Qt.TabFocusReason) })
        return true
    }
    function chooseStickerImage() {
        if (sending || maker.preparing || !client || kind !== "sticker") return
        if (!maker.supported) { selected = { created: true }; maker.prepare(""); return }
        pickerProfile = targetProfile
        pickerChat = targetChat
        stickerFilePicker.open()
    }
    function send() {
        if (sending || !selected || !client) return
        sendError = ""
        sendingProfile = targetProfile
        sendingChat = targetChat
        sendingGeneration = viewGeneration
        if (kind === "gif") {
            sendingToken = catalog.holdPrepared()
            if (!sendingToken) return
            client.sendGif(sendingToken, targetProfile, targetChat, replyTo)
        } else if (creating) {
            sendingToken = maker.holdPrepared()
            if (!sendingToken) return
            client.sendCreatedSticker(sendingToken, targetProfile, targetChat, replyTo)
        } else {
            sendingToken = "sticker:" + selected.chat_jid + "/" + selected.id
            client.sendSticker(selected.chat_jid, selected.id, targetProfile, targetChat, replyTo)
        }
    }

    GifCatalog { id: catalog; settings: AppSettings }
    StickerMaker {
        id: maker
        objectName: "stickerMaker"
        onChanged: if (root.creating && !preparing) Qt.callLater(() => {
            if (root.visible && root.creating && !root.sending) backButton.forceActiveFocus(Qt.TabFocusReason)
        })
    }
    FileDialog {
        id: stickerFilePicker
        objectName: "stickerCreateFilePicker"
        title: qsTr("Choose an image for your sticker")
        fileMode: FileDialog.OpenFile
        nameFilters: [qsTr("Images (*.jpg *.jpeg *.png)")]
        onAccepted: {
            if (!root.visible || root.kind !== "sticker" || root.pickerProfile !== root.targetProfile
                    || root.pickerChat !== root.targetChat || !root.client
                    || root.client.profile !== root.targetProfile
                    || String(root.client.selectedChat.jid || "") !== root.targetChat) return
            root.sendError = ""
            root.selected = { created: true }
            maker.prepare(selectedFile.toString())
        }
    }
    Timer {
        id: debounce
        interval: 350
        onTriggered: { if (root.visible && root.kind === "gif") catalog.search(search.text) }
    }
    Connections {
        target: root.client
        function onExpressionSendFinished(token, success) {
            catalog.release(token)
            maker.release(token)
            if (root.sendingToken !== token) return
            root.sendingToken = ""
            if (root.targetProfile !== root.sendingProfile || root.targetChat !== root.sendingChat
                    || root.viewGeneration !== root.sendingGeneration) return
            if (success) root.sent()
            else root.sendError = qsTr("Not sent. Check the connection and try again.")
        }
    }
    onKindChanged: if (visible) { clear(); start() }
    onVisibleChanged: { if (visible) start(); else clear() }

    RowLayout {
        Layout.fillWidth: true
        visible: !root.selected
        TextField {
            id: search
            objectName: "gifSearchField"
            visible: root.kind === "gif"
            Layout.fillWidth: true
            implicitHeight: 44
            leftPadding: 14
            maximumLength: 100
            color: Theme.text
            placeholderText: qsTr("Search %1 GIFs").arg(catalog.providerName || "")
            Accessible.name: placeholderText
            enabled: catalog.configured && !root.sending
            onTextEdited: debounce.restart()
            onAccepted: { debounce.stop(); catalog.search(text) }
            background: Rectangle { radius: 22; color: Theme.surfaceMuted; border.width: 2; border.color: search.activeFocus ? Theme.primary : "transparent" }
        }
        ThemedToolButton {
            objectName: "clearGifSearchButton"
            visible: root.kind === "gif" && search.text.length > 0
            implicitWidth: 40; implicitHeight: 40
            iconSource: Qt.resolvedUrl("icons/close.svg")
            Accessible.name: qsTr("Clear GIF search")
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            enabled: !root.sending
            onClicked: {
                debounce.stop()
                search.clear()
                catalog.search("")
                search.forceActiveFocus()
            }
        }
        Repeater {
            model: root.kind === "sticker" ? [qsTr("Recent"), qsTr("Starred")] : []
            ExpressionButton {
                required property int index
                required property string modelData
                text: modelData
                checkable: true
                checked: root.favorites === (index === 1)
                enabled: !root.sending
                onClicked: { root.favorites = index === 1; root.client.loadStickers(root.favorites) }
            }
        }
        ExpressionButton {
            id: createButton
            objectName: "createStickerButton"
            visible: root.kind === "sticker"
            text: qsTr("Create")
            Accessible.name: qsTr("Create a sticker from an image")
            enabled: !root.sending && maker.supported
            onClicked: root.chooseStickerImage()
        }
        ThemedToolButton {
            visible: root.kind === "gif"
            implicitWidth: 40; implicitHeight: 40
            iconSource: Qt.resolvedUrl("icons/settings.svg")
            Accessible.name: qsTr("GIF provider settings")
            onClicked: root.settingsRequested()
        }
    }

    Label {
        Layout.fillWidth: true
        visible: !root.selected
        text: root.kind === "gif"
            ? (catalog.configured ? qsTr("Searches and your IP go to %1, not your chats. Select a GIF to preview.").arg(catalog.providerName)
                                  : qsTr("Add a GIPHY or KLIPY key in WhatsAppGo settings to search GIFs."))
            : (maker.supported
                ? qsTr("Create a sticker from a JPEG or PNG, or reuse one from this account’s history. Star a sticker message to keep it here.")
                : qsTr("Reuse stickers from this account’s history. Creating stickers needs WebP export support, which is unavailable in this build."))
        font.pixelSize: 12
        color: Theme.textMuted
        wrapMode: Text.Wrap
    }

    Label {
        objectName: "gifDiscoveryHeading"
        Layout.fillWidth: true
        visible: root.kind === "gif" && !root.selected && catalog.configured && search.text.trim().length === 0
        text: catalog.providerName === "KLIPY" ? qsTr("Featured GIFs") : qsTr("Trending GIFs")
        font.pixelSize: 13
        font.weight: Font.Medium
        color: Theme.text
    }

    RowLayout {
        visible: !!root.selected
        Layout.fillWidth: true
        ThemedToolButton {
            id: backButton
            objectName: "expressionBackButton"
            implicitWidth: 40; implicitHeight: 40
            iconSource: Qt.resolvedUrl("icons/back.svg")
            Accessible.name: qsTr("Back to results")
            enabled: !root.sending
            onClicked: root.back()
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("Send to %1").arg(root.targetTitle)
            textFormat: Text.PlainText
            font.weight: Font.Medium
            elide: Text.ElideRight
            color: Theme.text
        }
        ThemedToolButton {
            objectName: "stickerChooseAnotherButton"
            visible: root.creating
            implicitWidth: 40; implicitHeight: 40
            iconSource: Qt.resolvedUrl("icons/gallery.svg")
            Accessible.name: qsTr("Choose another sticker image")
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            enabled: !root.sending && !maker.preparing
            onClicked: root.chooseStickerImage()
        }
    }

    Label {
        Layout.fillWidth: true
        visible: root.creating
        text: maker.preparing ? qsTr("Preparing sticker locally…")
            : qsTr("Preview your sticker before sending. Transparency is preserved; the image is fitted without cropping.")
        color: Theme.textMuted
        font.pixelSize: 12
        wrapMode: Text.Wrap
    }

    Item {
        Layout.fillWidth: true
        Layout.fillHeight: true
        GridView {
            id: grid
            objectName: "expressionGrid"
            anchors.fill: parent
            visible: !root.selected
            clip: true
            cellWidth: width / Math.max(2, Math.floor(width / 145))
            cellHeight: 130
            boundsBehavior: Flickable.StopAtBounds
            model: root.kind === "gif" ? catalog.results : root.client ? root.client.stickers : []
            keyNavigationEnabled: true
            ScrollBar.vertical: OverlayScrollBar {}
            delegate: AbstractButton {
                id: tile
                required property var modelData
                required property int index
                width: grid.cellWidth - 6
                height: grid.cellHeight - 6
                enabled: !root.sending && (root.kind !== "gif" || !!modelData.mp4)
                Accessible.name: root.kind === "gif" ? (modelData.title || qsTr("GIF")) : qsTr("Sticker from %1").arg(modelData.chat_title || qsTr("a conversation"))
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
                onClicked: {
                    root.selected = modelData
                    root.sendError = ""
                    if (root.kind === "gif") catalog.prepare(index)
                }
                background: Rectangle {
                    radius: 8
                    color: tile.hovered || tile.activeFocus ? Theme.hoverRow : Theme.surfaceMuted
                    border.color: tile.activeFocus ? Theme.primary : "transparent"
                    border.width: 2
                }
                contentItem: Item {
                    Image {
                        anchors.fill: parent; anchors.margins: 5
                        source: root.kind === "gif" ? (tile.modelData.image || "")
                            : Theme.fileUrl(tile.modelData.media_path || tile.modelData.media_thumbnail || "")
                        sourceSize: Qt.size(240, 240)
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                    }
                    Label {
                        anchors.centerIn: parent
                        visible: root.kind === "gif" ? !tile.modelData.image : !(tile.modelData.media_path || tile.modelData.media_thumbnail)
                        text: root.kind === "gif" ? (tile.modelData.loading ? qsTr("Loading…") : qsTr("Preview unavailable")) : qsTr("Sticker")
                        font.pixelSize: 12; color: Theme.textMuted
                    }
                }
            }
        }
        Label {
            anchors.centerIn: parent
            width: parent.width - 32
            visible: !root.selected && grid.count === 0 && !(root.kind === "gif" ? catalog.busy : root.client && root.client.stickersLoading)
            text: root.kind === "gif" ? (catalog.configured
                ? (search.text.trim().length === 0 ? qsTr("No featured GIFs available. Try searching for one.") : qsTr("No GIFs found. Try a different word."))
                : qsTr("GIF search is not configured."))
                : (root.favorites ? qsTr("No starred stickers yet.") : qsTr("No stickers in this account’s local history yet."))
            color: Theme.textMuted
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
        }
        Item {
            anchors.fill: parent
            visible: !!root.selected
            Image {
                objectName: "createdStickerPreview"
                anchors.fill: parent
                source: root.creating ? maker.previewUrl : root.selected ? (root.kind === "gif" ? root.selected.image || "" : Theme.fileUrl(root.selected.media_path || root.selected.media_thumbnail || "")) : ""
                sourceSize: Qt.size(512, 512)
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                Accessible.role: Accessible.Graphic
                Accessible.name: root.creating ? qsTr("New sticker preview") : qsTr("Selected expression preview")
                visible: !playerLoader.item || !playerLoader.item.hasFrame
            }
            Loader {
                id: playerLoader
                anchors.fill: parent
                // QVideoSink initializes the multimedia backend too: defer
                // the entire playback surface until a preview is downloaded.
                active: root.visible && root.Window.active && !!root.selected && !root.previewPaused
                    && (root.kind === "gif" ? catalog.preparedUrl !== ""
                        : !!root.selected.sticker_animated && !!root.selected.sticker_source)
                sourceComponent: InlineAnimation {
                    source: root.kind === "gif" ? catalog.preparedUrl : Theme.fileUrl(root.selected.sticker_source)
                    sticker: root.kind === "sticker"
                    onFailed: message => { root.sendError = message; root.previewPaused = true }
                }
            }
            ExpressionButton {
                objectName: "expressionPreviewToggle"
                anchors.right: parent.right; anchors.bottom: parent.bottom; anchors.margins: 8
                visible: !!root.selected && (root.kind === "gif" ? catalog.preparedUrl !== "" : !!root.selected.sticker_animated)
                text: root.previewPaused ? qsTr("Play") : qsTr("Pause")
                Accessible.name: root.previewPaused ? qsTr("Play preview") : qsTr("Pause preview")
                onClicked: root.previewPaused = !root.previewPaused
            }
        }
        BusyIndicator {
            anchors.centerIn: parent
            running: root.creating ? maker.preparing : root.selected ? catalog.preparing : root.kind === "gif" ? catalog.busy : root.client && root.client.stickersLoading
            visible: running
        }
    }

    Label {
        Layout.fillWidth: true
        text: root.sendError || (root.creating ? maker.error : root.kind === "gif" ? catalog.error : root.client ? root.client.stickersError : "")
        visible: text !== ""
        color: Theme.danger
        textFormat: Text.PlainText
        wrapMode: Text.Wrap
    }
    RowLayout {
        Layout.fillWidth: true
        ExpressionButton {
            visible: !root.selected
            text: qsTr("Retry")
            enabled: !(root.kind === "gif" ? catalog.busy : root.client && root.client.stickersLoading)
            onClicked: root.kind === "gif" ? catalog.search(search.text) : root.client.loadStickers(root.favorites)
        }
        ExpressionButton {
            visible: !root.selected && (root.kind === "gif" ? catalog.hasMore : root.client && root.client.stickersHasMore)
            text: qsTr("Load more")
            enabled: !(root.kind === "gif" ? catalog.busy : root.client && root.client.stickersLoading)
            onClicked: root.kind === "gif" ? catalog.search(search.text, true) : root.client.loadStickers(root.favorites, true)
        }
        Item { Layout.fillWidth: true }
        Label {
            visible: root.kind === "gif" && catalog.configured && catalog.providerName !== "GIPHY"
            text: "Powered by " + catalog.providerName
            font.pixelSize: 12; font.bold: true; color: Theme.textMuted
        }
        Image {
            visible: root.kind === "gif" && catalog.configured && catalog.providerName === "GIPHY"
            Layout.preferredWidth: 150
            Layout.preferredHeight: 24
            source: Qt.resolvedUrl(Theme.dark ? "icons/PoweredBy_200px-Black_HorizLogo.png" : "icons/PoweredBy_200px-White_HorizLogo.png")
            fillMode: Image.PreserveAspectFit
            Accessible.name: "Powered by GIPHY"
        }
        ExpressionButton {
            objectName: "expressionSendButton"
            primary: true
            visible: !!root.selected
            text: root.sending ? qsTr("Sending…") : qsTr("Send")
            enabled: !root.sending && !(root.client && root.client.busy) && (root.kind !== "gif" || !!catalog.preparedUrl)
                && (!root.creating || (!maker.preparing && !!maker.preparedUrl))
            onClicked: root.send()
        }
    }
}
