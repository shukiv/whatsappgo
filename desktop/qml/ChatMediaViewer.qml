import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import QtCore
import org.whatsappgo

FocusScope {
    id: root
    objectName: "chatMediaViewer"

    property url imageUrl: ""
    property string messageId: ""
    property string contactTitle: ""
    property url contactAvatarSource
    property string sentAt: ""
    property string caption: ""
    property real zoomFactor: 1.0
    property real panX: 0
    property real panY: 0
    property var gallery: []
    property int galleryIndex: -1
    property var sourceMessage: ({})
    property string ownerChat: ""
    property string ownerProfile: ""
    property int messageRevision: 0
    readonly property bool previewActive: String(imageUrl).length > 0 || galleryIndex >= 0
    readonly property bool ownerCurrent: ownerChat !== "" && ownerChat === String(backend.selectedChat.jid || "")
        && ownerProfile === String(backend.profile)
    readonly property var currentMessage: {
        const revision = messageRevision
        const loaded = ownerCurrent && messageId ? backend.messageById(messageId) : ({})
        return loaded.id ? loaded : sourceMessage
    }
    readonly property bool messageEligible: ownerCurrent && Boolean(currentMessage.id)
        && currentMessage.kind === "image" && !currentMessage.revoked
    readonly property bool downloadBusy: (backend.documentDownloads || []).indexOf(
        ownerChat + "/" + messageId) >= 0
    onCurrentMessageChanged: Qt.callLater(validateCurrentPhoto)
    readonly property bool inputReady: previewActive && !Theme.popupOwnsFocus(root.Overlay.overlay,
        root.Window.window ? root.Window.window.activeFocusItem : null)
    signal messageActionRequested(string action, var message)
    signal sessionEnded()
    readonly property real minimumZoom: 1.0
    readonly property real maximumZoom: 5.0
    readonly property real zoomRatio: 1.2

    visible: previewActive
    focus: previewActive
    Accessible.role: Accessible.Pane
    Accessible.name: qsTr("Photo viewer")

    function openImage(url, title, avatarSource, timestampText, imageCaption, sourceMessageId) {
        gallery = []
        galleryIndex = -1
        sourceMessage = ({})
        ownerChat = ""
        ownerProfile = ""
        imageUrl = url
        messageId = sourceMessageId || ""
        contactTitle = title || qsTr("Photo")
        contactAvatarSource = avatarSource || ""
        sentAt = timestampText || ""
        caption = imageCaption || ""
        zoomFactor = minimumZoom
        panX = 0
        panY = 0
        forceActiveFocus()
    }

    function closePreview() {
        imageActionMenu.close()
        photoReactionPicker.close()
        photoPinDialog.close()
        saveImageDialog.close()
        gallery = []
        galleryIndex = -1
        sourceMessage = ({})
        ownerChat = ""
        ownerProfile = ""
        imageUrl = ""
        messageId = ""
        zoomFactor = minimumZoom
        panX = 0
        panY = 0
        sessionEnded()
    }

    function openGallery(items, index, chat, profile) {
        if (!items.length || index < 0 || index >= items.length) return
        gallery = items
        ownerChat = chat
        ownerProfile = profile
        selectPhoto(index)
    }

    function selectPhoto(index) {
        if (!ownerCurrent || index < 0 || index >= gallery.length) return
        imageActionMenu.close()
        photoReactionPicker.close()
        const item = gallery[index]
        sourceMessage = item.message
        messageId = String(item.message.id)
        imageUrl = item.url || ""
        contactTitle = item.title
        contactAvatarSource = item.avatar || ""
        sentAt = item.sentAt
        caption = String(item.message.body || "")
        galleryIndex = index
        zoomFactor = minimumZoom
        panX = 0
        panY = 0
        messageRevision++
        thumbnails.positionViewAtIndex(index, ListView.Contain)
        if (!item.message.media_path) backend.ensureMedia(messageId)
        forceActiveFocus()
    }

    function requestMessageAction(action) {
        if (!messageEligible) return
        if (action === "download") backend.downloadPhoto(currentMessage)
        else if (action === "star") backend.starMessage(messageId, currentMessage.sender_jid || "",
                                                       Boolean(currentMessage.from_me), !currentMessage.starred)
        else if (action === "pin") { photoPinSevenDays.checked = true; photoPinDialog.open() }
        else if (action === "react") photoReactionPicker.open()
        else messageActionRequested(action, currentMessage)
    }

    function panBy(dx, dy) {
        if (zoomFactor <= minimumZoom) return
        panX += dx
        panY += dy
        clampPan()
    }

    function validateCurrentPhoto() {
        if (galleryIndex >= 0 && currentMessage.id
                && (currentMessage.revoked || currentMessage.kind !== "image")) closePreview()
    }

    Connections {
        target: backend
        function onSelectedChatChanged() {
            if (root.galleryIndex >= 0 && root.ownerChat !== String(backend.selectedChat.jid || ""))
                root.closePreview()
        }
        function onProfileChanged() { if (root.previewActive) root.closePreview() }
    }
    Connections {
        target: backend.messages
        function onDataChanged() { root.messageRevision++ }
        function onModelReset() { root.messageRevision++ }
        function onRowsRemoved() { root.messageRevision++ }
    }

    Shortcut {
        sequence: "Left"
        enabled: root.inputReady && root.galleryIndex > 0
        onActivated: root.selectPhoto(root.galleryIndex - 1)
    }
    Shortcut {
        sequence: "Right"
        enabled: root.inputReady && root.galleryIndex >= 0 && root.galleryIndex + 1 < root.gallery.length
        onActivated: root.selectPhoto(root.galleryIndex + 1)
    }
    Shortcut { sequence: "Shift+Left"; enabled: root.inputReady; onActivated: root.panBy(60, 0) }
    Shortcut { sequence: "Shift+Right"; enabled: root.inputReady; onActivated: root.panBy(-60, 0) }
    Shortcut { sequence: "Shift+Up"; enabled: root.inputReady; onActivated: root.panBy(0, 60) }
    Shortcut { sequence: "Shift+Down"; enabled: root.inputReady; onActivated: root.panBy(0, -60) }

    function replaceImage(url) {
        if (!url || String(imageUrl) === String(url))
            return
        imageUrl = url
        zoomFactor = minimumZoom
        panX = 0
        panY = 0
    }

    // The extent the photo actually covers at the current zoom. A photo whose
    // aspect differs from the stage is letterboxed, so the stage box is wider or
    // taller than the picture inside it.
    //
    // This repeats what PreserveAspectFit does rather than reading paintedWidth,
    // because the painted size depends on the surface geometry, which the pan
    // offset moves - reading it here would close a binding loop through
    // clampPan. sourceSize is the decoded size and does not move.
    readonly property size drawnSize: {
        const sourceWidth = fullImage.sourceSize.width
        const sourceHeight = fullImage.sourceSize.height
        if (sourceWidth <= 0 || sourceHeight <= 0 || stage.width <= 0 || stage.height <= 0)
            return Qt.size(stage.width * zoomFactor, stage.height * zoomFactor)
        const fit = Math.min(stage.width / sourceWidth, stage.height / sourceHeight)
        return Qt.size(sourceWidth * fit * zoomFactor, sourceHeight * fit * zoomFactor)
    }

    function clampPan() {
        // Pan is bounded by the photo, not by the stage around it. Measuring the
        // stage would let a drag pull the picture off its own edge and leave the
        // reader looking at the empty surface beside it.
        const maxX = Math.max(0, (drawnSize.width - stage.width) / 2)
        const maxY = Math.max(0, (drawnSize.height - stage.height) / 2)
        panX = Math.max(-maxX, Math.min(maxX, panX))
        panY = Math.max(-maxY, Math.min(maxY, panY))
    }

    function setZoomAt(value, pointerX, pointerY) {
        const oldZoom = zoomFactor
        const nextZoom = Math.max(minimumZoom, Math.min(maximumZoom, value))
        if (Math.abs(nextZoom - oldZoom) < 0.0001)
            return
        if (nextZoom <= minimumZoom) {
            zoomFactor = minimumZoom
            panX = 0
            panY = 0
            return
        }
        const centerX = stage.width / 2
        const centerY = stage.height / 2
        const ratio = nextZoom / oldZoom
        panX = pointerX - centerX - (pointerX - centerX - panX) * ratio
        panY = pointerY - centerY - (pointerY - centerY - panY) * ratio
        zoomFactor = nextZoom
        clampPan()
    }

    function zoomIn() {
        setZoomAt(zoomFactor * zoomRatio, stage.width / 2, stage.height / 2)
    }

    function zoomOut() {
        setZoomAt(zoomFactor / zoomRatio, stage.width / 2, stage.height / 2)
    }

    function adjustZoomFromWheel(delta, pointerX, pointerY) {
        if (delta === 0)
            return
        // Multiplicative zoom respects both a mouse-wheel notch and the smaller,
        // continuous deltas emitted by a touchpad.
        const factor = Math.exp(delta * 0.0015)
        setZoomAt(zoomFactor * factor,
                  pointerX === undefined ? stage.width / 2 : pointerX,
                  pointerY === undefined ? stage.height / 2 : pointerY)
    }

    function imageFileName() {
        const source = String(imageUrl || "").split("?")[0]
        const candidate = decodeURIComponent(source.substring(source.lastIndexOf("/") + 1))
        return candidate || "WhatsApp image.jpg"
    }

    function openActionMenu(pointerX, pointerY) {
        if (!previewActive)
            return
        const overlay = Overlay.overlay
        const mapped = stage.mapToItem(overlay, pointerX, pointerY)
        imageActionMenu.x = Math.max(8, Math.min(overlay.width - imageActionMenu.width - 8, mapped.x))
        imageActionMenu.y = Math.max(8, Math.min(overlay.height - imageActionMenu.implicitHeight - 8, mapped.y))
        imageActionMenu.open()
    }

    function chooseSaveDestination() {
        const pictures = StandardPaths.writableLocation(StandardPaths.PicturesLocation)
        saveImageDialog.currentFile = String(pictures) + "/" + imageFileName()
        saveImageDialog.open()
    }

    Shortcut {
        sequences: [StandardKey.Cancel]
        enabled: root.previewActive && !Theme.popupOwnsFocus(root.Overlay.overlay,
            root.Window.window ? root.Window.window.activeFocusItem : null)
        onActivated: root.closePreview()
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.surface
    }

    // A full-window input shield keeps clicks from activating the chat behind
    // the viewer. Controls declared later remain above it.
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.AllButtons
    }

    Rectangle {
        id: toolbar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 64
        color: Theme.surface

        RowLayout {
            anchors.left: parent.left
            anchors.leftMargin: 20
            anchors.right: toolbarActions.left
            anchors.rightMargin: 12
            anchors.verticalCenter: parent.verticalCenter
            spacing: 10

            Avatar {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                diameter: 44
                title: root.contactTitle
                source: root.contactAvatarSource
                fallbackIdentity: source.toString() === ""
                Accessible.ignored: true
            }

            ColumnLayout {
                Layout.fillWidth: true
                Layout.minimumWidth: 0
                spacing: 1

                Label {
                    Layout.fillWidth: true
                    text: root.contactTitle
                    textFormat: Text.PlainText
                    color: Theme.text
                    font.pixelSize: 15
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }

                Label {
                    Layout.fillWidth: true
                    visible: text.length > 0
                    text: root.sentAt
                    color: Theme.textMuted
                    font.pixelSize: 12
                    elide: Text.ElideRight
                }
            }
        }

        RowLayout {
            id: toolbarActions
            anchors.right: closeButton.left
            anchors.rightMargin: 4
            anchors.verticalCenter: parent.verticalCenter
            spacing: 4

            ThemedToolButton {
                id: zoomOutButton
                objectName: "chatMediaViewerZoomOut"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                enabled: root.zoomFactor > root.minimumZoom
                Accessible.name: qsTr("Zoom out")
                onClicked: root.zoomOut()
                contentItem: Label {
                    text: "−"
                    color: parent.enabled ? Theme.icon : Theme.textMuted
                    font.pixelSize: 25
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }

            Label {
                Layout.preferredWidth: 54
                text: Math.round(root.zoomFactor * 100) + "%"
                color: Theme.textMuted
                font.pixelSize: 12
                horizontalAlignment: Text.AlignHCenter
            }

            ThemedToolButton {
                id: zoomInButton
                objectName: "chatMediaViewerZoomIn"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                enabled: root.zoomFactor < root.maximumZoom
                Accessible.name: qsTr("Zoom in")
                onClicked: root.zoomIn()
                contentItem: Label {
                    text: "+"
                    color: parent.enabled ? Theme.icon : Theme.textMuted
                    font.pixelSize: 23
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }

            Repeater {
                model: [
                    { action: "jump", title: qsTr("Go to message"), icon: "chats" },
                    { action: "reply", title: qsTr("Reply"), icon: "reply" },
                    { action: "star", title: root.currentMessage.starred ? qsTr("Unstar") : qsTr("Star"), icon: "star" },
                    { action: "pin", title: qsTr("Pin"), icon: "pin" },
                    { action: "react", title: qsTr("React"), icon: "smile" },
                    { action: "forward", title: qsTr("Forward"), icon: "forward" },
                    { action: "download", title: qsTr("Download"), icon: "download" }
                ]
                ThemedToolButton {
                    required property var modelData
                    objectName: "photoAction_" + modelData.action
                    visible: root.galleryIndex >= 0 && toolbar.width >= 1000
                    enabled: root.messageEligible && (modelData.action !== "download" || !root.downloadBusy)
                    Layout.preferredWidth: 36
                    Layout.preferredHeight: 40
                    iconSource: Qt.resolvedUrl("icons/" + modelData.icon + ".svg")
                    iconSize: 20
                    Accessible.name: modelData.title
                    iconSpinning: modelData.action === "download" && root.downloadBusy
                    onClicked: root.requestMessageAction(modelData.action)
                    ToolTip.visible: root.inputReady && (hovered || activeFocus)
                    ToolTip.text: Accessible.name
                }
            }
            ThemedToolButton {
                objectName: "photoActionsMenuButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/menu.svg")
                Accessible.name: qsTr("Photo actions")
                onClicked: imageActionMenu.openUnder(this)
                ToolTip.visible: !imageActionMenu.visible && root.inputReady && (hovered || activeFocus)
                ToolTip.text: Accessible.name
            }
        }

        ThemedToolButton {
            id: closeButton
            objectName: "chatMediaViewerCloseButton"
            anchors.right: parent.right
            anchors.rightMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            iconSource: Qt.resolvedUrl("icons/close.svg")
            iconSize: 21
            Accessible.name: qsTr("Close photo viewer")
            onClicked: root.closePreview()
            background: Rectangle {
                radius: 22
                color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
            }
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 1
            color: Theme.border
        }
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: toolbar.bottom
        anchors.bottom: filmstrip.top
        anchors.margins: 20
        clip: true
        onWidthChanged: root.clampPan()
        onHeightChanged: root.clampPan()

        Item {
            id: zoomSurface
            objectName: "chatMediaViewerZoomSurface"
            anchors.fill: parent
            anchors.leftMargin: root.panX
            anchors.rightMargin: -root.panX
            anchors.topMargin: root.panY
            anchors.bottomMargin: -root.panY
            scale: root.zoomFactor
            transformOrigin: Item.Center
            // Do not cache this transformed subtree. With Qt's software scene
            // graph a layer can retain only the previously exposed rectangle,
            // leaving a cropped photo surrounded by black after the viewer is
            // opened or resized. Image already renders its texture efficiently.
            readonly property bool renderCached: false

            Image {
                id: fullImage
                objectName: "chatMediaViewerImage"
                anchors.fill: parent
                source: root.imageUrl
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                cache: true
                smooth: true
                mipmap: true
                // A photo that has just decoded, or one replaced by the next in
                // the gallery, changes the bounds a pan is allowed to reach.
                onSourceSizeChanged: root.clampPan()
                Accessible.name: root.caption || qsTr("Shared photo")
            }
        }

        WheelHandler {
            objectName: "chatMediaViewerZoomWheel"
            target: null
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.pixelDelta.y
                root.adjustZoomFromWheel(delta, event.x, event.y)
                event.accepted = true
            }
        }

        MouseArea {
            objectName: "chatMediaViewerClickHandler"
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            cursorShape: root.zoomFactor > root.minimumZoom ? (pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor) : Qt.ArrowCursor
            property point pressPoint
            property point initialPan
            property bool panned: false
            onPressed: mouse => {
                pressPoint = Qt.point(mouse.x, mouse.y)
                initialPan = Qt.point(root.panX, root.panY)
                panned = false
            }
            onPositionChanged: mouse => {
                if (!(pressedButtons & Qt.LeftButton) || root.zoomFactor <= root.minimumZoom) return
                const dx = mouse.x - pressPoint.x
                const dy = mouse.y - pressPoint.y
                if (!panned && Math.abs(dx) + Math.abs(dy) < 8) return
                panned = true
                root.panX = initialPan.x + dx
                root.panY = initialPan.y + dy
                root.clampPan()
            }
            onClicked: mouse => { if (!panned) root.openActionMenu(mouse.x, mouse.y) }
        }

        ThemedToolButton {
            objectName: "photoPrevious"
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44; height: 44
            visible: root.gallery.length > 1
            enabled: root.galleryIndex > 0
            iconSource: Qt.resolvedUrl("icons/back.svg")
            Accessible.name: qsTr("Previous photo")
            onClicked: root.selectPhoto(root.galleryIndex - 1)
            ToolTip.visible: hovered || activeFocus
            ToolTip.text: Accessible.name
        }
        ThemedToolButton {
            objectName: "photoNext"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: 44; height: 44
            visible: root.gallery.length > 1
            enabled: root.galleryIndex + 1 < root.gallery.length
            iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
            Accessible.name: qsTr("Next photo")
            onClicked: root.selectPhoto(root.galleryIndex + 1)
            ToolTip.visible: hovered || activeFocus
            ToolTip.text: Accessible.name
        }

        BusyIndicator {
            anchors.centerIn: parent
            width: 44
            height: 44
            running: fullImage.status === Image.Loading
            visible: running
            Accessible.name: qsTr("Loading photo")
        }

        Label {
            anchors.centerIn: parent
            visible: fullImage.status === Image.Error || String(root.imageUrl) === ""
            text: qsTr("This photo is not available yet. Use Download to fetch it.")
            color: Theme.textMuted
            font.pixelSize: 15
        }
    }

    Rectangle {
        id: filmstrip
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: root.galleryIndex >= 0 ? 116 : 92
        color: Theme.surface

        Rectangle {
            visible: root.galleryIndex < 0
            anchors.centerIn: parent
            width: 64
            height: 64
            radius: 6
            color: "transparent"
            border.width: 3
            border.color: Theme.primary

            Image {
                anchors.fill: parent
                anchors.margins: 4
                source: root.imageUrl
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: false
            }
        }

        Label {
            anchors.left: parent.left
            anchors.leftMargin: 20
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, (parent.width - 180) / 2)
            text: root.caption
            visible: root.galleryIndex < 0 && text.length > 0
            color: Theme.textMuted
            font.pixelSize: 13
            elide: Text.ElideRight
        }

        Label {
            id: galleryCaption
            objectName: "photoGalleryCaption"
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: 12
            visible: root.galleryIndex >= 0
            text: (root.galleryIndex + 1) + " / " + root.gallery.length + (root.caption ? " · " + root.caption : "")
            textFormat: Text.PlainText
            color: Theme.textMuted
            elide: Text.ElideRight
            font.pixelSize: 13
        }
        ListView {
            id: thumbnails
            objectName: "photoThumbnails"
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: galleryCaption.bottom
            anchors.bottom: parent.bottom
            anchors.margins: 8
            visible: root.galleryIndex >= 0
            orientation: ListView.Horizontal
            spacing: 8
            clip: true
            model: root.gallery
            currentIndex: root.galleryIndex
            delegate: ThemedToolButton {
                required property var modelData
                required property int index
                objectName: "photoThumbnail_" + index
                width: 68; height: 68
                padding: 4
                Accessible.name: qsTr("Photo %1 of %2, %3").arg(index + 1).arg(root.gallery.length).arg(modelData.title)
                Accessible.description: modelData.message.body || ""
                Accessible.selected: index === root.galleryIndex
                onClicked: root.selectPhoto(index)
                background: Rectangle {
                    radius: 6
                    color: Theme.surfaceMuted
                    border.width: 3
                    border.color: parent.activeFocus || parent.index === root.galleryIndex ? Theme.primary : "transparent"
                }
                contentItem: Image {
                    source: parent.index === root.galleryIndex ? root.imageUrl : parent.modelData.url
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                }
                ToolTip.visible: hovered || activeFocus
                ToolTip.text: Accessible.name
            }
            ScrollBar.horizontal: ScrollBar {}
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 1
            color: Theme.border
        }
    }

    WhatsAppMenuPopup {
        id: imageActionMenu
        objectName: "chatMediaViewerActionMenu"
        parent: Overlay.overlay
        width: 230

        Repeater {
            model: [
                { action: "jump", title: qsTr("Go to message") }, { action: "reply", title: qsTr("Reply") },
                { action: "star", title: root.currentMessage.starred ? qsTr("Unstar") : qsTr("Star") },
                { action: "pin", title: qsTr("Pin") }, { action: "react", title: qsTr("React") },
                { action: "forward", title: qsTr("Forward") }, { action: "download", title: qsTr("Download") }
            ]
            WhatsAppMenuItem {
                required property var modelData
                objectName: "photoMenu_" + modelData.action
                text: modelData.title
                visible: root.galleryIndex >= 0
                enabled: root.messageEligible && (modelData.action !== "download" || !root.downloadBusy)
                onClicked: { imageActionMenu.close(); root.requestMessageAction(modelData.action) }
            }
        }

        WhatsAppMenuItem {
            objectName: "chatMediaViewerCopyAction"
            text: qsTr("Copy image")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: {
                imageActionMenu.close()
                backend.copyImage(root.messageId, String(root.imageUrl))
            }
        }

        WhatsAppMenuItem {
            objectName: "chatMediaViewerSaveAction"
            text: qsTr("Save image as…")
            iconSource: Qt.resolvedUrl("icons/document.svg")
            onClicked: {
                imageActionMenu.close()
                root.chooseSaveDestination()
            }
        }
    }

    EmojiPicker {
        id: photoReactionPicker
        objectName: "photoReactionPicker"
        parent: Overlay.overlay
        x: Math.max(8, Math.min(parent.width - width - 8, (parent.width - width) / 2))
        y: Math.max(8, (parent.height - height) / 2)
        onEmojiChosen: emoji => {
            if (!root.messageEligible) return
            const self = String(backend.status.user_jid || "").split("@")[0].split(":")[0]
            const own = (root.currentMessage.reactions || []).find(r => String(r.sender_jid || "").split("@")[0].split(":")[0] === self)
            backend.reactMessage(root.messageId, root.currentMessage.sender_jid || "", own && own.emoji === emoji ? "" : emoji)
            close()
        }
        onClosed: if (root.previewActive) root.forceActiveFocus()
    }

    WhatsAppDialog {
        id: photoPinDialog
        objectName: "photoPinDialog"
        title: qsTr("Choose how long to pin this message")
        subtitle: qsTr("You can unpin it at any time.")
        acceptText: qsTr("Pin")
        acceptEnabled: root.messageEligible
        onAccepted: {
            if (root.messageEligible)
                backend.pinMessage(root.messageId, root.currentMessage.sender_jid || "",
                    (photoPinOneDay.checked ? 1 : photoPinThirtyDays.checked ? 30 : 7) * 86400)
        }
        onClosed: if (root.previewActive) root.forceActiveFocus()
        ButtonGroup { id: photoPinDurations }
        DialogRadioButton { id: photoPinOneDay; text: qsTr("24 hours"); ButtonGroup.group: photoPinDurations }
        DialogRadioButton { id: photoPinSevenDays; text: qsTr("7 days"); checked: true; ButtonGroup.group: photoPinDurations }
        DialogRadioButton { id: photoPinThirtyDays; text: qsTr("30 days"); ButtonGroup.group: photoPinDurations }
    }

    FileDialog {
        id: saveImageDialog
        objectName: "chatMediaViewerSaveDialog"
        title: qsTr("Save image as")
        fileMode: FileDialog.SaveFile
        nameFilters: [
            qsTr("Images (*.jpg *.jpeg *.png *.gif *.webp *.bmp)"),
            qsTr("All files (*)")
        ]
        onAccepted: backend.saveImage(String(root.imageUrl), String(selectedFile))
    }
}
