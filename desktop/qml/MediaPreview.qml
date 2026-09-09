import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: root
    objectName: "mediaPreviewOverlay"

    property url imageUrl: ""
    property real imageRotation: 0
    property bool sending: false
    property bool sendAllowed: true
    property bool enterIsSend: true
    property bool replaceEmoticons: true
    property bool spellChecking: false
    property string spellLanguage: "en_US"
    readonly property bool previewActive: String(imageUrl).length > 0
    signal sendRequested(url imageUrl, string caption, int rotation)
    signal canceled(url imageUrl)
    signal addRequested()

    visible: previewActive
    onVisibleChanged: if (!visible) previewEmoji.close()
    onSendingChanged: if (sending) previewEmoji.close()
    Accessible.name: qsTr("Image preview")

    // Escape backs out of the preview, the way it closes every other overlay.
    Shortcut {
        sequences: [StandardKey.Cancel]
        enabled: root.visible && !root.sending && !Theme.popupOwnsFocus(root.Overlay.overlay,
            root.Window.window ? root.Window.window.activeFocusItem : null)
        onActivated: root.closePreview()
    }

    function openImage(url) {
        imageUrl = url
        imageRotation = 0
        caption.clear()
        Qt.callLater(() => caption.forceActiveFocus())
    }

    function closePreview() {
        if (sending)
            return
        const discardedUrl = imageUrl
        imageUrl = ""
        canceled(discardedUrl)
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.surface
    }

    Rectangle {
        id: toolbar
        enabled: !root.sending
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 68
        color: Theme.surface

        ThemedToolButton {
            anchors.left: parent.left
            anchors.leftMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            iconSource: Qt.resolvedUrl("icons/close.svg")
            iconSize: 20
            Accessible.name: qsTr("Close image preview")
            onClicked: root.closePreview()
            background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
        }

        RowLayout {
            anchors.centerIn: parent
            spacing: 8

            ThemedToolButton {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/rotate-left.svg")
                iconSize: 21
                Accessible.name: qsTr("Rotate left")
                onClicked: root.imageRotation -= 90
                background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            ThemedToolButton {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/rotate-right.svg")
                iconSize: 21
                Accessible.name: qsTr("Rotate right")
                onClicked: root.imageRotation += 90
                background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            ThemedToolButton {
                id: previewEmojiButton
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/smile.svg")
                iconSize: 21
                Accessible.name: qsTr("Add emoji to caption")
                onClicked: previewEmoji.opened ? previewEmoji.close() : previewEmoji.open()
                background: Rectangle { radius: 22; color: parent.hovered || previewEmoji.opened ? Theme.hoverRow : "transparent" }
            }
        }
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: toolbar.bottom
        anchors.bottom: footer.top
        anchors.margins: 20
        // A rotated image is drawn from its centre, so a quarter turn pushes
        // its corners outside this area and over the toolbar unless it is both
        // clipped and measured against the side it will occupy once turned.
        clip: true

        Image {
            id: previewImage
            anchors.centerIn: parent
            readonly property bool quarterTurn: Math.abs(root.imageRotation % 180) === 90
            readonly property real naturalWidth: sourceSize.width > 0 ? sourceSize.width : 520
            readonly property real naturalHeight: sourceSize.height > 0 ? sourceSize.height : 520
            readonly property real availableAcross: quarterTurn ? parent.height - 16 : parent.width - 40
            readonly property real availableDown: quarterTurn ? parent.width - 40 : parent.height - 16
            width: Math.max(1, Math.min(naturalWidth, availableAcross))
            height: Math.max(1, Math.min(naturalHeight, availableDown))
            source: root.imageUrl
            rotation: root.imageRotation
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            smooth: true
            Accessible.name: qsTr("Image to send")
        }
    }

    Item {
        id: footer
        objectName: "mediaPreviewFooter"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        // Reserve the thumbnail/send row below the growing caption. Otherwise
        // extra lines either disappear or overlap these controls and the image.
        height: captionFrame.height + 86

        Rectangle {
            id: captionFrame
            objectName: "mediaPreviewCaptionFrame"
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.top
            width: Math.max(1, Math.min(parent.width - 150, 760))
            readonly property real maximumHeight: Math.max(56, Math.min(180, root.height * 0.3))
            height: Math.min(maximumHeight, Math.max(56,
                caption.contentHeight + caption.topPadding + caption.bottomPadding))
            radius: 10
            color: Theme.surfaceMuted

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 14
                anchors.rightMargin: 6
                spacing: 4

                ScrollView {
                    id: captionScroll
                    objectName: "mediaPreviewCaptionScroll"
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true
                    contentWidth: availableWidth
                    ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
                    ScrollBar.vertical: OverlayScrollBar {}

                    // Wrapping changes on a window resize without moving the
                    // text cursor. Keep that cursor in view after layout, but
                    // do not recenter while the reader scrolls older lines.
                    function revealCursorAfterResize() {
                        const flick = contentItem
                        if (!caption.activeFocus || !flick || typeof flick.contentY === "undefined")
                            return
                        const cursor = caption.cursorRectangle
                        const top = caption.mapToItem(flick.contentItem, 0, cursor.y).y
                        const bottom = top + cursor.height
                        const maxY = Math.max(0, flick.contentHeight - flick.height)
                        if (top < flick.contentY)
                            flick.contentY = Math.max(0, top)
                        else if (bottom > flick.contentY + flick.height)
                            flick.contentY = Math.min(maxY, bottom - flick.height)
                    }
                    onAvailableWidthChanged: Qt.callLater(revealCursorAfterResize)
                    onAvailableHeightChanged: Qt.callLater(revealCursorAfterResize)

                    // ScrollView keeps the cursor visible once the caption
                    // reaches its height limit, including pasted and RTL text.
                    TextArea {
                        id: caption
                        readOnly: root.sending
                        objectName: "mediaPreviewCaption"
                        width: captionScroll.availableWidth
                        leftPadding: 0
                        rightPadding: 6
                        topPadding: 15
                        bottomPadding: 8
                        placeholderText: qsTr("Add a caption")
                        color: Theme.text
                        font.pixelSize: 14
                        wrapMode: TextEdit.Wrap
                        textFormat: TextEdit.PlainText
                        ComposerText {
                            id: captionText
                            editor: caption
                            spellChecking: root.spellChecking && root.visible
                            spellLanguage: root.spellLanguage
                        }
                        ComposerEditMenu { id: captionEditMenu; editor: caption; formatter: captionText }
                        MouseArea {
                            anchors.fill: parent
                            acceptedButtons: Qt.RightButton
                            onPressed: mouse => captionEditMenu.showAt(caption.positionAt(mouse.x, mouse.y), mouse.x, mouse.y)
                        }
                        Accessible.name: qsTr("Image caption")
                        background: Item {}
                        Keys.onReleased: event => {
                            if (root.replaceEmoticons && (event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
                                    && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier)))
                                captionText.convertEmoticons()
                        }
                        Keys.onPressed: event => {
                            if (event.key === Qt.Key_Menu || (event.key === Qt.Key_F10 && event.modifiers & Qt.ShiftModifier)) {
                                captionEditMenu.showAt(caption.cursorPosition, caption.cursorRectangle.x, caption.cursorRectangle.y + caption.cursorRectangle.height)
                                event.accepted = true
                            } else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !(event.modifiers & Qt.ShiftModifier)
                                    && (root.enterIsSend || event.modifiers & (Qt.ControlModifier | Qt.MetaModifier))) {
                                sendButton.clicked()
                                event.accepted = true
                            }
                        }
                    }
                }

                ThemedToolButton {
                    enabled: !root.sending
                    Layout.alignment: Qt.AlignBottom
                    Layout.bottomMargin: 6
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    iconSource: Qt.resolvedUrl("icons/smile.svg")
                    iconSize: 21
                    Accessible.name: qsTr("Add emoji to caption")
                    onClicked: previewEmoji.opened ? previewEmoji.close() : previewEmoji.open()
                    background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }
            }
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 16
            spacing: 12

            Rectangle {
                width: 66
                height: 66
                radius: 6
                color: "transparent"
                border.width: 2
                border.color: Theme.primary
                Image {
                    anchors.fill: parent
                    anchors.margins: 3
                    source: root.imageUrl
                    fillMode: Image.PreserveAspectCrop
                    cache: false
                }
            }

            Button {
                enabled: !root.sending
                width: 66
                height: 66
                text: "+"
                font.pixelSize: 28
                font.weight: Font.Light
                Accessible.name: qsTr("Replace with clipboard image")
                onClicked: root.addRequested()
                contentItem: Label {
                    text: parent.text
                    color: Theme.text
                    font: parent.font
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 6
                    color: parent.hovered ? Theme.hoverRow : Theme.surface
                    border.width: 1
                    border.color: Theme.border
                }
            }
        }

        ThemedToolButton {
            id: sendButton
            objectName: "mediaPreviewSendButton"
            anchors.right: parent.right
            anchors.rightMargin: 24
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 16
            width: 64
            height: 64
            iconSource: Qt.resolvedUrl("icons/send.svg")
            iconTint: Theme.primaryText
            iconSize: 24
            enabled: root.visible && root.previewActive && root.sendAllowed && !root.sending
            Accessible.name: qsTr("Send image")
            onClicked: {
                if (!enabled)
                    return
                if (root.replaceEmoticons) captionText.convertEmoticons(true)
                const sentUrl = root.imageUrl
                const sentCaption = caption.text
                // What the reader turned is what gets sent; the preview used to
                // be the only place the turn existed.
                const turned = root.imageRotation
                root.sending = true
                root.sendRequested(sentUrl, sentCaption, turned)
            }
            background: Rectangle {
                radius: width / 2
                color: sendButton.down ? Qt.darker(Theme.primary, 1.12) : Theme.primary
            }
        }
    }

    EmojiPicker {
        id: previewEmoji
        parent: root
        x: Math.max(12, (root.width - width) / 2)
        y: Math.max(76, footer.y - height - 8)
        onEmojiChosen: emoji => {
            if (root.sending)
                return
            const position = Math.max(0, caption.cursorPosition)
            caption.insert(position, emoji)
            caption.cursorPosition = position + emoji.length
            caption.forceActiveFocus()
        }
    }
}
