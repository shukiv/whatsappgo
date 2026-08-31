import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: root
    required property var modelData
    signal editRequested(string messageId, string body)
    signal deleteRequested(string messageId, string senderJid)
    signal replyRequested(string messageId, string body)

    readonly property bool hasReply: Boolean(modelData.reply_to)
    readonly property bool hasMedia: Boolean(modelData.media_path)
    readonly property bool mediaKind: ["image", "video", "audio", "document", "sticker"].indexOf(modelData.kind) >= 0
    readonly property bool visualKind: ["image", "video", "sticker"].indexOf(modelData.kind) >= 0

    // The local file when it has been downloaded, otherwise the inline
    // thumbnail WhatsApp delivers with the message itself. Showing the
    // thumbnail means a photo or video is visible before it is fetched.
    readonly property string previewPath: {
        if ((modelData.kind === "image" || modelData.kind === "sticker") && modelData.media_path)
            return String(modelData.media_path)
        return String(modelData.media_thumbnail || "")
    }
    readonly property bool hasPreview: visualKind && previewPath !== ""
    readonly property bool previewIsThumbnail: hasPreview && previewPath !== String(modelData.media_path || "")

    // Every width limit inside the bubble derives from the conversation width.
    // Nothing may size itself from the bubble, because the bubble sizes itself
    // from its content and the two together would form a binding loop.
    readonly property real horizontalPadding: 10
    readonly property real maxBubbleWidth: Math.max(180, Math.min(root.width * 0.68, 620))
    readonly property real contentMaxWidth: maxBubbleWidth - 2 * horizontalPadding
    readonly property real mediaWidth: Math.min(contentMaxWidth, modelData.kind === "sticker" ? 180 : 320)

    readonly property string mediaLabel: {
        switch (modelData.kind) {
        case "image": return qsTr("Photo")
        case "video": return qsTr("Video")
        case "audio": return qsTr("Voice message")
        case "document": return modelData.media_name || qsTr("Document")
        case "sticker": return qsTr("Sticker")
        case "contact": return qsTr("Contact")
        case "location": return qsTr("Location")
        case "poll": return qsTr("Poll")
        default: return ""
        }
    }

    width: ListView.view ? ListView.view.width : 640
    implicitHeight: bubble.implicitHeight + 5

    Rectangle {
        id: bubble
        objectName: "messageBubble"
        implicitWidth: Math.min(Math.max(messageContent.implicitWidth + 2 * root.horizontalPadding, 104), root.maxBubbleWidth)
        width: implicitWidth
        implicitHeight: messageContent.implicitHeight + 13
        anchors.right: root.modelData.from_me ? parent.right : undefined
        anchors.left: root.modelData.from_me ? undefined : parent.left
        anchors.rightMargin: root.modelData.from_me ? 40 : 0
        anchors.leftMargin: root.modelData.from_me ? 0 : 40
        color: root.modelData.from_me ? Theme.outgoingBubble : Theme.incomingBubble
        radius: Theme.radiusLarge
        border.color: Theme.bubbleBorder
        border.width: root.modelData.from_me ? 0 : 1

        Canvas {
            id: messageTail
            objectName: "messageTail"
            width: 10
            height: 13
            anchors.top: parent.top
            anchors.topMargin: 0
            anchors.right: root.modelData.from_me ? parent.right : undefined
            anchors.left: root.modelData.from_me ? undefined : parent.left
            anchors.rightMargin: root.modelData.from_me ? -8 : 0
            anchors.leftMargin: root.modelData.from_me ? 0 : -8
            property color fillColor: bubble.color
            visible: root.modelData.kind !== "system"

            renderStrategy: Canvas.Immediate
            antialiasing: true
            onFillColorChanged: requestPaint()
            onPaint: {
                const ctx = getContext("2d")
                ctx.reset()
                ctx.clearRect(0, 0, width, height)
                ctx.fillStyle = fillColor
                ctx.beginPath()
                if (root.modelData.from_me) {
                    ctx.moveTo(0, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(0, height)
                } else {
                    ctx.moveTo(0, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(width, height)
                }
                ctx.closePath()
                ctx.fill()
            }
        }

        ColumnLayout {
            id: messageContent
            x: root.horizontalPadding
            y: 7
            width: bubble.width - 2 * root.horizontalPadding
            spacing: 4

            Label {
                visible: !root.modelData.from_me && Boolean(root.modelData.sender_name)
                Layout.maximumWidth: root.contentMaxWidth
                text: root.modelData.sender_name || ""
                color: Theme.primary
                font.pixelSize: 12
                font.weight: Font.DemiBold
                elide: Text.ElideRight
                maximumLineCount: 1
            }

            Rectangle {
                visible: root.hasReply
                Layout.fillWidth: true
                Layout.minimumWidth: 96
                Layout.maximumWidth: root.contentMaxWidth
                implicitHeight: replyPreviewColumn.implicitHeight + 10
                color: Theme.replyBackground
                radius: 5
                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    width: 4
                    radius: 2
                    color: Theme.primary
                }
                Column {
                    id: replyPreviewColumn
                    anchors.left: parent.left
                    anchors.leftMargin: 11
                    anchors.right: parent.right
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Label {
                        width: parent.width
                        text: root.modelData.reply_from_me ? qsTr("You") : (root.modelData.reply_sender || qsTr("Message"))
                        color: Theme.primary
                        font.pixelSize: 11
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Label {
                        id: replyLabel
                        width: parent.width
                        text: Theme.emojiRichText(root.modelData.reply_preview || qsTr("Replied message"))
                        color: Theme.textMuted
                        font.pixelSize: 12
                        elide: Text.ElideRight
                        textFormat: Text.RichText
                    }
                }
            }

            Item {
                id: mediaFrame
                // A cached preview file can be missing or unreadable. Falling
                // back to the descriptive row keeps the message usable instead
                // of leaving an empty frame in the conversation.
                readonly property bool previewReady: root.hasPreview && mediaImage.status !== Image.Error
                visible: previewReady
                Layout.preferredWidth: root.mediaWidth
                Layout.preferredHeight: visible ? mediaImage.displayHeight : 0

                Image {
                    id: mediaImage
                    objectName: "messageMedia"
                    anchors.fill: parent
                    source: root.hasPreview ? "file://" + root.previewPath : ""
                    // The thumbnail is a small image that WhatsApp embeds in the
                    // message. Cropping keeps its frame proportional instead of
                    // letting a low-resolution picture dictate the bubble shape.
                    fillMode: root.modelData.kind === "sticker" ? Image.PreserveAspectFit : Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    smooth: true
                    mipmap: root.previewIsThumbnail
                    layer.enabled: root.modelData.kind !== "sticker"
                    layer.effect: null
                    readonly property real displayHeight: {
                        if (implicitWidth <= 0 || implicitHeight <= 0)
                            return 180
                        const natural = root.mediaWidth * (implicitHeight / implicitWidth)
                        return Math.max(120, Math.min(root.modelData.kind === "sticker" ? 180 : 340, natural))
                    }
                    Accessible.name: root.modelData.body || root.mediaLabel
                }

                Rectangle {
                    objectName: "mediaPlayBadge"
                    visible: root.modelData.kind === "video"
                    anchors.centerIn: parent
                    width: 52
                    height: 52
                    radius: 26
                    color: "#99000000"
                    Label {
                        anchors.centerIn: parent
                        text: "▶"
                        color: "#FFFFFF"
                        font.pixelSize: 22
                        Accessible.ignored: true
                    }
                }

                // A thumbnail is a preview only. The full file still has to be
                // fetched, so the action stays available on top of it.
                Button {
                    objectName: "mediaOverlayAction"
                    visible: root.visualKind && !root.hasMedia
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: 8
                    height: 30
                    padding: 10
                    text: qsTr("Download")
                    font.pixelSize: 12
                    Accessible.name: qsTr("Download %1").arg(root.mediaLabel)
                    contentItem: Label {
                        text: parent.text
                        color: "#FFFFFF"
                        font: parent.font
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle {
                        radius: height / 2
                        color: parent.down ? "#CC000000" : "#99000000"
                    }
                    onClicked: backend.downloadMedia(root.modelData.id)
                }

                MouseArea {
                    anchors.fill: parent
                    enabled: root.hasMedia
                    cursorShape: Qt.PointingHandCursor
                    onClicked: backend.openFile(root.modelData.media_path)
                }
            }

            RowLayout {
                // Files that have no picture of their own keep the descriptive
                // row; a photo or video only falls back to it without a preview.
                visible: (root.mediaKind && !mediaFrame.previewReady) || ["contact", "location", "poll"].indexOf(root.modelData.kind) >= 0
                Layout.fillWidth: true
                Layout.minimumWidth: 120
                Layout.maximumWidth: root.contentMaxWidth
                spacing: 8
                Rectangle {
                    Layout.preferredWidth: 36
                    Layout.preferredHeight: 36
                    radius: 18
                    color: Theme.surfaceMuted
                    Label {
                        anchors.centerIn: parent
                        text: root.modelData.kind === "audio" ? "▶" : root.modelData.kind === "document" ? "↓" : "•"
                        color: Theme.icon
                        font.pixelSize: 15
                        Accessible.ignored: true
                    }
                }
                Label {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    text: root.mediaLabel
                    color: Theme.text
                    font.pixelSize: 14
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }
                ToolButton {
                    objectName: "mediaAction"
                    visible: root.mediaKind
                    text: root.hasMedia ? qsTr("Open") : qsTr("Download")
                    flat: true
                    Accessible.name: root.hasMedia ? qsTr("Open %1").arg(root.mediaLabel) : qsTr("Download %1").arg(root.mediaLabel)
                    onClicked: root.hasMedia ? backend.openFile(root.modelData.media_path) : backend.downloadMedia(root.modelData.id)
                }
            }

            SelectableMessageText {
                id: bodyText
                visible: Boolean(root.modelData.body)
                Layout.preferredWidth: implicitWidth
                Layout.maximumWidth: root.contentMaxWidth
                Layout.alignment: Qt.AlignLeft
                maximumWidth: root.contentMaxWidth
                plainText: root.modelData.revoked ? qsTr("This message was deleted") : root.modelData.body || ""
                color: root.modelData.revoked ? Theme.textMuted : Theme.text
                font.italic: Boolean(root.modelData.revoked)
            }

            Label {
                visible: !Boolean(root.modelData.body) && !root.mediaKind && root.modelData.kind !== "system"
                text: qsTr("Unsupported message")
                color: Theme.textMuted
                font.pixelSize: 13
                font.italic: true
            }

            RowLayout {
                Layout.alignment: Qt.AlignRight
                spacing: 4
                Label {
                    visible: Boolean(root.modelData.edited)
                    text: qsTr("edited")
                    color: Theme.textMuted
                    font.pixelSize: 11
                }
                Label {
                    text: Qt.formatTime(new Date(root.modelData.timestamp), "HH:mm")
                    color: Theme.textMuted
                    font.pixelSize: 11
                }
                Label {
                    visible: Boolean(root.modelData.from_me)
                    text: root.modelData.status === "read" || root.modelData.status === "played" ? "✓✓" : root.modelData.status === "delivered" ? "✓✓" : "✓"
                    color: root.modelData.status === "read" || root.modelData.status === "played" ? Theme.readReceipt : Theme.textMuted
                    font.pixelSize: 12
                    Accessible.name: root.modelData.status
                }
            }

            Flow {
                visible: Boolean(root.modelData.reactions) && root.modelData.reactions.length > 0
                Layout.maximumWidth: root.contentMaxWidth
                spacing: 4
                Repeater {
                    model: root.modelData.reactions || []
                    Label {
                        required property var modelData
                        text: modelData.emoji
                        font.family: Theme.emojiFontFamily
                        font.pixelSize: 16
                        padding: 3
                        background: Rectangle { color: Theme.surfaceMuted; radius: 8; border.color: Theme.border }
                        Accessible.name: qsTr("Reaction %1").arg(modelData.emoji)
                    }
                }
            }
        }
    }

    TapHandler {
        acceptedButtons: Qt.RightButton
        onTapped: (eventPoint, button) => {
            const mapped = root.mapToItem(contextMenu.parent, eventPoint.position.x, eventPoint.position.y)
            contextMenu.x = Math.max(8, Math.min(contextMenu.parent.width - contextMenu.width - 8, mapped.x))
            contextMenu.y = Math.max(8, Math.min(contextMenu.parent.height - contextMenu.implicitHeight - 8, mapped.y))
            contextMenu.open()
        }
    }

    WhatsAppMenuPopup {
        id: contextMenu
        parent: Overlay.overlay
        width: 246

        WhatsAppMenuItem {
            visible: Boolean(bodyText.selectedText)
            height: visible ? 46 : 0
            text: qsTr("Copy selected text")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: {
                bodyText.copy()
                contextMenu.close()
            }
        }

        WhatsAppMenuItem {
            visible: root.modelData.kind === "image" || root.modelData.kind === "sticker"
            height: visible ? 46 : 0
            text: qsTr("Copy image")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: {
                contextMenu.close()
                backend.copyImage(root.modelData.id, root.modelData.media_path || "")
            }
        }

        WhatsAppMenuItem {
            text: qsTr("Reply")
            iconSource: Qt.resolvedUrl("icons/reply.svg")
            onClicked: {
                contextMenu.close()
                root.replyRequested(root.modelData.id, root.modelData.body || root.mediaLabel)
            }
        }
        WhatsAppMenuItem {
            text: qsTr("React with thumbs up")
            iconSource: Qt.resolvedUrl("icons/smile.svg")
            onClicked: {
                contextMenu.close()
                backend.reactMessage(root.modelData.id, root.modelData.sender_jid || "", "👍")
            }
        }
        WhatsAppMenuItem {
            text: qsTr("React with heart")
            iconSource: Qt.resolvedUrl("icons/heart.svg")
            onClicked: {
                contextMenu.close()
                backend.reactMessage(root.modelData.id, root.modelData.sender_jid || "", "❤️")
            }
        }
        Rectangle {
            visible: Boolean(root.modelData.from_me)
            width: parent.width
            height: visible ? 1 : 0
            color: Theme.border
        }
        WhatsAppMenuItem {
            visible: Boolean(root.modelData.from_me) && root.modelData.kind === "text" && !root.modelData.revoked
            height: visible ? 46 : 0
            text: qsTr("Edit")
            iconSource: Qt.resolvedUrl("icons/edit.svg")
            onClicked: {
                contextMenu.close()
                root.editRequested(root.modelData.id, root.modelData.body || "")
            }
        }
        WhatsAppMenuItem {
            visible: Boolean(root.modelData.from_me) && !root.modelData.revoked
            height: visible ? 46 : 0
            text: qsTr("Delete for everyone")
            iconSource: Qt.resolvedUrl("icons/delete.svg")
            destructive: true
            onClicked: {
                contextMenu.close()
                root.deleteRequested(root.modelData.id, root.modelData.sender_jid || "")
            }
        }
    }
}
